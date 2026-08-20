package main

import (
	"strings"
	"testing"
)

func TestCSPPolicyDefaultBlocksRemoteImages(t *testing.T) {
	policy := cspPolicy(false)
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline'",
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
