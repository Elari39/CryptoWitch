package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	currentVaultVersion = 1
	aesGCMNonceSize     = 12
	defaultKeyLength    = 32
)

var ErrInvalidPassword = errors.New("invalid password or vault data")

func EncryptVault(plain PlainVault, password string, params KDFParams) (EncryptedVault, error) {
	if password == "" {
		return EncryptedVault{}, errors.New("password is required")
	}
	params = normalizeKDFParams(params)

	payload, err := json.Marshal(plain)
	if err != nil {
		return EncryptedVault{}, fmt.Errorf("marshal vault: %w", err)
	}

	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return EncryptedVault{}, fmt.Errorf("generate salt: %w", err)
	}
	nonce := make([]byte, aesGCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedVault{}, fmt.Errorf("generate nonce: %w", err)
	}

	key := deriveKey(password, salt, params)
	defer zeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return EncryptedVault{}, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedVault{}, fmt.Errorf("create gcm: %w", err)
	}

	return EncryptedVault{
		Version:    currentVaultVersion,
		KDF:        params,
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: aead.Seal(nil, nonce, payload, nil),
	}, nil
}

func DecryptVault(encrypted EncryptedVault, password string) (PlainVault, error) {
	if password == "" || encrypted.Version != currentVaultVersion {
		return PlainVault{}, ErrInvalidPassword
	}
	params := normalizeKDFParams(encrypted.KDF)
	if len(encrypted.Salt) == 0 || len(encrypted.Nonce) != aesGCMNonceSize || len(encrypted.Ciphertext) == 0 {
		return PlainVault{}, ErrInvalidPassword
	}

	key := deriveKey(password, encrypted.Salt, params)
	defer zeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return PlainVault{}, ErrInvalidPassword
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return PlainVault{}, ErrInvalidPassword
	}
	payload, err := aead.Open(nil, encrypted.Nonce, encrypted.Ciphertext, nil)
	if err != nil {
		return PlainVault{}, ErrInvalidPassword
	}

	var plain PlainVault
	if err := json.Unmarshal(payload, &plain); err != nil {
		return PlainVault{}, ErrInvalidPassword
	}
	return plain, nil
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
