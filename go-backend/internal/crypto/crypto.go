// Package crypto provides encryption utilities for sensitive data.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrInvalidKey        = errors.New("encryption key must be 32 bytes (AES-256)")
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	ErrKeyNotConfigured  = errors.New("encryption key not configured")
)

// Encryptor provides AES-256-GCM encryption for sensitive data.
type Encryptor struct {
	key []byte
}

// NewEncryptor creates a new encryptor with the given 32-byte key.
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	return &Encryptor{key: key}, nil
}

// NewEncryptorFromEnv creates an encryptor from ENCRYPTION_KEY environment variable.
// The key should be a 32-byte base64-encoded string.
func NewEncryptorFromEnv() (*Encryptor, error) {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		return nil, ErrKeyNotConfigured
	}

	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}

	return NewEncryptor(key)
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns base64-encoded ciphertext (nonce + ciphertext).
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext (nonce + ciphertext).
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// GenerateKey generates a random 32-byte key for AES-256.
// Returns the key as a base64-encoded string.
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
