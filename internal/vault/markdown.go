package vault

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
)

func RenderMarkdown(source string) (string, error) {
	var output bytes.Buffer
	if err := markdownRenderer.Convert([]byte(source), &output); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return output.String(), nil
}
