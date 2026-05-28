package vault

type KDFParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

type EncryptedVault struct {
	Version   int                 `json:"version"`
	KDF       KDFParams           `json:"kdf"`
	Salt      []byte              `json:"salt"`
	Manifest  EncryptedPayload    `json:"manifest"`
	Documents []EncryptedDocument `json:"documents"`
}

type EncryptedPayload struct {
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

type EncryptedDocument struct {
	ID string `json:"id"`
	EncryptedPayload
}

type DocumentMetadata struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Path         string `json:"path"`
	DocumentType string `json:"documentType"`
	MimeType     string `json:"mimeType,omitempty"`
	Size         int64  `json:"size"`
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
}

type UnlockResponse struct {
	Tree []TreeNode `json:"tree"`
}
