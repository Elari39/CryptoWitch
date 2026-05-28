package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRequiresPasswordEnvironment(t *testing.T) {
	t.Setenv(vaultPasswordEnv, "")
	root := t.TempDir()
	configPath := writeTestFile(t, root, "config.yaml", "app:\n  title: CryptoWitch\nvault:\n  kdf:\n    time: 1\n")
	contentDir := filepath.Join(root, "content")
	writeTestFile(t, contentDir, "intro.md", "# Intro\n")

	err := run(configPath, contentDir, filepath.Join(root, "generated.go"))
	if err == nil || !strings.Contains(err.Error(), vaultPasswordEnv) {
		t.Fatalf("run() error = %v, want missing %s", err, vaultPasswordEnv)
	}
}

func TestReadDocumentsSupportsMarkdownPDFAndSkipsOtherFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "guide/intro.md", "# Intro\n\nHello")
	writeTestFile(t, root, "guide/manual.pdf", "%PDF-1.4")
	writeTestFile(t, root, "guide/notes.txt", "skip")

	documents, err := readDocuments(root)
	if err != nil {
		t.Fatalf("readDocuments() error = %v", err)
	}
	if len(documents) != 2 {
		t.Fatalf("len(documents) = %d, want 2: %#v", len(documents), documents)
	}

	markdown := documents[0]
	if markdown.DocumentType != "markdown" || markdown.MimeType != "text/markdown; charset=utf-8" {
		t.Fatalf("markdown metadata = %#v", markdown)
	}
	if markdown.Title != "Intro" || string(markdown.Content) != "# Intro\n\nHello" || markdown.Size != int64(len("# Intro\n\nHello")) {
		t.Fatalf("markdown document = %#v", markdown)
	}

	pdf := documents[1]
	if pdf.DocumentType != "pdf" || pdf.MimeType != "application/pdf" {
		t.Fatalf("pdf metadata = %#v", pdf)
	}
	if pdf.Title != "manual" || string(pdf.Content) != "%PDF-1.4" || pdf.Size != int64(len("%PDF-1.4")) {
		t.Fatalf("pdf document = %#v", pdf)
	}
}

func TestReadDocumentsReturnsEmptyForUnsupportedOnly(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "notes.txt", "skip")

	documents, err := readDocuments(root)
	if err != nil {
		t.Fatalf("readDocuments() error = %v", err)
	}
	if len(documents) != 0 {
		t.Fatalf("len(documents) = %d, want 0", len(documents))
	}
}

func TestRunErrorsWhenNoSupportedDocumentsFound(t *testing.T) {
	t.Setenv(vaultPasswordEnv, "test-password")
	root := t.TempDir()
	configPath := writeTestFile(t, root, "config.yaml", "app:\n  title: CryptoWitch\nvault:\n  kdf:\n    time: 1\n")
	contentDir := filepath.Join(root, "content")
	writeTestFile(t, contentDir, "notes.txt", "skip")

	err := run(configPath, contentDir, filepath.Join(root, "generated.go"))
	if err == nil || !strings.Contains(err.Error(), "no supported documents found") {
		t.Fatalf("run() error = %v, want no supported documents found", err)
	}
}

func writeTestFile(t *testing.T, root string, name string, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
