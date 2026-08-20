package vault

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// markdownSanitizer 是 goldmark AST transformer，负责消毒链接与图片 URL：
// 仅保留 http/https/mailto 与无 scheme 的相对路径（含 # 锚点）；
// javascript:、data:、vbscript:、file: 等其余 scheme 一律清空。
// 远程图片（http/https）仅在 allowRemoteImages 时保留，否则清空 src。
type markdownSanitizer struct {
	allowRemoteImages bool
}

// Transform 实现 parser.ASTTransformer，遍历整棵 AST 并消毒 Link / Image 的 destination。
func (s *markdownSanitizer) Transform(document *ast.Document, _ text.Reader, _ parser.Context) {
	sanitizeNode(document, s.allowRemoteImages)
}

func sanitizeNode(node ast.Node, allowRemoteImages bool) {
	switch n := node.(type) {
	case *ast.Link:
		n.Destination = sanitizeLinkDestination(n.Destination)
	case *ast.Image:
		n.Destination = sanitizeImageDestination(n.Destination, allowRemoteImages)
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		sanitizeNode(child, allowRemoteImages)
	}
}

// sanitizeLinkDestination 白名单：无 scheme（相对路径 / 锚点）、http、https、mailto。
func sanitizeLinkDestination(destination []byte) []byte {
	switch urlScheme(destination) {
	case "", "http", "https", "mailto":
		return destination
	default:
		return nil
	}
}

// sanitizeImageDestination 图片仅允许相对路径与 http/https；
// 远程图片是否放行由 allowRemoteImages 决定（默认禁止，隐私优先）。
func sanitizeImageDestination(destination []byte, allowRemoteImages bool) []byte {
	switch urlScheme(destination) {
	case "":
		return destination
	case "http", "https":
		if allowRemoteImages {
			return destination
		}
		return nil
	default:
		return nil
	}
}

// urlScheme 提取 URL 的 scheme（小写）；无 scheme 或非法 scheme 返回空串。
// 例如 "javascript:alert(1)" -> "javascript"；"guide/intro.md" -> ""；"#anchor" -> ""。
func urlScheme(raw []byte) string {
	value := string(raw)
	colon := strings.IndexByte(value, ':')
	if colon <= 0 {
		return ""
	}
	scheme := value[:colon]
	for index, r := range scheme {
		isValid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			(index > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'))
		if !isValid {
			return ""
		}
	}
	return strings.ToLower(scheme)
}

func newMarkdownRenderer(allowRemoteImages bool) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("friendly"),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&markdownSanitizer{allowRemoteImages: allowRemoteImages}, 100),
			),
		),
	)
}

var (
	markdownStrict = newMarkdownRenderer(false)
	markdownRemote = newMarkdownRenderer(true)
)

// RenderMarkdown 把 Markdown 渲染为安全 HTML。
// allowRemoteImages 为 true 时允许文档中的远程图片（http/https）被引用，否则其 src 会被清空。
func RenderMarkdown(source string, allowRemoteImages bool) (string, error) {
	renderer := markdownStrict
	if allowRemoteImages {
		renderer = markdownRemote
	}
	var output bytes.Buffer
	if err := renderer.Convert([]byte(source), &output); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return output.String(), nil
}
