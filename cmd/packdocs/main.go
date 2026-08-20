package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cryptowitch/internal/vault"
	"gopkg.in/yaml.v3"
)

const defaultAccessPath = "access.yaml"

type vaultConfig struct {
	Vault struct {
		KDF struct {
			Time    uint32 `yaml:"time"`
			Memory  uint32 `yaml:"memory"`
			Threads uint8  `yaml:"threads"`
			KeyLen  uint32 `yaml:"keyLen"`
		} `yaml:"kdf"`
		// AllowRemoteImages 是否允许文档中的远程图片（http/https）被加载，默认 false。
		AllowRemoteImages bool `yaml:"allowRemoteImages"`
	} `yaml:"vault"`
}

type accessConfig struct {
	Password    string         `yaml:"password"`
	AllowedMACs []string       `yaml:"allowedMACs"`
	AI          aiAccessConfig `yaml:"ai"`
}

type aiAccessConfig struct {
	Endpoint string `yaml:"endpoint"`
	ApiKey   string `yaml:"apiKey"`
	Model    string `yaml:"model"`
}

func main() {
	configPath := flag.String("config", "config.yaml", "build-time app config")
	accessPath := flag.String("access", defaultAccessPath, "build-time access config (password + MAC whitelist)")
	contentDir := flag.String("content", "content/plain", "plain markdown content directory")
	outputPath := flag.String("out", "internal/vault/generated.go", "generated Go vault output")
	flag.Parse()

	if err := run(*configPath, *accessPath, *contentDir, *outputPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(configPath, accessPath, contentDir, outputPath string) error {
	config, err := readConfig(configPath)
	if err != nil {
		return err
	}
	access, err := readAccess(accessPath)
	if err != nil {
		return err
	}
	if access.Password == "" {
		return fmt.Errorf("password is required in %s", accessPath)
	}
	documents, err := readDocuments(contentDir)
	if err != nil {
		return err
	}
	if len(documents) == 0 {
		return errors.New("no supported documents found")
	}

	encrypted, err := vault.EncryptVault(vault.PlainVault{Documents: documents}, access.Password, vault.KDFParams{
		Time:    config.Vault.KDF.Time,
		Memory:  config.Vault.KDF.Memory,
		Threads: config.Vault.KDF.Threads,
		KeyLen:  config.Vault.KDF.KeyLen,
	})
	if err != nil {
		return err
	}
	encrypted.AllowedMACs = access.AllowedMACs
	encrypted.AllowRemoteImages = config.Vault.AllowRemoteImages
	encrypted.AIConfig = vault.AIConfig{
		Endpoint: access.AI.Endpoint,
		ApiKey:   access.AI.ApiKey,
		Model:    access.AI.Model,
	}

	generated, err := renderGeneratedVault(encrypted)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, generated, 0o644); err != nil {
		return fmt.Errorf("write generated vault: %w", err)
	}
	return nil
}

func readConfig(path string) (vaultConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return vaultConfig{}, fmt.Errorf("read config: %w", err)
	}
	var config vaultConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return vaultConfig{}, fmt.Errorf("parse config: %w", err)
	}
	return config, nil
}

func readAccess(path string) (accessConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return accessConfig{}, fmt.Errorf("read access config: %w", err)
	}
	var access accessConfig
	if err := yaml.Unmarshal(data, &access); err != nil {
		return accessConfig{}, fmt.Errorf("parse access config: %w", err)
	}
	return access, nil
}

func readDocuments(root string) ([]vault.PlainDocument, error) {
	var documents []vault.PlainDocument
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !isSupportedDocument(entry.Name()) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read document %s: %w", path, err)
		}
		if len(data) == 0 {
			fmt.Fprintf(os.Stderr, "warning: skip empty file %s\n", path)
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve document path %s: %w", path, err)
		}
		normalizedPath := filepath.ToSlash(relative)
		documents = append(documents, plainDocument(entry.Name(), normalizedPath, data))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan content: %w", err)
	}
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Path < documents[j].Path
	})
	return documents, nil
}

func isSupportedDocument(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".md", ".pdf":
		return true
	default:
		return false
	}
}

func plainDocument(filename string, normalizedPath string, data []byte) vault.PlainDocument {
	extension := strings.ToLower(filepath.Ext(filename))
	switch extension {
	case ".pdf":
		return vault.PlainDocument{
			DocumentMetadata: vault.DocumentMetadata{
				ID:           documentID(normalizedPath),
				Title:        strings.TrimSuffix(filename, filepath.Ext(filename)),
				Path:         normalizedPath,
				DocumentType: "pdf",
				MimeType:     "application/pdf",
				Size:         int64(len(data)),
			},
			Content: data,
		}
	default:
		return vault.PlainDocument{
			DocumentMetadata: vault.DocumentMetadata{
				ID:           documentID(normalizedPath),
				Title:        strings.TrimSuffix(filename, filepath.Ext(filename)),
				Path:         normalizedPath,
				DocumentType: "markdown",
				MimeType:     "text/markdown; charset=utf-8",
				Size:         int64(len(data)),
			},
			Content: data,
		}
	}
}

