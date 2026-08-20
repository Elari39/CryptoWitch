package vault

import (
	"strings"
	"testing"
)

func TestRenderMarkdownSanitizesJavaScriptLinks(t *testing.T) {
	html, err := RenderMarkdown("[点我](javascript:alert(1))", false)
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	if strings.Contains(html, "javascript:") {
		t.Fatalf("RenderMarkdown() kept javascript: link: %s", html)
	}
}

func TestRenderMarkdownBlocksDangerousSchemes(t *testing.T) {
	source := "[d](data:text/html,<script>alert(1)</script>) [v](vbscript:msgbox(1)) [f](file:///etc/passwd)"
	html, err := RenderMarkdown(source, false)
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	for _, forbidden := range []string{"data:", "vbscript:", "file:"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("RenderMarkdown() kept %q link: %s", forbidden, html)
		}
	}
}

func TestRenderMarkdownKeepsSafeLinkSchemes(t *testing.T) {
	source := "[web](https://example.com/a) [http](http://example.com) [mail](mailto:a@b.c) [rel](guide/intro.md) [anchor](#section)"
	html, err := RenderMarkdown(source, false)
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	for _, want := range []string{
		`href="https://example.com/a"`,
		`href="http://example.com"`,
		`href="mailto:a@b.c"`,
		`href="guide/intro.md"`,
		`href="#section"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("RenderMarkdown() missing %s in: %s", want, html)
		}
	}
}

func TestRenderMarkdownImageSchemeWhitelist(t *testing.T) {
	source := "![local](img/a.png) ![data](data:image/png;base64,AAAA)"
	html, err := RenderMarkdown(source, true)
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	if !strings.Contains(html, `src="img/a.png"`) {
		t.Fatalf("RenderMarkdown() dropped relative image src: %s", html)
	}
	if strings.Contains(html, "data:image") {
		t.Fatalf("RenderMarkdown() kept data: image src: %s", html)
	}
}

func TestRenderMarkdownRemoteImagesRespectSwitch(t *testing.T) {
	source := "![remote](https://img.example.com/a.png)"
	strict, err := RenderMarkdown(source, false)
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	if strings.Contains(strict, "https://img.example.com") {
		t.Fatalf("RenderMarkdown(false) kept remote image src: %s", strict)
	}

	allowed, err := RenderMarkdown(source, true)
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	if !strings.Contains(allowed, `src="https://img.example.com/a.png"`) {
		t.Fatalf("RenderMarkdown(true) dropped remote image src: %s", allowed)
	}
}

func TestRenderMarkdownSanitizesNestedLinks(t *testing.T) {
	// 链接包裹在列表、引用等嵌套结构中，确保 transformer 递归生效。
	source := "- [点击](javascript:alert(2))\n\n> [y](https://ok.example)\n"
	html, err := RenderMarkdown(source, false)
	if err != nil {
		t.Fatalf("RenderMarkdown() error = %v", err)
	}
	if strings.Contains(html, "javascript:") {
		t.Fatalf("RenderMarkdown() kept nested javascript: link: %s", html)
	}
	if !strings.Contains(html, `href="https://ok.example"`) {
		t.Fatalf("RenderMarkdown() dropped nested safe link: %s", html)
	}
}

func TestURLScheme(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"javascript:alert(1)", "javascript"},
		{"HTTPS://example.com", "https"},
		{"mailto:a@b.c", "mailto"},
		{"guide/intro.md", ""},
		{"#anchor", ""},
		{"./rel", ""},
		{"", ""},
		{"1abc:def", ""}, // scheme 不能以数字开头
		{"a+b.c-d:e", "a+b.c-d"},
	}
	for _, tc := range cases {
		if got := urlScheme([]byte(tc.raw)); got != tc.want {
			t.Errorf("urlScheme(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
