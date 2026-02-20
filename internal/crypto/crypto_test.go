package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := "my-secret-key-for-testing-12345"
	plaintext := "EAABsbCS1iHgBO..."

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted == plaintext {
		t.Fatal("Encrypted value should differ from plaintext")
	}

	if !IsEncrypted(encrypted) {
		t.Fatal("Encrypted value should have enc: prefix")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("Decrypted value %q != plaintext %q", decrypted, plaintext)
	}
}

func TestDecrypt_LegacyUnencrypted(t *testing.T) {
	key := "my-secret-key"
	legacy := "plain-text-token-without-prefix"

	decrypted, err := Decrypt(legacy, key)
	if err != nil {
		t.Fatalf("Decrypt legacy failed: %v", err)
	}
	if decrypted != legacy {
		t.Fatalf("Legacy value should be returned as-is, got %q", decrypted)
	}
}

func TestEncryptDecrypt_EmptyKey(t *testing.T) {
	plaintext := "some-secret"

	encrypted, err := Encrypt(plaintext, "")
	if err != nil {
		t.Fatalf("Encrypt with empty key failed: %v", err)
	}
	if encrypted != plaintext {
		t.Fatal("Empty key should return plaintext unchanged")
	}

	decrypted, err := Decrypt(plaintext, "")
	if err != nil {
		t.Fatalf("Decrypt with empty key failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatal("Empty key should return ciphertext unchanged")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := "correct-key"
	key2 := "wrong-key"

	encrypted, _ := Encrypt("secret", key1)
	_, err := Decrypt(encrypted, key2)
	if err == nil {
		t.Fatal("Decrypt with wrong key should fail")
	}
}
