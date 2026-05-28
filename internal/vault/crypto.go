package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	currentVaultVersion = 2
	aesGCMNonceSize     = 12
	defaultKeyLength    = 32
)

var ErrInvalidPassword = errors.New("invalid password or vault data")

func EncryptVault(plain PlainVault, password string, params KDFParams) (EncryptedVault, error) {
	if password == "" {
		return EncryptedVault{}, errors.New("password is required")
	}
	params = normalizeKDFParams(params)

	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return EncryptedVault{}, fmt.Errorf("generate salt: %w", err)
	}

	key := deriveKey(password, salt, params)
	defer zeroBytes(key)

	aead, err := newAEAD(key)
	if err != nil {
		return EncryptedVault{}, err
	}

	manifest := DocumentManifest{
		Documents: make([]DocumentMetadata, 0, len(plain.Documents)),
	}
	encryptedDocuments := make([]EncryptedDocument, 0, len(plain.Documents))
	for _, document := range plain.Documents {
		metadata := normalizeDocumentMetadata(document.DocumentMetadata, len(document.Content))
		manifest.Documents = append(manifest.Documents, metadata)

		encryptedPayload, err := encryptPayload(aead, document.Content, documentAAD(metadata.ID))
		if err != nil {
			return EncryptedVault{}, fmt.Errorf("encrypt document %s: %w", metadata.Path, err)
		}
		encryptedDocuments = append(encryptedDocuments, EncryptedDocument{
			ID:               metadata.ID,
			EncryptedPayload: encryptedPayload,
		})
	}

	manifestPayload, err := json.Marshal(manifest)
	if err != nil {
		return EncryptedVault{}, fmt.Errorf("marshal manifest: %w", err)
	}
	encryptedManifest, err := encryptPayload(aead, manifestPayload, manifestAAD())
	if err != nil {
		return EncryptedVault{}, fmt.Errorf("encrypt manifest: %w", err)
	}

	return EncryptedVault{
		Version:   currentVaultVersion,
		KDF:       params,
		Salt:      salt,
		Manifest:  encryptedManifest,
		Documents: encryptedDocuments,
	}, nil
}

func DecryptVault(encrypted EncryptedVault, password string) (PlainVault, error) {
	aead, key, err := unlockAEAD(encrypted, password)
	if err != nil {
		return PlainVault{}, err
	}
	defer zeroBytes(key)

	manifest, err := decryptManifestWithAEAD(encrypted, aead)
	if err != nil {
		return PlainVault{}, ErrInvalidPassword
	}

	encryptedDocuments := encryptedDocumentsByID(encrypted.Documents)
	plain := PlainVault{Documents: make([]PlainDocument, 0, len(manifest.Documents))}
	for _, metadata := range manifest.Documents {
		encryptedDocument, ok := encryptedDocuments[metadata.ID]
		if !ok {
			return PlainVault{}, ErrInvalidPassword
		}
		payload, err := decryptDocumentPayloadWithAEAD(encryptedDocument, aead)
		if err != nil {
			return PlainVault{}, ErrInvalidPassword
		}
		plain.Documents = append(plain.Documents, PlainDocument{
			DocumentMetadata: metadata,
			Content:          payload,
		})
	}
	return plain, nil
}

func decryptManifestWithPassword(encrypted EncryptedVault, password string) (DocumentManifest, cipher.AEAD, error) {
	aead, key, err := unlockAEAD(encrypted, password)
	if err != nil {
		return DocumentManifest{}, nil, err
	}
	defer zeroBytes(key)
	manifest, err := decryptManifestWithAEAD(encrypted, aead)
	if err != nil {
		return DocumentManifest{}, nil, ErrInvalidPassword
	}
	return manifest, aead, nil
}

func decryptManifestWithAEAD(encrypted EncryptedVault, aead cipher.AEAD) (DocumentManifest, error) {
	payload, err := decryptPayload(aead, encrypted.Manifest, manifestAAD())
	if err != nil {
		return DocumentManifest{}, ErrInvalidPassword
	}
	var manifest DocumentManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return DocumentManifest{}, ErrInvalidPassword
	}
	return manifest, nil
}

func decryptDocumentPayloadWithAEAD(document EncryptedDocument, aead cipher.AEAD) ([]byte, error) {
	return decryptPayload(aead, document.EncryptedPayload, documentAAD(document.ID))
}

func unlockAEAD(encrypted EncryptedVault, password string) (cipher.AEAD, []byte, error) {
	if password == "" || encrypted.Version != currentVaultVersion {
		return nil, nil, ErrInvalidPassword
	}
	params := normalizeKDFParams(encrypted.KDF)
	if len(encrypted.Salt) == 0 || !validEncryptedPayload(encrypted.Manifest) {
		return nil, nil, ErrInvalidPassword
	}
	key := deriveKey(password, encrypted.Salt, params)
	aead, err := newAEAD(key)
	if err != nil {
		zeroBytes(key)
		return nil, nil, ErrInvalidPassword
	}
	return aead, key, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return aead, nil
}

func encryptPayload(aead cipher.AEAD, payload []byte, additionalData []byte) (EncryptedPayload, error) {
	nonce := make([]byte, aesGCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedPayload{}, fmt.Errorf("generate nonce: %w", err)
	}
	return EncryptedPayload{
		Nonce:      nonce,
		Ciphertext: aead.Seal(nil, nonce, payload, additionalData),
	}, nil
}

func decryptPayload(aead cipher.AEAD, payload EncryptedPayload, additionalData []byte) ([]byte, error) {
	if !validEncryptedPayload(payload) {
		return nil, ErrInvalidPassword
	}
	plain, err := aead.Open(nil, payload.Nonce, payload.Ciphertext, additionalData)
	if err != nil {
		return nil, ErrInvalidPassword
	}
	return plain, nil
}

func validEncryptedPayload(payload EncryptedPayload) bool {
	return len(payload.Nonce) == aesGCMNonceSize && len(payload.Ciphertext) > 0
}

func encryptedDocumentsByID(documents []EncryptedDocument) map[string]EncryptedDocument {
	byID := make(map[string]EncryptedDocument, len(documents))
	for _, document := range documents {
		byID[document.ID] = document
	}
	return byID
}

func normalizeDocumentMetadata(metadata DocumentMetadata, contentLength int) DocumentMetadata {
	if metadata.DocumentType == "" {
		metadata.DocumentType = "markdown"
	}
	if metadata.MimeType == "" {
		switch metadata.DocumentType {
		case "pdf":
			metadata.MimeType = "application/pdf"
		default:
			metadata.MimeType = "text/markdown; charset=utf-8"
		}
	}
	if metadata.Size == 0 {
		metadata.Size = int64(contentLength)
	}
	return metadata
}

func manifestAAD() []byte {
	return []byte("cryptowitch:manifest:v2")
}

func documentAAD(id string) []byte {
	return []byte("cryptowitch:document:v2:" + base64.RawURLEncoding.EncodeToString([]byte(id)))
}

func deriveKey(password string, salt []byte, params KDFParams) []byte {
	params = normalizeKDFParams(params)
	return argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
}

func normalizeKDFParams(params KDFParams) KDFParams {
	if params.Time == 0 {
		params.Time = 3
	}
	if params.Memory == 0 {
		params.Memory = 64 * 1024
	}
	if params.Threads == 0 {
		params.Threads = 4
	}
	if params.KeyLen == 0 {
		params.KeyLen = defaultKeyLength
	}
	return params
}

func zeroBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
