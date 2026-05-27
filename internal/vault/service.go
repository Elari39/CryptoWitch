package vault

import (
	"errors"
	"sync"
)

var (
	ErrLocked           = errors.New("vault is locked")
	ErrDocumentNotFound = errors.New("document not found")
)

type Service struct {
	mu        sync.RWMutex
	encrypted EncryptedVault
	unlocked  bool
	documents map[string]PlainDocument
	tree      []TreeNode
}

func NewService(encrypted EncryptedVault) *Service {
	return &Service{
		encrypted: encrypted,
		documents: make(map[string]PlainDocument),
	}
}

func (s *Service) Unlock(password string) (UnlockResponse, error) {
	plain, err := DecryptVault(s.encrypted, password)
	if err != nil {
		s.clearUnlockedState()
		return UnlockResponse{}, ErrInvalidPassword
	}

	documents := make(map[string]PlainDocument, len(plain.Documents))
	for _, document := range plain.Documents {
		documents[document.ID] = document
	}
	tree := BuildTree(plain.Documents)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents = documents
	s.tree = tree
	s.unlocked = true

	return UnlockResponse{Tree: cloneTree(tree)}, nil
}

func (s *Service) Lock() {
	s.clearUnlockedState()
}

func (s *Service) clearUnlockedState() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unlocked = false
	s.documents = make(map[string]PlainDocument)
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
	s.mu.RUnlock()
	if !ok {
		return DocumentResponse{}, ErrDocumentNotFound
	}

	rendered, err := RenderMarkdown(document.Content)
	if err != nil {
		return DocumentResponse{}, err
	}
	return DocumentResponse{
		ID:    document.ID,
		Title: document.Title,
		HTML:  rendered,
	}, nil
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
