package main_test

import (
	"bytes"
	"testing"
)

func TestMediaEncryption(t *testing.T) {
	// Test data
	plaintext := []byte("This is a test message for Snapchat media encryption!")

	// Create encryptor
	encryptor, err := NewMediaEncryption()
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt
	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Verify ciphertext is different from plaintext
	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("Ciphertext should be different from plaintext")
	}

	// Decrypt
	decrypted, err := encryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	// Verify decrypted matches original
	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("Decrypted text doesn't match original.\nExpected: %s\nGot: %s", plaintext, decrypted)
	}

	t.Log("✅ Encryption/decryption test passed!")
}

func TestEncryptFile(t *testing.T) {
	// Test with image-like data
	testData := make([]byte, 1024*10) // 10KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	encryptor, err := NewMediaEncryption()
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt file
	encrypted, metadata, err := encryptor.EncryptFile(testData)
	if err != nil {
		t.Fatalf("Failed to encrypt file: %v", err)
	}

	// Verify metadata
	if metadata["encryption_key"] == "" {
		t.Fatal("Missing encryption_key in metadata")
	}
	if metadata["encryption_iv"] == "" {
		t.Fatal("Missing encryption_iv in metadata")
	}
	if metadata["hash"] == "" {
		t.Fatal("Missing hash in metadata")
	}
	if metadata["algorithm"] != "AES-256-CBC" {
		t.Fatal("Wrong algorithm in metadata")
	}

	// Decrypt using metadata
	decrypted, err := DecryptFile(encrypted, metadata["encryption_key"], metadata["encryption_iv"])
	if err != nil {
		t.Fatalf("Failed to decrypt file: %v", err)
	}

	// Verify
	if !bytes.Equal(testData, decrypted) {
		t.Fatal("Decrypted file doesn't match original")
	}

	t.Logf("✅ File encryption test passed! Encrypted %d bytes", len(encrypted))
}

func TestPKCS7Padding(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		blockSize int
	}{
		{"Empty", []byte{}, 16},
		{"Single byte", []byte{0x42}, 16},
		{"Full block", []byte("0123456789ABCDEF"), 16},
		{"Partial block", []byte("Hello"), 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pad
			padded := pkcs7Pad(tt.data, tt.blockSize)

			// Verify padding length
			if len(padded)%tt.blockSize != 0 {
				t.Fatalf("Padded length %d is not multiple of block size %d", len(padded), tt.blockSize)
			}

			// Unpad
			unpadded, err := pkcs7Unpad(padded, tt.blockSize)
			if err != nil {
				t.Fatalf("Failed to unpad: %v", err)
			}

			// Verify original data
			if !bytes.Equal(tt.data, unpadded) {
				t.Fatalf("Unpadded data doesn't match original.\nExpected: %v\nGot: %v", tt.data, unpadded)
			}
		})
	}

	t.Log("✅ PKCS7 padding test passed!")
}
