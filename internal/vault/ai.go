package vault

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const aiSystemPrompt = "你是一位严谨的文档解读助手。用户会从一篇文档中划选一段原文并提问，请基于该片段作答；若片段信息不足以回答，请明确指出，不要编造。回答使用中文，简洁清晰。"

// AI 请求的 token 控制上限：
//   - 划选片段最多保留前 4000 个字符（按 rune 计），避免超长划选浪费 token；
//   - 多轮追问时只携带最近 12 条历史消息（约 6 轮问答）。
const (
	aiMaxSelectedTextRunes = 4000
	aiMaxHistoryMessages   = 12
	aiSelectedTextPrefix   = "以下是文档划选片段："
)

// AI 流式请求的超时策略：
//   - 首字 60 秒：从发起请求到收到响应头（TTFT）。思考型模型的思考阶段可能较长，
//     但超过 60 秒基本可判定服务异常或网络问题，直接中断并提示；
//   - 总时长 30 分钟：整段流式读取（含正文增量）的硬上限，超时后中断并提示内容可能不完整。
const (
	aiTTFTTimeout      = 60 * time.Second
	aiMaxStreamTimeout = 1800 * time.Second
)

// aiEventChunk / aiEventDone / aiEventError 是推送给前端的事件名。
const (
	aiEventChunk = "ai:chunk"
	aiEventDone  = "ai:done"
	aiEventError = "ai:error"
)

// openAIMessage 对应 OpenAI Chat Completions 的 messages 元素。
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIStreamRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []openAIMessage `json:"messages"`
}

type openAIStreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
}

type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// modelList 返回配置中的全部可用模型：ai.models 数组优先；
// 旧格式 ai.model 单值作为回退；两者都未配置时返回 nil。
func modelList(cfg AIConfig) []string {
	if len(cfg.Models) > 0 {
		return cfg.Models
	}
	if cfg.Model != "" {
		return []string{cfg.Model}
	}
	return nil
}

// resolveAIModel 解析前端请求的模型：请求为空时取配置列表首个；
// 请求命中列表时原样返回；否则返回错误（防御前端被篡改或配置变更）。
func resolveAIModel(requested string, models []string) (string, error) {
	if requested == "" {
		if len(models) == 0 {
			return "", errors.New("ai models are not configured")
		}
		return models[0], nil
	}
	for _, model := range models {
		if model == requested {
			return requested, nil
		}
	}
	return "", fmt.Errorf("模型 %s 未在 access.yaml 的 ai.models 中配置", requested)
}

// GetAIInfo 返回当前 AI 配置的可用性与展示信息（不含 apiKey）。
func (s *Service) GetAIInfo() (AIInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.aiConfig
	models := modelList(cfg)
	available := cfg.Endpoint != "" && cfg.ApiKey != "" && len(models) > 0
	info := AIInfo{
		Available: available,
		Models:    models,
		Endpoint:  cfg.Endpoint,
	}
	if len(models) > 0 {
		info.Model = models[0]
	}
	return info, nil
}

// AIChat 发起一次流式对话。方法在校验后立即返回，真正的流式读取在
// goroutine 中完成，增量通过 Wails 事件 ai:chunk / ai:done / ai:error 推送。
func (s *Service) AIChat(req AIChatRequest) error {
	s.mu.RLock()
	if !s.unlocked {
		s.mu.RUnlock()
		return ErrLocked
	}
	cfg := s.aiConfig
	session := s.session
	_, documentExists := s.documents[req.DocumentID]
	s.mu.RUnlock()

	// 仅校验文档存在性（划选片段本身无法可靠验证归属）。
	if req.DocumentID != "" && !documentExists {
		return ErrDocumentNotFound
	}
	if cfg.Endpoint == "" || cfg.ApiKey == "" || len(modelList(cfg)) == 0 {
		return ErrAINotConfigured
	}

	// 校验所选模型属于配置列表（空请求回退到列表首个）。
	model, err := resolveAIModel(req.Model, modelList(cfg))
	if err != nil {
		return err
	}

	messages := buildAIMessages(req)
	body, err := json.Marshal(openAIStreamRequest{
		Model:    model,
		Stream:   true,
		Messages: messages,
	})
	if err != nil {
		return fmt.Errorf("encode ai request: %w", err)
	}

	// 在后台完成流式读取，事件驱动前端更新。
	go s.streamAIChat(cfg, body, req.RequestID, session)
	return nil
}

