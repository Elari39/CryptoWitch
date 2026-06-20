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
	Model    string           `json:"model"`
	Stream   bool             `json:"stream"`
	Messages []openAIMessage  `json:"messages"`
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

// GetAIInfo 返回当前 AI 配置的可用性与展示信息（不含 apiKey）。
func (s *Service) GetAIInfo() (AIInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.aiConfig
	return AIInfo{
		Available: cfg.Endpoint != "" && cfg.ApiKey != "" && cfg.Model != "",
		Model:     cfg.Model,
		Endpoint:  cfg.Endpoint,
	}, nil
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
	s.mu.RUnlock()

	if cfg.Endpoint == "" || cfg.ApiKey == "" || cfg.Model == "" {
		return ErrAINotConfigured
	}

	messages := buildAIMessages(req)
	body, err := json.Marshal(openAIStreamRequest{
		Model:    cfg.Model,
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		s.emitAI(aiEventError, requestID, session, "构建 AI 请求失败："+err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	request.Header.Set("Accept", "text/event-stream")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		s.emitAI(aiEventError, requestID, session, "AI 请求失败："+err.Error())
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
		s.emitAI(aiEventError, requestID, session, "读取 AI 响应失败："+err.Error())
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
func buildAIMessages(req AIChatRequest) []openAIMessage {
	messages := []openAIMessage{{Role: "system", Content: aiSystemPrompt}}
	if strings.TrimSpace(req.SelectedText) != "" {
		messages = append(messages, openAIMessage{
			Role: "user",
			Content: "以下是文档划选片段：\n\n" + req.SelectedText + "\n\n请基于此片段回答我接下来的问题。",
		})
	}
	for _, msg := range req.History {
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
