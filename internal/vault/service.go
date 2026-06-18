package vault

import (
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"sync"
)

var (
	ErrLocked              = errors.New("vault is locked")
	ErrDocumentNotFound    = errors.New("document not found")
	ErrUnsupportedType     = errors.New("unsupported document type")
	ErrInvalidChunk        = errors.New("invalid pdf chunk")
	ErrDeviceNotAuthorized = errors.New("device not authorized")
)

type Service struct {
	mu          sync.RWMutex
	encrypted   EncryptedVault
	unlocked    bool
	session     uint64
	aead        cipher.AEAD
	version     int
	documents   map[string]DocumentMetadata
	payloads    map[string]EncryptedDocument
	htmlCache   map[string]string
	tree        []TreeNode
	allowedMACs []string
}

func NewService(encrypted EncryptedVault) *Service {
	return &Service{
		encrypted:   encrypted,
		allowedMACs: encrypted.AllowedMACs,
		documents:   make(map[string]DocumentMetadata),
		payloads:    make(map[string]EncryptedDocument),
		htmlCache:   make(map[string]string),
	}
}

func (s *Service) Unlock(password string) (UnlockResponse, error) {
	if err := s.verifyDevice(); err != nil {
		s.clearUnlockedState()
		return UnlockResponse{}, err
	}

	manifest, aead, err := decryptManifestWithPassword(s.encrypted, password)
	if err != nil {
		s.clearUnlockedState()
		return UnlockResponse{}, ErrInvalidPassword
	}

	documents := make(map[string]DocumentMetadata, len(manifest.Documents))
	for _, document := range manifest.Documents {
		documents[document.ID] = document
	}
	payloads := encryptedDocumentsByID(s.encrypted.Documents)
	tree := BuildTree(manifest.Documents)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.aead = aead
	s.version = s.encrypted.Version
	s.documents = documents
	s.payloads = payloads
	s.htmlCache = make(map[string]string)
	s.tree = tree
	s.unlocked = true
	s.session++

	return UnlockResponse{Tree: cloneTree(tree)}, nil
}

func (s *Service) Lock() {
	s.clearUnlockedState()
}

func (s *Service) clearUnlockedState() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unlocked = false
	s.session++
	s.aead = nil
	s.version = 0
	s.documents = make(map[string]DocumentMetadata)
	s.payloads = make(map[string]EncryptedDocument)
	s.htmlCache = make(map[string]string)
	s.tree = nil
}

func (s *Service) GetTree() ([]TreeNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.unlocked {
		return nil, ErrLocked
	}
	return cloneTree(s.tree), nil
}

func (s *Service) GetDocument(id string) (DocumentResponse, error) {
	s.mu.RLock()
	if !s.unlocked {
		s.mu.RUnlock()
		return DocumentResponse{}, ErrLocked
	}
	document, ok := s.documents[id]
	payload, hasPayload := s.payloads[id]
	aead := s.aead
	version := s.version
	cachedHTML := s.htmlCache[id]
	session := s.session
	s.mu.RUnlock()
	if !ok || !hasPayload || aead == nil {
		return DocumentResponse{}, ErrDocumentNotFound
	}

	response := DocumentResponse{
		ID:           document.ID,
		Title:        document.Title,
		DocumentType: document.DocumentType,
		MimeType:     document.MimeType,
		Size:         document.Size,
	}
	switch document.DocumentType {
	case "", "markdown":
		if cachedHTML == "" {
			content, err := decryptDocumentPayloadWithAEAD(version, payload, document, aead)
			if err != nil {
				return DocumentResponse{}, ErrInvalidPassword
			}
			rendered, err := RenderMarkdown(string(content))
			zeroBytes(content)
			if err != nil {
				return DocumentResponse{}, err
			}
			cachedHTML = rendered
			s.cacheMarkdownHTML(id, rendered, session)
		}
		response.DocumentType = "markdown"
		response.MimeType = "text/markdown; charset=utf-8"
		response.HTML = cachedHTML
	case "pdf":
		response.MimeType = "application/pdf"
		response.Chunked = document.Chunked
		response.ChunkSize = document.ChunkSize
		response.ChunkCount = document.ChunkCount
		if !document.Chunked {
			content, err := decryptDocumentPayloadWithAEAD(version, payload, document, aead)
			if err != nil {
				return DocumentResponse{}, ErrInvalidPassword
			}
			response.ContentBase64 = base64.StdEncoding.EncodeToString(content)
			zeroBytes(content)
		}
	default:
		return DocumentResponse{}, ErrUnsupportedType
	}
	if !s.isCurrentSession(session) {
		return DocumentResponse{}, ErrLocked
	}
	return response, nil
}

func (s *Service) GetPDFChunk(id string, index int) (PDFChunkResponse, error) {
	s.mu.RLock()
	if !s.unlocked {
		s.mu.RUnlock()
		return PDFChunkResponse{}, ErrLocked
	}
	document, ok := s.documents[id]
	payload, hasPayload := s.payloads[id]
	aead := s.aead
	version := s.version
	session := s.session
	s.mu.RUnlock()
	if !ok || !hasPayload || aead == nil {
		return PDFChunkResponse{}, ErrDocumentNotFound
	}
	if document.DocumentType != "pdf" || !document.Chunked {
		return PDFChunkResponse{}, ErrUnsupportedType
	}
	if index < 0 || index >= document.ChunkCount || index >= len(payload.Chunks) {
		return PDFChunkResponse{}, ErrInvalidChunk
	}

	content, err := decryptDocumentChunkWithAEAD(version, payload, document, index, aead)
	if err != nil {
		return PDFChunkResponse{}, ErrInvalidPassword
	}
	defer zeroBytes(content)

	if !s.isCurrentSession(session) {
		return PDFChunkResponse{}, ErrLocked
	}
	return PDFChunkResponse{
		ID:            document.ID,
		Index:         index,
		Offset:        int64(index * document.ChunkSize),
		Size:          len(content),
		ChunkCount:    document.ChunkCount,
		ContentBase64: base64.StdEncoding.EncodeToString(content),
	}, nil
}

func (s *Service) cacheMarkdownHTML(id string, html string, session uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unlocked && s.session == session {
		s.htmlCache[id] = html
	}
}

func (s *Service) isCurrentSession(session uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unlocked && s.session == session
}

func cloneTree(nodes []TreeNode) []TreeNode {
	if nodes == nil {
		return nil
	}
	copied := make([]TreeNode, len(nodes))
	copy(copied, nodes)
	for i := range copied {
		copied[i].Children = cloneTree(copied[i].Children)
	}
	return copied
}