func (s *Service) streamAIChat(cfg AIConfig, body []byte, requestID int, session uint64) {
	// 总时长上限：请求与流式读取共用此上下文，到期后 net/http 会中断底层连接。
	totalCtx, cancel := context.WithTimeout(context.Background(), aiMaxStreamTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(totalCtx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		s.emitAI(aiEventError, requestID, session, "构建 AI 请求失败："+err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	request.Header.Set("Accept", "text/event-stream")

	// 首字超时 60 秒：Do 在 goroutine 中执行，主流程等待响应头或超时中断。
	type doResult struct {
		response *http.Response
		err      error
	}
	doCh := make(chan doResult, 1)
	go func() {
		response, doErr := http.DefaultClient.Do(request)
		doCh <- doResult{response: response, err: doErr}
	}()

	ttft := time.NewTimer(aiTTFTTimeout)
	defer ttft.Stop()

	var response *http.Response
	select {
	case result := <-doCh:
		if result.err != nil {
			cancel()
			s.emitAI(aiEventError, requestID, session, "AI 请求失败："+result.err.Error())
			return
		}
		response = result.response
	case <-ttft.C:
		cancel()
		s.emitAI(aiEventError, requestID, session, "AI 首字响应超时（60 秒），请重试或切换模型。")
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		s.emitAI(aiEventError, requestID, session, fmt.Sprintf("AI 服务返回 %d：%s", response.StatusCode, strings.TrimSpace(string(raw))))
		return
	}

	scanner := bufio.NewScanner(response.Body)
	// 单行可能较长（大段增量），提高缓冲上限。
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			s.emitAI(aiEventError, requestID, session, "AI 服务错误："+chunk.Error.Message)
			return
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		s.emitAI(aiEventChunk, requestID, session, delta)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		// 总时长上限到期时，net/http 会以 deadline/cancel 类错误中断读取。
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			s.emitAI(aiEventError, requestID, session, "读取 AI 响应超时（最长 30 分钟），内容可能不完整。")
		} else {
			s.emitAI(aiEventError, requestID, session, "读取 AI 响应失败："+err.Error())
		}
		return
	}
	s.emitAI(aiEventDone, requestID, session, "")
}

func (s *Service) emitAI(eventName string, requestID int, session uint64, data string) {
	// 会话已切换（锁定或重新解锁），丢弃过期事件。
	s.mu.RLock()
	current := s.session
	unlocked := s.unlocked
	s.mu.RUnlock()
	if !unlocked || current != session {
		return
	}
	app := application.Get()
	if app == nil || app.Event == nil {
		return
	}
	app.Event.Emit(eventName, map[string]any{
		"requestId": requestID,
		"data":      data,
	})
}

// buildAIMessages 把系统提示、划选片段、历史对话与当前问题组装成 OpenAI messages。
// 划选片段只注入一次：若历史首条已携带片段（多轮追问），不再重复发送；
// 片段超过 aiMaxSelectedTextRunes 时截断，历史仅保留最近 aiMaxHistoryMessages 条。
func buildAIMessages(req AIChatRequest) []openAIMessage {
	messages := []openAIMessage{{Role: "system", Content: aiSystemPrompt}}
	selected := strings.TrimSpace(req.SelectedText)
	if selected != "" && !historyAlreadyHasSelection(req.History) {
		if runes := []rune(selected); len(runes) > aiMaxSelectedTextRunes {
			selected = string(runes[:aiMaxSelectedTextRunes])
		}
		messages = append(messages, openAIMessage{
			Role:    "user",
			Content: aiSelectedTextPrefix + "\n\n" + selected + "\n\n请基于此片段回答我接下来的问题。",
		})
	}
	history := req.History
	if len(history) > aiMaxHistoryMessages {
		history = history[len(history)-aiMaxHistoryMessages:]
	}
	for _, msg := range history {
		role := msg.Role
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		messages = append(messages, openAIMessage{Role: role, Content: content})
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		question = "请解读上述片段。"
	}
	messages = append(messages, openAIMessage{Role: "user", Content: question})
	return messages
}

// historyAlreadyHasSelection 判断历史首条是否已携带划选片段
// （首条 user 消息以固定前缀开头），避免多轮追问重复发送同一片段。
func historyAlreadyHasSelection(history []AIMessage) bool {
	if len(history) == 0 {
		return false
	}
	first := history[0]
	return first.Role == "user" && strings.HasPrefix(strings.TrimSpace(first.Content), aiSelectedTextPrefix)
}