func documentID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}

func renderGeneratedVault(encrypted vault.EncryptedVault) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("package vault\n\n")
	buffer.WriteString("// Code generated by `go run ./cmd/packdocs`; DO NOT EDIT.\n")
	buffer.WriteString("var EmbeddedVault = EncryptedVault{\n")
	buffer.WriteString(fmt.Sprintf("\tVersion: %d,\n", encrypted.Version))
	buffer.WriteString("\tKDF: KDFParams{\n")
	buffer.WriteString(fmt.Sprintf("\t\tTime: %d,\n", encrypted.KDF.Time))
	buffer.WriteString(fmt.Sprintf("\t\tMemory: %d,\n", encrypted.KDF.Memory))
	buffer.WriteString(fmt.Sprintf("\t\tThreads: %d,\n", encrypted.KDF.Threads))
	buffer.WriteString(fmt.Sprintf("\t\tKeyLen: %d,\n", encrypted.KDF.KeyLen))
	buffer.WriteString("\t},\n")
	buffer.WriteString(fmt.Sprintf("\tSalt: %s,\n", byteSliceLiteral(encrypted.Salt)))
	buffer.WriteString("\tManifest: EncryptedPayload{\n")
	buffer.WriteString(fmt.Sprintf("\t\tNonce:      %s,\n", byteSliceLiteral(encrypted.Manifest.Nonce)))
	buffer.WriteString(fmt.Sprintf("\t\tCiphertext: %s,\n", chunkedByteSliceLiteral(encrypted.Manifest.Ciphertext)))
	buffer.WriteString("\t},\n")
	buffer.WriteString("\tDocuments: []EncryptedDocument{\n")
	for _, document := range encrypted.Documents {
		buffer.WriteString("\t\t{\n")
		buffer.WriteString(fmt.Sprintf("\t\t\tID: %q,\n", document.ID))
		if len(document.Chunks) > 0 {
			buffer.WriteString("\t\t\tChunks: []EncryptedPayload{\n")
			for _, chunk := range document.Chunks {
				buffer.WriteString("\t\t\t\t{\n")
				buffer.WriteString(fmt.Sprintf("\t\t\t\t\tNonce:      %s,\n", byteSliceLiteral(chunk.Nonce)))
				buffer.WriteString(fmt.Sprintf("\t\t\t\t\tCiphertext: %s,\n", chunkedByteSliceLiteral(chunk.Ciphertext)))
				buffer.WriteString("\t\t\t\t},\n")
			}
			buffer.WriteString("\t\t\t},\n")
		} else {
			buffer.WriteString("\t\t\tEncryptedPayload: EncryptedPayload{\n")
			buffer.WriteString(fmt.Sprintf("\t\t\t\tNonce:      %s,\n", byteSliceLiteral(document.Nonce)))
			buffer.WriteString(fmt.Sprintf("\t\t\t\tCiphertext: %s,\n", chunkedByteSliceLiteral(document.Ciphertext)))
			buffer.WriteString("\t\t\t},\n")
		}
		buffer.WriteString("\t\t},\n")
	}
	buffer.WriteString("\t},\n")
	buffer.WriteString("\tAllowedMACs: []string{")
	for i, mac := range encrypted.AllowedMACs {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(fmt.Sprintf("%q", mac))
	}
	buffer.WriteString("},\n")
	buffer.WriteString(fmt.Sprintf("\tAllowRemoteImages: %t,\n", encrypted.AllowRemoteImages))
	buffer.WriteString("\tAIConfig: AIConfig{\n")
	buffer.WriteString(fmt.Sprintf("\t\tEndpoint: %q,\n", encrypted.AIConfig.Endpoint))
	buffer.WriteString(fmt.Sprintf("\t\tApiKey:   %q,\n", encrypted.AIConfig.ApiKey))
	buffer.WriteString(fmt.Sprintf("\t\tModel:    %q,\n", encrypted.AIConfig.Model))
	buffer.WriteString("\t},\n")
	buffer.WriteString("}\n")

	formatted, err := format.Source(buffer.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated vault: %w", err)
	}
	return formatted, nil
}

func byteSliceLiteral(data []byte) string {
	if len(data) > 96 {
		return chunkedByteSliceLiteral(data)
	}
	var buffer bytes.Buffer
	buffer.WriteString("[]byte{")
	for i, value := range data {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(fmt.Sprintf("0x%02x", value))
	}
	buffer.WriteString("}")
	return buffer.String()
}

func chunkedByteSliceLiteral(data []byte) string {
	const bytesPerLine = 16
	var buffer bytes.Buffer
	buffer.WriteString("[]byte{\n")
	for i, value := range data {
		if i%bytesPerLine == 0 {
			buffer.WriteString("\t\t\t")
		}
		buffer.WriteString(fmt.Sprintf("0x%02x,", value))
		if i%bytesPerLine == bytesPerLine-1 || i == len(data)-1 {
			buffer.WriteString("\n")
		} else {
			buffer.WriteString(" ")
		}
	}
	buffer.WriteString("\t\t}")
	return buffer.String()
}
