package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

// GenerateRSAKeyPair generates a 2048-bit RSA key pair and returns them in PEM format.
func GenerateRSAKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Marshal private key to PKCS#8 PEM
	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})

	// Marshal public key to PKIX PEM
	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal public key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return string(privPEM), string(pubPEM), nil
}

// DecryptFlowsAESKey decrypts the encrypted AES key received from Meta WhatsApp Flows
// using RSA-OAEP with SHA-256 and MGF1 padding.
func DecryptFlowsAESKey(encryptedAESKeyBase64, privateKeyPEM string) ([]byte, error) {
	encryptedKey, err := base64.StdEncoding.DecodeString(encryptedAESKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode encrypted aes key: %w", err)
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("failed to parse private key PEM block")
	}

	privKeyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Fallback to PKCS1
		privKey, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse private key as PKCS8 or PKCS1: %w", err)
		}
		privKeyInterface = privKey
	}

	rsaPrivKey, ok := privKeyInterface.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an RSA private key")
	}

	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, rsaPrivKey, encryptedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt AES key with RSA-OAEP: %w", err)
	}

	return aesKey, nil
}

// DecryptFlowsPayload decrypts the encrypted flow data using AES-GCM with the decrypted AES key and IV.
func DecryptFlowsPayload(encryptedFlowDataBase64, ivBase64 string, aesKey []byte) ([]byte, error) {
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedFlowDataBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode encrypted flow data: %w", err)
	}

	iv, err := base64.StdEncoding.DecodeString(ivBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode initial vector: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	plaintext, err := aesGCM.Open(nil, iv, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt AES-GCM payload: %w", err)
	}

	return plaintext, nil
}

// EncryptFlowsResponse encrypts the response data using AES-GCM with the AES key and inverted IV.
func EncryptFlowsResponse(responseData []byte, ivBase64 string, aesKey []byte) (string, error) {
	iv, err := base64.StdEncoding.DecodeString(ivBase64)
	if err != nil {
		return "", fmt.Errorf("failed to base64 decode initial vector: %w", err)
	}

	// Invert all bits of the IV as required by Meta WhatsApp Flows specification
	flippedIV := make([]byte, len(iv))
	for i := range iv {
		flippedIV[i] = ^iv[i]
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	// Seal automatically appends the 16-byte authentication tag
	ciphertext := aesGCM.Seal(nil, flippedIV, responseData, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
