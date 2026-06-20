package vault

type KDFParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

type EncryptedVault struct {
	Version     int                 `json:"version"`
	KDF         KDFParams           `json:"kdf"`
	Salt        []byte              `json:"salt"`
	Manifest    EncryptedPayload    `json:"manifest"`
	Documents   []EncryptedDocument `json:"documents"`
	AllowedMACs []string            `json:"allowedMacs,omitempty"`
	AIConfig    AIConfig            `json:"aiConfig,omitempty"`
}

// AIConfig 是划词 AI 解读的服务凭证，构建期由 access.yaml 注入并嵌入 generated.go。
type AIConfig struct {
	Endpoint string `json:"endpoint,omitempty"`
	ApiKey   string `json:"apiKey,omitempty"`
	Model    string `json:"model,omitempty"`
}

type EncryptedPayload struct {
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

type EncryptedDocument struct {
	ID string `json:"id"`
	EncryptedPayload
	Chunks []EncryptedPayload `json:"chunks,omitempty"`
}

type DocumentMetadata struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Path         string `json:"path"`
	DocumentType string `json:"documentType"`
	MimeType     string `json:"mimeType,omitempty"`
	Size         int64  `json:"size"`
	Chunked      bool   `json:"chunked,omitempty"`
	ChunkSize    int    `json:"chunkSize,omitempty"`
	ChunkCount   int    `json:"chunkCount,omitempty"`
}

type DocumentManifest struct {
	Documents []DocumentMetadata `json:"documents"`
}

type PlainVault struct {
	Documents []PlainDocument `json:"documents"`
}

type PlainDocument struct {
	DocumentMetadata
	Content []byte `json:"content,omitempty"`
}

type TreeNode struct {
	ID           string     `json:"id,omitempty"`
	Title        string     `json:"title"`
	Path         string     `json:"path"`
	Kind         string     `json:"kind"`
	DocumentType string     `json:"documentType,omitempty"`
	MimeType     string     `json:"mimeType,omitempty"`
	Size         int64      `json:"size,omitempty"`
	Children     []TreeNode `json:"children,omitempty"`
}

type DocumentResponse struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	DocumentType  string `json:"documentType"`
	MimeType      string `json:"mimeType,omitempty"`
	Size          int64  `json:"size"`
	HTML          string `json:"html,omitempty"`
	ContentBase64 string `json:"contentBase64,omitempty"`
	Chunked       bool   `json:"chunked,omitempty"`
	ChunkSize     int    `json:"chunkSize,omitempty"`
	ChunkCount    int    `json:"chunkCount,omitempty"`
}

type PDFChunkResponse struct {
	ID            string `json:"id"`
	Index         int    `json:"index"`
	Offset        int64  `json:"offset"`
	Size          int    `json:"size"`
	ChunkCount    int    `json:"chunkCount"`
	ContentBase64 string `json:"contentBase64"`
}

type UnlockResponse struct {
	Tree []TreeNode `json:"tree"`
}

// AIMessage 是一条对话消息，role 为 user 或 assistant。
type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIChatRequest 由前端发起的划词 AI 解读请求。
type AIChatRequest struct {
	RequestID    int        `json:"requestId"`
	DocumentID   string     `json:"documentId"`
	SelectedText string     `json:"selectedText"`
	Question     string     `json:"question"`
	History      []AIMessage `json:"history"`
}

// AIInfo 返回当前可用的 AI 配置信息（不含 apiKey），供前端展示与可用性判断。
type AIInfo struct {
	Available bool   `json:"available"`
	Model     string `json:"model,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
}
