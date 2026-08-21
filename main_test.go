package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cryptowitch/internal/vault"
)

func TestCSPPolicyDefaultBlocksRemoteImages(t *testing.T) {
	policy := cspPolicy(false)
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"frame-src blob:",
		"connect-src 'self'",
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'none'",
	} {
		if !strings.Contains(policy, want) {
			t.Errorf("cspPolicy(false) missing %q in: %s", want, policy)
		}
	}
	if strings.Contains(policy, "script-src 'self' 'unsafe-inline'") {
		t.Errorf("cspPolicy(false) must not allow inline scripts: %s", policy)
	}
	if strings.Contains(policy, "https:") || strings.Contains(policy, "http:") {
		t.Errorf("cspPolicy(false) must not allow remote images: %s", policy)
	}
}

func TestCSPPolicyAllowsRemoteImagesWhenEnabled(t *testing.T) {
	policy := cspPolicy(true)
	if !strings.Contains(policy, "img-src 'self' data: blob: https: http:") {
		t.Errorf("cspPolicy(true) should allow remote image sources: %s", policy)
	}
}

func TestInjectCSPMeta(t *testing.T) {
	meta := []byte(`<meta http-equiv="Content-Security-Policy" content="default-src 'self'">`)
	html := []byte("<!DOCTYPE html>\n<html>\n<head>\n  <title>T</title>\n</head>\n<body></body>\n</html>\n")
	injected := injectCSPMeta(html, meta)
	if injected == nil {
		t.Fatal("injectCSPMeta() returned nil, want injected html")
	}
	text := string(injected)
	if !strings.Contains(text, string(meta)) {
		t.Fatalf("injected html missing meta: %s", text)
	}
	if strings.Index(text, string(meta)) >= strings.Index(text, "</head>") {
		t.Fatalf("meta must appear before </head>: %s", text)
	}
	// 原文档内容应完整保留。
	for _, fragment := range []string{"<!DOCTYPE html>", "<title>T</title>", "<body></body>"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("injected html dropped original fragment %q: %s", fragment, text)
		}
	}
}

func TestInjectCSPMetaWithoutHeadTag(t *testing.T) {
	html := []byte("<html><body>no head</body></html>")
	if injected := injectCSPMeta(html, []byte("<meta>")); injected != nil {
		t.Fatalf("injectCSPMeta() = %q, want nil when </head> missing", injected)
	}
}

// newTestAssetHandler 构造用于集成测试的 cspAssetHandler（空 vault，禁远程图片）。
func newTestAssetHandler() http.Handler {
	return cspAssetHandler(assets, vault.NewService(vault.EncryptedVault{}))
}

// TestCSPAssetHandlerInjectsIntoHTML 验证 HTML 响应注入 CSP meta 且移除失效的
// Content-Length；注入位置在 </head> 之前。
func TestCSPAssetHandlerInjectsIntoHTML(t *testing.T) {
	handler := newTestAssetHandler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `<meta http-equiv="Content-Security-Policy"`) {
		t.Fatalf("response missing CSP meta:\n%s", body)
	}
	if !strings.Contains(body, "script-src 'self'") {
		t.Fatalf("response CSP missing script-src 'self':\n%s", body)
	}
	if metaIndex := strings.Index(body, `http-equiv="Content-Security-Policy"`); metaIndex >= strings.Index(body, "</head>") {
		t.Fatalf("CSP meta must appear before </head>:\n%s", body)
	}
	// 注入后正文长度变化，原始 Content-Length 必须被移除，避免客户端截断。
	if recorder.Header().Get("Content-Length") != "" {
		t.Fatalf("Content-Length should be removed after injection, got %q", recorder.Header().Get("Content-Length"))
	}
	if recorder.Body.Len() == 0 {
		t.Fatal("injected response body is empty")
	}
}

// TestCSPAssetHandlerPassthroughNonHTML 验证非 HTML 静态资源不注入 CSP、
// 响应体原样透传且 Content-Length 保留。
func TestCSPAssetHandlerPassthroughNonHTML(t *testing.T) {
	handler := newTestAssetHandler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/style.css", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	contentType := recorder.Header().Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want css", contentType)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "Content-Security-Policy") {
		t.Fatal("non-HTML asset must not be injected with CSP meta")
	}
	// 非 HTML 资源正文未被改写，Content-Length 应保留。
	if recorder.Header().Get("Content-Length") == "" {
		t.Fatal("Content-Length should be preserved for passthrough assets")
	}
}

// TestCSPAssetHandlerNotFoundKeepsStatus 验证不存在的资源保持 404 状态码透传。
func TestCSPAssetHandlerNotFoundKeepsStatus(t *testing.T) {
	handler := newTestAssetHandler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/definitely-missing-xyz.js", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "Content-Security-Policy") {
		t.Fatal("non-200 response must not be injected with CSP meta")
	}
}
