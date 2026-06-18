package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cryptowitch/internal/vault"
)

func TestRunRequiresAccessPassword(t *testing.T) {
	root := t.TempDir()
	configPath := writeTestFile(t, root, "config.yaml", "app:\n  title: CryptoWitch\nvault:\n  kdf:\n    time: 1\n")
	accessPath := writeTestFile(t, root, "access.yaml", "password: \"\"\nallowedMACs:\n  - \"*\"\n")
	contentDir := filepath.Join(root, "content")
	writeTestFile(t, contentDir, "intro.md", "# Intro\n")

	err := run(configPath, accessPath, contentDir, filepath.Join(root, "generated.go"))
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("run() error = %v, want missing password", err)
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
	if markdown.Title != "intro" || string(markdown.Content) != "# Intro\n\nHello" || markdown.Size != int64(len("# Intro\n\nHello")) {
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
	root := t.TempDir()
	configPath := writeTestFile(t, root, "config.yaml", "app:\n  title: CryptoWitch\nvault:\n  kdf:\n    time: 1\n")
	accessPath := writeTestFile(t, root, "access.yaml", "password: \"test-password\"\nallowedMACs:\n  - \"*\"\n")
	contentDir := filepath.Join(root, "content")
	writeTestFile(t, contentDir, "notes.txt", "skip")

	err := run(configPath, accessPath, contentDir, filepath.Join(root, "generated.go"))
	if err == nil || !strings.Contains(err.Error(), "no supported documents found") {
		t.Fatalf("run() error = %v, want no supported documents found", err)
	}
}

func TestRenderGeneratedVaultIncludesPDFChunks(t *testing.T) {
	documents := []vault.PlainDocument{
		{
			DocumentMetadata: vault.DocumentMetadata{
				ID:           "pdf",
				Title:        "Manual",
				Path:         "manual.pdf",
				DocumentType: "pdf",
				MimeType:     "application/pdf",
				Size:         int64(len("%PDF-1.4")),
			},
			Content: []byte("%PDF-1.4"),
		},
	}
	encrypted, err := vault.EncryptVault(vault.PlainVault{Documents: documents}, "password", vault.KDFParams{Time: 1, Memory: 64 * 1024, Threads: 1, KeyLen: 32})
	if err != nil {
		t.Fatalf("EncryptVault() error = %v", err)
	}
	encrypted.AllowedMACs = []string{"AA:BB:CC:DD:EE:FF", "*"}
	generated, err := renderGeneratedVault(encrypted)
	if err != nil {
		t.Fatalf("renderGeneratedVault() error = %v", err)
	}
	source := string(generated)
	if !strings.Contains(source, "Version: 3") {
		t.Fatalf("generated source missing v3 marker: %s", source)
	}
	if !strings.Contains(source, "Chunks: []EncryptedPayload") {
		t.Fatalf("generated source missing PDF chunks: %s", source)
	}
	if !strings.Contains(source, "AllowedMACs: []string{") || !strings.Contains(source, `"*"`) {
		t.Fatalf("generated source missing AllowedMACs: %s", source)
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
