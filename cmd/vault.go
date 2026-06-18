package cmd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

const (
	PBKDF2Iterations = 600000
	SaltLength       = 32
	KeyLength        = 32
	NonceLength      = 12
	VaultVersion     = 1
)

// VaultEntry represents a single TOTP credential.
type VaultEntry struct {
	Name      string `json:"name"`
	Secret    string `json:"secret"` // Base32 encoded key
	Issuer    string `json:"issuer,omitempty"`
	Algorithm string `json:"algorithm,omitempty"` // e.g. SHA1 (default), SHA256, SHA512
	Digits    int    `json:"digits,omitempty"`    // e.g. 6 (default), 8
	Period    uint   `json:"period,omitempty"`    // e.g. 30 (default)
}

// EncryptedVault represents the encrypted structure saved on disk.
type EncryptedVault struct {
	Version    int    `json:"version"`
	Salt       string `json:"salt"` // Base64
	Iterations int    `json:"iterations"`
	Nonce      string `json:"nonce"`      // Base64
	Ciphertext string `json:"ciphertext"` // Base64
}

// DefaultVaultPath returns the path to ~/.config/cfa/vault.enc
func DefaultVaultPath() (string, error) {
	if path := os.Getenv("CFA_VAULT_PATH"); path != "" {
		return path, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "cfa", "vault.enc"), nil
}

// GetMasterPassword retrieves the password either from env or by prompting.
func GetMasterPassword(prompt string, confirm bool) (string, error) {
	if pwd := os.Getenv("CFA_PASSWORD"); pwd != "" {
		return pwd, nil
	}

	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println() // print newline after password input

	pwd := string(bytePassword)
	if pwd == "" {
		return "", errors.New("password cannot be empty")
	}

	if confirm {
		fmt.Print("Confirm password: ")
		byteConfirm, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", fmt.Errorf("failed to read confirmation password: %w", err)
		}
		fmt.Println()
		if pwd != string(byteConfirm) {
			return "", errors.New("passwords do not match")
		}
	}

	return pwd, nil
}

// LoadVault loads and decrypts the vault entries using the provided password.
func LoadVault(path string, password string) ([]VaultEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var encVault EncryptedVault
	if err := json.Unmarshal(data, &encVault); err != nil {
		return nil, fmt.Errorf("failed to parse vault JSON structure: %w", err)
	}

	if encVault.Version != VaultVersion {
		return nil, fmt.Errorf("unsupported vault version: %d", encVault.Version)
	}

	salt, err := base64.StdEncoding.DecodeString(encVault.Salt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encVault.Nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encVault.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, encVault.Iterations, KeyLength, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher initialization error: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM initialization error: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("failed to decrypt vault: incorrect master password or corrupted vault file")
	}

	var entries []VaultEntry
	if err := json.Unmarshal(plaintext, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted JSON: %w", err)
	}

	return entries, nil
}

// SaveVault encrypts and writes the vault entries using the provided password.
func SaveVault(path string, entries []VaultEntry, password string) error {
	plaintext, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("failed to serialize vault entries: %w", err)
	}

	// Generate a secure random salt
	salt := make([]byte, SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate random salt: %w", err)
	}

	// Generate a secure random nonce
	nonce := make([]byte, NonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate random nonce: %w", err)
	}

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, KeyLength, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("cipher initialization error: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("GCM initialization error: %w", err)
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	encVault := EncryptedVault{
		Version:    VaultVersion,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Iterations: PBKDF2Iterations,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}

	output, err := json.MarshalIndent(encVault, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal encrypted vault: %w", err)
	}

	// Ensure destination directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	// Write the encrypted vault file with restricted permissions (read/write by owner only)
	if err := os.WriteFile(path, output, 0600); err != nil {
		return fmt.Errorf("failed to write vault file %s: %w", path, err)
	}

	return nil
}

func getVaultPassword(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("vault not initialized. Please run: cfa init")
	}
	return GetMasterPassword("Enter master password: ", false)
}
