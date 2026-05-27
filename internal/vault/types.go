package vault

type KDFParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

type EncryptedVault struct {
	Version    int       `json:"version"`
	KDF        KDFParams `json:"kdf"`
	Salt       []byte    `json:"salt"`
	Nonce      []byte    `json:"nonce"`
	Ciphertext []byte    `json:"ciphertext"`
}

type PlainVault struct {
	Documents []PlainDocument `json:"documents"`
}

type PlainDocument struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type TreeNode struct {
	ID       string     `json:"id,omitempty"`
	Title    string     `json:"title"`
	Path     string     `json:"path"`
	Kind     string     `json:"kind"`
	Children []TreeNode `json:"children,omitempty"`
}

type DocumentResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	HTML  string `json:"html"`
}

type UnlockResponse struct {
	Tree []TreeNode `json:"tree"`
}
