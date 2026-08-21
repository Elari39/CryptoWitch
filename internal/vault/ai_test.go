package vault

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildAIMessagesTruncatesSelectedText(t *testing.T) {
	long := strings.Repeat("文", aiMaxSelectedTextRunes+1000)
	messages := buildAIMessages(AIChatRequest{
		SelectedText: long,
		Question:     "请解读",
	})
	// system + 片段 + 问题
	if len(messages) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(messages))
	}
	selection := messages[1].Content
	body := strings.TrimPrefix(selection, aiSelectedTextPrefix+"\n\n")
	body = strings.TrimSuffix(body, "\n\n请基于此片段回答我接下来的问题。")
	if runes := len([]rune(body)); runes != aiMaxSelectedTextRunes {
		t.Fatalf("selection body runes = %d, want %d", runes, aiMaxSelectedTextRunes)
	}
}

func TestBuildAIMessagesInjectsSelectionWithHistory(t *testing.T) {
	history := []AIMessage{
		{Role: "user", Content: "首问"},
		{Role: "assistant", Content: "回答1"},
	}
	messages := buildAIMessages(AIChatRequest{
		SelectedText: "片段A",
		Question:     "追问",
		History:      history,
	})
	// 对话接口为无状态请求，前端历史不含片段消息，因此每轮都要注入片段：
	// system + 片段 + 2 条历史 + 当前问题。
	if len(messages) != 1+1+len(history)+1 {
		t.Fatalf("len(messages) = %d, want %d", len(messages), 1+1+len(history)+1)
	}
	if !strings.HasPrefix(messages[1].Content, aiSelectedTextPrefix) {
		t.Fatalf("selection not injected in follow-up turn: %#v", messages[1])
	}
	for index, message := range messages[2 : 2+len(history)] {
		if message.Role != history[index].Role || message.Content != history[index].Content {
			t.Fatalf("message %d = %#v, want history %#v", index, message, history[index])
		}
	}
	if messages[len(messages)-1].Content != "追问" {
		t.Fatalf("last message = %#v, want current question", messages[len(messages)-1])
	}
}

func TestBuildAIMessagesTruncatesQuestionAndHistory(t *testing.T) {
	history := []AIMessage{{Role: "user", Content: strings.Repeat("史", aiMaxHistoryMessageRunes+500)}}
	messages := buildAIMessages(AIChatRequest{
		SelectedText: "片段",
		Question:     strings.Repeat("问", aiMaxQuestionRunes+500),
		History:      history,
	})
	// system + 片段 + 1 条历史 + 问题
	if len(messages) != 4 {
		t.Fatalf("len(messages) = %d, want 4", len(messages))
	}
	if got := len([]rune(messages[2].Content)); got != aiMaxHistoryMessageRunes {
		t.Fatalf("history message runes = %d, want %d", got, aiMaxHistoryMessageRunes)
	}
	if got := len([]rune(messages[3].Content)); got != aiMaxQuestionRunes {
		t.Fatalf("question runes = %d, want %d", got, aiMaxQuestionRunes)
	}
}

func TestBuildAIMessagesInjectsSelectionWhenHistoryLacksIt(t *testing.T) {
	req := AIChatRequest{
		SelectedText: "片段B",
		Question:     "首问",
		History:      nil,
	}
	messages := buildAIMessages(req)
	if len(messages) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(messages))
	}
	if !strings.HasPrefix(messages[1].Content, aiSelectedTextPrefix) {
		t.Fatalf("selection not injected: %#v", messages[1])
	}
}

