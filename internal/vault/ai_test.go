package vault

import (
	"errors"
	"strings"
	"testing"
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

func TestBuildAIMessagesInjectsSelectionOnce(t *testing.T) {
	history := []AIMessage{
		{Role: "user", Content: aiSelectedTextPrefix + "\n\n片段A\n\n请基于此片段回答我接下来的问题。"},
		{Role: "assistant", Content: "回答1"},
		{Role: "user", Content: "追问1"},
		{Role: "assistant", Content: "回答2"},
	}
	req := AIChatRequest{
		SelectedText: "片段A",
		Question:     "追问2",
		History:      history,
	}
	messages := buildAIMessages(req)
	// system + 4 条历史 + 当前问题；历史首条已携带片段，不得再注入新片段。
	if len(messages) != 1+len(history)+1 {
		t.Fatalf("len(messages) = %d, want %d", len(messages), 1+len(history)+1)
	}
	for index, message := range messages[1 : 1+len(history)] {
		if message.Role != history[index].Role || message.Content != history[index].Content {
			t.Fatalf("message %d = %#v, want history %#v", index, message, history[index])
		}
	}
	if messages[len(messages)-1].Content != "追问2" {
		t.Fatalf("last message = %#v, want current question", messages[len(messages)-1])
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
