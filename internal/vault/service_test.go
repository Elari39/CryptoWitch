package vault

import (
	"errors"
	"strings"
	"testing"
)

func testService(t *testing.T) *Service {
	t.Helper()
	encrypted, err := EncryptVault(PlainVault{
		Documents: []PlainDocument{
			{
				ID:      "intro",
				Title:   "Intro",
				Path:    "guide/intro.md",
				Content: "# Intro\n\nHello **CryptoWitch**.\n<script>alert('x')</script>",
			},
		},
	}, "correct-password", KDFParams{Time: 1, Memory: 64 * 1024, Threads: 1, KeyLen: 32})
	if err != nil {
		t.Fatalf("EncryptVault() error = %v", err)
	}
	return NewService(encrypted)
}

func TestUnlockWithCorrectPassword(t *testing.T) {
	service := testService(t)

	response, err := service.Unlock("correct-password")
	if err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if len(response.Tree) != 1 || response.Tree[0].Kind != "folder" {
		t.Fatalf("unexpected tree: %#v", response.Tree)
	}
}

func TestUnlockWithWrongPassword(t *testing.T) {
	service := testService(t)

	_, err := service.Unlock("wrong-password")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("Unlock() error = %v, want ErrInvalidPassword", err)
	}
	if _, err := service.GetTree(); !errors.Is(err, ErrLocked) {
		t.Fatalf("GetTree() error = %v, want ErrLocked", err)
	}
}

func TestUnlockWithWrongPasswordClearsExistingState(t *testing.T) {
	service := testService(t)

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if _, err := service.Unlock("wrong-password"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("Unlock() error = %v, want ErrInvalidPassword", err)
	}
	if _, err := service.GetTree(); !errors.Is(err, ErrLocked) {
		t.Fatalf("GetTree() error = %v, want ErrLocked", err)
	}
	if _, err := service.GetDocument("intro"); !errors.Is(err, ErrLocked) {
		t.Fatalf("GetDocument() error = %v, want ErrLocked", err)
	}
}

func TestLockedAccessAndLock(t *testing.T) {
	service := testService(t)

	if _, err := service.GetDocument("intro"); !errors.Is(err, ErrLocked) {
		t.Fatalf("GetDocument() before unlock error = %v, want ErrLocked", err)
	}
	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	service.Lock()

	if _, err := service.GetDocument("intro"); !errors.Is(err, ErrLocked) {
		t.Fatalf("GetDocument() after lock error = %v, want ErrLocked", err)
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	service := testService(t)

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if _, err := service.GetDocument("missing"); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("GetDocument() error = %v, want ErrDocumentNotFound", err)
	}
}

func TestRenderMarkdownDoesNotAllowRawHTML(t *testing.T) {
	html, err := RenderMarkdown("# Title\n\n<script>alert('x')</script>")
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	if strings.Contains(html, "<script>") {
		t.Fatalf("RenderMarkdown() allowed raw script: %s", html)
	}
	if !strings.Contains(html, "<h1") || !strings.Contains(html, "Title") {
		t.Fatalf("RenderMarkdown() did not render safe markdown: %s", html)
	}
}

func TestRenderMarkdownHighlightsFencedCode(t *testing.T) {
	html, err := RenderMarkdown("```go\nfmt.Println(\"hi\")\n```")
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	if !strings.Contains(html, "<pre") || !strings.Contains(html, "<code") {
		t.Fatalf("RenderMarkdown() did not render a code block: %s", html)
	}
	if !strings.Contains(html, "<span style=") || !strings.Contains(html, "color:") {
		t.Fatalf("RenderMarkdown() did not add syntax highlighting markup: %s", html)
	}
	if !strings.Contains(html, "fmt") || !strings.Contains(html, "Println") {
		t.Fatalf("RenderMarkdown() dropped code contents: %s", html)
	}
}

func TestRenderMarkdownKeepsUnknownLanguageCode(t *testing.T) {
	html, err := RenderMarkdown("```not-a-real-language\n<token>\n```")
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	if !strings.Contains(html, "<pre") || !strings.Contains(html, "<code") {
		t.Fatalf("RenderMarkdown() did not render a code block: %s", html)
	}
	if !strings.Contains(html, "&lt;token&gt;") {
		t.Fatalf("RenderMarkdown() did not preserve escaped code contents: %s", html)
	}
}
