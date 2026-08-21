package vault

import (
	"context"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrLocked              = errors.New("vault is locked")
	ErrDocumentNotFound    = errors.New("document not found")
	ErrUnsupportedType     = errors.New("unsupported document type")
	ErrInvalidChunk        = errors.New("invalid pdf chunk")
	ErrDeviceNotAuthorized = errors.New("device not authorized")
	ErrAINotConfigured     = errors.New("ai is not configured")
	// ErrTooManyAttempts 表示密码连续失败次数过多，处于冷却期内。
	ErrTooManyAttempts = errors.New("too many attempts, please try again later")
	// ErrVaultCorrupted 表示 vault 数据损坏（密码正确但解密/校验失败），与密码错误区分。
	ErrVaultCorrupted = errors.New("vault data corrupted")
)

// maxHTMLCacheEntries 限制解锁会话内 Markdown 渲染结果缓存条数，
// 避免大 vault 全部打开后 HTML 缓存无限增长。
const maxHTMLCacheEntries = 32

// 解锁限速：连续失败达到 maxUnlockFailures 次后进入冷却，
// 冷却时长 = unlockCooldownBase 每次翻倍，最多 4 次翻倍（最长约 8 分钟）。
// 冷却仅存在于进程内存中（重启即重置），用于抑制在线暴力猜解。
const (
	maxUnlockFailures  = 5
	unlockCooldownBase = 30 * time.Second
)

type Service struct {
	mu                  sync.RWMutex
	encrypted           EncryptedVault
	unlocked            bool
	session             uint64
	aead                cipher.AEAD
	version             int
	documents           map[string]DocumentMetadata
	payloads            map[string]EncryptedDocument
	htmlCache           map[string]string
	htmlCacheOrder      []string
	tree                []TreeNode
	allowedMACs         []string
	aiConfig            AIConfig
	allowRemoteImages   bool
	aiCancel            context.CancelFunc
	unlockFailures      int
	unlockCooldownUntil time.Time
}

func NewService(encrypted EncryptedVault) *Service {
	return &Service{
		encrypted:         encrypted,
		allowedMACs:       encrypted.AllowedMACs,
		aiConfig:          encrypted.AIConfig,
		allowRemoteImages: encrypted.AllowRemoteImages,
		documents:         make(map[string]DocumentMetadata),
		payloads:          make(map[string]EncryptedDocument),
		htmlCache:         make(map[string]string),
	}
}

func (s *Service) Unlock(password string) (UnlockResponse, error) {
	// 冷却期内直接拒绝，避免在线暴力猜解（设备校验之前先检查，省去 KDF 开销）。
	if wait := s.unlockWaitRemaining(); wait > 0 {
		return UnlockResponse{}, fmt.Errorf("%w（约 %d 秒后重试）", ErrTooManyAttempts, int(wait.Seconds())+1)
	}
	if err := s.verifyDevice(); err != nil {
		s.clearUnlockedState()
		return UnlockResponse{}, err
	}

	manifest, aead, err := decryptManifestWithPassword(s.encrypted, password)
	if err != nil {
		s.noteUnlockFailure()
		s.clearUnlockedState()
		return UnlockResponse{}, ErrInvalidPassword
	}
	s.resetUnlockThrottle()

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
	s.htmlCacheOrder = nil
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
	// 取消进行中的 AI 流式请求，避免 goroutine 在锁定后仍挂起至总超时。
	if s.aiCancel != nil {
		s.aiCancel()
		s.aiCancel = nil
	}
	s.unlocked = false
	s.session++
	s.aead = nil
	s.version = 0
	s.documents = make(map[string]DocumentMetadata)
	s.payloads = make(map[string]EncryptedDocument)
	s.htmlCache = make(map[string]string)
	s.htmlCacheOrder = nil
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
				return DocumentResponse{}, ErrVaultCorrupted
			}
			rendered, err := RenderMarkdown(string(content), s.allowRemoteImages)
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
				return DocumentResponse{}, ErrVaultCorrupted
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
		return PDFChunkResponse{}, ErrVaultCorrupted
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
	if !s.unlocked || s.session != session {
		return
	}
	if _, exists := s.htmlCache[id]; exists {
		s.htmlCache[id] = html
		return
	}
	s.htmlCache[id] = html
	s.htmlCacheOrder = append(s.htmlCacheOrder, id)
	// 超过上限时按插入顺序淘汰最旧条目，防止会话内缓存无限增长。
	if len(s.htmlCacheOrder) > maxHTMLCacheEntries {
		evicted := s.htmlCacheOrder[0]
		s.htmlCacheOrder = s.htmlCacheOrder[1:]
		delete(s.htmlCache, evicted)
	}
}

func (s *Service) isCurrentSession(session uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unlocked && s.session == session
}

// unlockWaitRemaining 返回解锁冷却剩余时长（0 表示可直接尝试）。
func (s *Service) unlockWaitRemaining() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unlockCooldownUntil.IsZero() {
		return 0
	}
	return time.Until(s.unlockCooldownUntil)
}

// noteUnlockFailure 记录一次密码失败；连续失败达到阈值后进入冷却，
// 冷却时长随超额次数指数递增（上限 4 次翻倍）。
func (s *Service) noteUnlockFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unlockFailures++
	if s.unlockFailures >= maxUnlockFailures {
		extra := s.unlockFailures - maxUnlockFailures
		if extra > 4 {
			extra = 4
		}
		s.unlockCooldownUntil = time.Now().Add(unlockCooldownBase << extra)
	}
}

// resetUnlockThrottle 解锁成功后清空失败计数与冷却。
func (s *Service) resetUnlockThrottle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unlockFailures = 0
	s.unlockCooldownUntil = time.Time{}
}

// GetSecurityPolicy 返回运行时安全策略信息（不含任何密钥或文档内容），
// 供宿主（main.go）注入 Content-Security-Policy 使用。
func (s *Service) GetSecurityPolicy() (SecurityPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SecurityPolicy{AllowRemoteImages: s.allowRemoteImages}, nil
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
