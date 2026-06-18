package vault

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func testService(t *testing.T) *Service {
	t.Helper()
	encrypted, err := EncryptVault(PlainVault{
		Documents: []PlainDocument{
			{
				DocumentMetadata: DocumentMetadata{
					ID:           "intro",
					Title:        "Intro",
					Path:         "guide/intro.md",
					DocumentType: "markdown",
					MimeType:     "text/markdown; charset=utf-8",
					Size:         int64(len("# Intro\n\nHello **CryptoWitch**.\n<script>alert('x')</script>")),
				},
				Content: []byte("# Intro\n\nHello **CryptoWitch**.\n<script>alert('x')</script>"),
			},
			{
				DocumentMetadata: DocumentMetadata{
					ID:           "pdf",
					Title:        "Guide PDF",
					Path:         "guide/guide.pdf",
					DocumentType: "pdf",
					MimeType:     "application/pdf",
					Size:         int64(len(strings.Repeat("%PDF-1.4\n", 256*1024))),
				},
				Content: []byte(strings.Repeat("%PDF-1.4\n", 256*1024)),
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
	if len(service.documents) != 2 || len(service.payloads) != 2 {
		t.Fatalf("unlock should keep metadata and encrypted payload handles only")
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

func TestUnlockFailsWhenDeviceNotAuthorized(t *testing.T) {
	encrypted, err := EncryptVault(PlainVault{
		Documents: []PlainDocument{
			{
				DocumentMetadata: DocumentMetadata{
					ID:           "intro",
					Title:        "Intro",
					Path:         "guide/intro.md",
					DocumentType: "markdown",
					MimeType:     "text/markdown; charset=utf-8",
					Size:         int64(len("# Intro")),
				},
				Content: []byte("# Intro"),
			},
		},
	}, "correct-password", KDFParams{Time: 1, Memory: 64 * 1024, Threads: 1, KeyLen: 32})
	if err != nil {
		t.Fatalf("EncryptVault() error = %v", err)
	}
	encrypted.AllowedMACs = []string{"00:00:00:00:00:99"}
	service := NewService(encrypted)

	_, err = service.Unlock("correct-password")
	if !errors.Is(err, ErrDeviceNotAuthorized) {
		t.Fatalf("Unlock() error = %v, want ErrDeviceNotAuthorized", err)
	}
	if service.unlocked {
		t.Fatal("unlocked = true, want false after device rejection")
	}
	if _, err := service.GetTree(); !errors.Is(err, ErrLocked) {
		t.Fatalf("GetTree() error = %v, want ErrLocked", err)
	}
}

func TestUnlockSucceedsWithWildcardMAC(t *testing.T) {
	service := testService(t)
	service.allowedMACs = []string{"*"}

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if !service.unlocked {
		t.Fatal("unlocked = false, want true after wildcard unlock")
	}
}

func TestUnlockSucceedsWithEmptyMACWhitelist(t *testing.T) {
	service := testService(t)

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
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

func TestGetMarkdownDocument(t *testing.T) {
	service := testService(t)

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	document, err := service.GetDocument("intro")
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if document.DocumentType != "markdown" {
		t.Fatalf("DocumentType = %q, want markdown", document.DocumentType)
	}
	if document.MimeType != "text/markdown; charset=utf-8" {
		t.Fatalf("MimeType = %q, want text/markdown; charset=utf-8", document.MimeType)
	}
	if document.Size == 0 {
		t.Fatal("Size is empty")
	}
	if document.HTML == "" {
		t.Fatal("HTML is empty")
	}
	if document.ContentBase64 != "" {
		t.Fatalf("ContentBase64 = %q, want empty for markdown", document.ContentBase64)
	}
}

func TestGetPDFDocument(t *testing.T) {
	service := testService(t)

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	document, err := service.GetDocument("pdf")
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if document.DocumentType != "pdf" {
		t.Fatalf("DocumentType = %q, want pdf", document.DocumentType)
	}
	if document.MimeType != "application/pdf" {
		t.Fatalf("MimeType = %q, want application/pdf", document.MimeType)
	}
	if !document.Chunked {
		t.Fatal("Chunked = false, want chunked PDF response")
	}
	if document.ChunkSize != defaultPDFChunkSize {
		t.Fatalf("ChunkSize = %d, want %d", document.ChunkSize, defaultPDFChunkSize)
	}
	if document.ChunkCount != 3 {
		t.Fatalf("ChunkCount = %d, want 3", document.ChunkCount)
	}
	if document.ContentBase64 != "" {
		t.Fatalf("ContentBase64 = %q, want empty for chunked PDF", document.ContentBase64)
	}
	if document.Size != int64(len(strings.Repeat("%PDF-1.4\n", 256*1024))) {
		t.Fatalf("Size = %d, want original PDF size", document.Size)
	}
	if document.HTML != "" {
		t.Fatalf("HTML = %q, want empty for PDF", document.HTML)
	}
}

func TestGetPDFChunksReassemblesDocument(t *testing.T) {
	service := testService(t)
	expected := []byte(strings.Repeat("%PDF-1.4\n", 256*1024))

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	document, err := service.GetDocument("pdf")
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	var assembled []byte
	for index := 0; index < document.ChunkCount; index++ {
		chunk, err := service.GetPDFChunk("pdf", index)
		if err != nil {
			t.Fatalf("GetPDFChunk(%d) error = %v", index, err)
		}
		if chunk.Index != index || chunk.ChunkCount != document.ChunkCount {
			t.Fatalf("chunk metadata = %#v, want index %d and count %d", chunk, index, document.ChunkCount)
		}
		content, err := base64.StdEncoding.DecodeString(chunk.ContentBase64)
		if err != nil {
			t.Fatalf("DecodeString() error = %v", err)
		}
		if len(content) != chunk.Size {
			t.Fatalf("chunk size = %d, want %d", chunk.Size, len(content))
		}
		assembled = append(assembled, content...)
	}
	if string(assembled) != string(expected) {
		t.Fatal("assembled PDF content did not match original")
	}
}

func TestGetPDFChunkErrors(t *testing.T) {
	service := testService(t)

	if _, err := service.GetPDFChunk("pdf", 0); !errors.Is(err, ErrLocked) {
		t.Fatalf("GetPDFChunk() before unlock error = %v, want ErrLocked", err)
	}
	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if _, err := service.GetPDFChunk("missing", 0); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("GetPDFChunk() missing error = %v, want ErrDocumentNotFound", err)
	}
	if _, err := service.GetPDFChunk("intro", 0); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("GetPDFChunk() markdown error = %v, want ErrUnsupportedType", err)
	}
	if _, err := service.GetPDFChunk("pdf", -1); !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("GetPDFChunk() negative index error = %v, want ErrInvalidChunk", err)
	}
	if _, err := service.GetPDFChunk("pdf", 99); !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("GetPDFChunk() out of range error = %v, want ErrInvalidChunk", err)
	}
}

func TestLegacyV2PDFStillReturnsFullPayload(t *testing.T) {
	encrypted, err := encryptLegacyV2Vault(PlainVault{
		Documents: []PlainDocument{
			{
				DocumentMetadata: DocumentMetadata{
					ID:           "legacy-pdf",
					Title:        "Legacy PDF",
					Path:         "legacy.pdf",
					DocumentType: "pdf",
					MimeType:     "application/pdf",
					Size:         int64(len("%PDF-1.4")),
				},
				Content: []byte("%PDF-1.4"),
			},
		},
	}, "correct-password", KDFParams{Time: 1, Memory: 64 * 1024, Threads: 1, KeyLen: 32})
	if err != nil {
		t.Fatalf("encryptLegacyV2Vault() error = %v", err)
	}
	service := NewService(encrypted)
	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	document, err := service.GetDocument("legacy-pdf")
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if document.Chunked || document.ChunkCount != 0 {
		t.Fatalf("legacy document chunk metadata = %#v, want not chunked", document)
	}
	if document.ContentBase64 != "JVBERi0xLjQ=" {
		t.Fatalf("ContentBase64 = %q, want legacy full payload", document.ContentBase64)
	}
}

func TestGetMarkdownDocumentUsesRenderCache(t *testing.T) {
	service := testService(t)

	if _, err := service.Unlock("correct-password"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if _, err := service.GetDocument("intro"); err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if cached := service.htmlCache["intro"]; cached == "" {
		t.Fatal("expected cached markdown HTML")
	}
	if _, err := service.GetDocument("intro"); err != nil {
		t.Fatalf("GetDocument() cached error = %v", err)
	}
	service.Lock()
	if len(service.htmlCache) != 0 {
		t.Fatalf("htmlCache len = %d, want cleared after lock", len(service.htmlCache))
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

func encryptLegacyV2Vault(plain PlainVault, password string, params KDFParams) (EncryptedVault, error) {
	params = normalizeKDFParams(params)
	salt := []byte("legacy-v2-salt!!")
	key := deriveKey(password, salt, params)
	defer zeroBytes(key)
	aead, err := newAEAD(key)
	if err != nil {
		return EncryptedVault{}, err
	}

	manifest := DocumentManifest{Documents: make([]DocumentMetadata, 0, len(plain.Documents))}
	encryptedDocuments := make([]EncryptedDocument, 0, len(plain.Documents))
	for _, document := range plain.Documents {
		metadata := normalizeDocumentMetadata(document.DocumentMetadata, len(document.Content))
		manifest.Documents = append(manifest.Documents, metadata)
		payload, err := encryptPayload(aead, document.Content, documentAAD(legacyVaultVersion, metadata.ID))
		if err != nil {
			return EncryptedVault{}, err
		}
		encryptedDocuments = append(encryptedDocuments, EncryptedDocument{
			ID:               metadata.ID,
			EncryptedPayload: payload,
		})
	}

	manifestPayload, err := json.Marshal(manifest)
	if err != nil {
		return EncryptedVault{}, err
	}
	encryptedManifest, err := encryptPayload(aead, manifestPayload, manifestAAD(legacyVaultVersion))
	if err != nil {
		return EncryptedVault{}, err
	}
	return EncryptedVault{
		Version:   legacyVaultVersion,
		KDF:       params,
		Salt:      salt,
		Manifest:  encryptedManifest,
		Documents: encryptedDocuments,
	}, nil
}