func TestBuildAIMessagesLimitsHistory(t *testing.T) {
	history := make([]AIMessage, 0, 20)
	for index := 0; index < 20; index++ {
		role := "user"
		if index%2 == 1 {
			role = "assistant"
		}
		history = append(history, AIMessage{Role: role, Content: "msg"})
	}
	messages := buildAIMessages(AIChatRequest{
		SelectedText: "片段",
		Question:     "问题",
		History:      history,
	})
	// system + 片段 + 最近 12 条历史 + 问题
	if len(messages) != 1+1+aiMaxHistoryMessages+1 {
		t.Fatalf("len(messages) = %d, want %d", len(messages), 1+1+aiMaxHistoryMessages+1)
	}
	// 截断后保留的是最近 12 条：system 与片段之后第一条历史内容为 "msg"（无法区分），
	// 直接断言历史总量即可。
	if got := len(messages) - 3; got != aiMaxHistoryMessages {
		t.Fatalf("history messages = %d, want %d", got, aiMaxHistoryMessages)
	}
}

func TestBuildAIMessagesFiltersInvalidRoles(t *testing.T) {
	messages := buildAIMessages(AIChatRequest{
		SelectedText: "片段",
		Question:     "问题",
		History: []AIMessage{
			{Role: "system", Content: "should be dropped"},
			{Role: "user", Content: "  "},
			{Role: "assistant", Content: "有效"},
			{Role: "tool", Content: "should be dropped"},
		},
	})
	// system + 片段 + 1 条有效历史 + 问题
	if len(messages) != 4 {
		t.Fatalf("len(messages) = %d, want 4: %#v", len(messages), messages)
	}
}

func TestAIChatRejectsUnknownDocument(t *testing.T) {
	service := testService(t)
	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	err := service.AIChat(AIChatRequest{
		RequestID:    1,
		DocumentID:   "missing",
		SelectedText: "x",
		Question:     "q",
	})
	if !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("AIChat() error = %v, want ErrDocumentNotFound", err)
	}
}

func TestAIChatValidatesLockedAndConfigured(t *testing.T) {
	service := testService(t)

	// 未解锁时优先报 ErrLocked（即使文档 id 无效）。
	err := service.AIChat(AIChatRequest{DocumentID: "missing", Question: "q"})
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("AIChat() locked error = %v, want ErrLocked", err)
	}

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	// 测试 vault 未配置 AI 服务：文档合法时返回 ErrAINotConfigured。
	err = service.AIChat(AIChatRequest{DocumentID: "intro", Question: "q"})
	if !errors.Is(err, ErrAINotConfigured) {
		t.Fatalf("AIChat() error = %v, want ErrAINotConfigured", err)
	}
}

func TestModelListPrecedence(t *testing.T) {
	// ai.models 数组优先于旧格式单值 ai.model。
	cfg := AIConfig{Model: "legacy", Models: []string{"m1", "m2"}}
	got := modelList(cfg)
	if len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("modelList() = %v, want [m1 m2]", got)
	}

	// 旧格式单值回退。
	if got := modelList(AIConfig{Model: "legacy"}); len(got) != 1 || got[0] != "legacy" {
		t.Fatalf("modelList() = %v, want [legacy]", got)
	}

	// 都未配置返回 nil。
	if got := modelList(AIConfig{}); got != nil {
		t.Fatalf("modelList() = %v, want nil", got)
	}
}

func TestResolveAIModel(t *testing.T) {
	models := []string{"a", "b", "c"}

	// 空请求取列表首个。
	model, err := resolveAIModel("", models)
	if err != nil || model != "a" {
		t.Fatalf("resolveAIModel(\"\") = %q, %v; want \"a\", nil", model, err)
	}

	// 命中列表原样返回。
	if model, err := resolveAIModel("b", models); err != nil || model != "b" {
		t.Fatalf("resolveAIModel(\"b\") = %q, %v; want \"b\", nil", model, err)
	}

	// 未知模型报错。
	if _, err := resolveAIModel("nope", models); err == nil {
		t.Fatal("resolveAIModel(\"nope\") error = nil, want error")
	}

	// 空列表 + 空请求报错。
	if _, err := resolveAIModel("", nil); err == nil {
		t.Fatal("resolveAIModel(\"\", nil) error = nil, want error")
	}
}

