package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

const prefix = "enc:"

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded
// ciphertext prefixed with "enc:" for identification.
// If key is empty, returns the plaintext unchanged (no-op for dev mode).
func Encrypt(plaintext, key string) (string, error) {
	if key == "" || plaintext == "" {
		return plaintext, nil
	}

	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return prefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a value previously encrypted with Encrypt.
// If the value doesn't have the "enc:" prefix, it's returned as-is
// (supports reading legacy unencrypted data).
func Decrypt(ciphertext, key string) (string, error) {
	if key == "" || ciphertext == "" {
		return ciphertext, nil
	}

	// Not encrypted — return as-is (legacy data)
	if len(ciphertext) < len(prefix) || ciphertext[:len(prefix)] != prefix {
		return ciphertext, nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext[len(prefix):])
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a value has the encryption prefix.
func IsEncrypted(value string) bool {
	return len(value) >= len(prefix) && value[:len(prefix)] == prefix
}

// deriveKey pads or truncates the key to exactly 32 bytes for AES-256.
func deriveKey(key string) []byte {
	k := make([]byte, 32)
	copy(k, []byte(key))
	return k
}