func TestGetAIInfoModels(t *testing.T) {
	// 多模型配置：可用，返回全列表与默认首个。
	service := NewService(EncryptedVault{
		AIConfig: AIConfig{
			Endpoint: "https://example.com/v1/chat/completions",
			ApiKey:   "sk-test",
			Models:   []string{"m1", "m2"},
		},
	})
	info, err := service.GetAIInfo()
	if err != nil {
		t.Fatalf("GetAIInfo() error = %v", err)
	}
	if !info.Available {
		t.Fatal("GetAIInfo().Available = false, want true")
	}
	if info.Model != "m1" || len(info.Models) != 2 || info.Models[1] != "m2" {
		t.Fatalf("GetAIInfo() = %#v, want model m1 with 2 models", info)
	}

	// 旧单值配置回退为单元素列表。
	service = NewService(EncryptedVault{
		AIConfig: AIConfig{Endpoint: "e", ApiKey: "k", Model: "legacy"},
	})
	info, err = service.GetAIInfo()
	if err != nil {
		t.Fatalf("GetAIInfo() error = %v", err)
	}
	if !info.Available || info.Model != "legacy" || len(info.Models) != 1 {
		t.Fatalf("GetAIInfo() = %#v, want single legacy model", info)
	}

	// 缺凭证不可用（models 已配但 endpoint/apiKey 为空）。
	service = NewService(EncryptedVault{AIConfig: AIConfig{Models: []string{"m"}}})
	info, err = service.GetAIInfo()
	if err != nil {
		t.Fatalf("GetAIInfo() error = %v", err)
	}
	if info.Available {
		t.Fatal("GetAIInfo().Available = true, want false (missing endpoint/apiKey)")
	}
}

func TestAIChatRejectsModelNotInList(t *testing.T) {
	service := testService(t)
	service.aiConfig = AIConfig{
		Endpoint: "https://example.com/v1/chat/completions",
		ApiKey:   "sk-test",
		Models:   []string{"m1", "m2"},
	}
	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	// 模型不在配置列表：在校验阶段直接返回错误，不发起网络请求。
	err := service.AIChat(AIChatRequest{
		RequestID:  1,
		DocumentID: "intro",
		Question:   "q",
		Model:      "not-configured",
	})
	if err == nil || !strings.Contains(err.Error(), "not-configured") {
		t.Fatalf("AIChat() error = %v, want model-not-configured error", err)
	}
}

// TestLockCancelsInflightAIStream 验证锁定会取消进行中的 AI 流式请求，
// 避免 goroutine 挂起至 30 分钟总超时（M2）。
func TestLockCancelsInflightAIStream(t *testing.T) {
	received := make(chan struct{})
	ctxDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 必须先消费请求体：带 body 的请求被客户端 cancel 时，
		// net/http 只有在 body 已被服务端读取后才会关闭连接，
		// 否则 r.Context().Done() 不会触发（真实 AI 服务端总会读取请求体）。
		io.Copy(io.Discard, r.Body)
		close(received)
		<-r.Context().Done()
		close(ctxDone)
	}))
	defer server.Close()

	service := testService(t)
	service.aiConfig = AIConfig{
		Endpoint: server.URL,
		ApiKey:   "sk-test",
		Models:   []string{"m"},
	}
	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	if err := service.AIChat(AIChatRequest{
		RequestID:  1,
		DocumentID: "intro",
		Question:   "q",
	}); err != nil {
		t.Fatalf("AIChat() error = %v", err)
	}

	select {
	case <-received:
	case <-time.After(3 * time.Second):
		t.Fatal("AI request never reached test server")
	}

	service.Lock()

	select {
	case <-ctxDone:
	case <-time.After(3 * time.Second):
		t.Fatal("in-flight AI request context was not canceled after Lock")
	}

	service.mu.Lock()
	if service.aiCancel != nil {
		t.Fatal("aiCancel should be cleared after Lock")
	}
	service.mu.Unlock()
}
