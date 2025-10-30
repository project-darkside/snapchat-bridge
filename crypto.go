package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// MediaEncryption handles encryption/decryption of Snap media
type MediaEncryption struct {
	Key []byte
	IV  []byte
}

// NewMediaEncryption creates a new media encryption instance with random key and IV
func NewMediaEncryption() (*MediaEncryption, error) {
	key := make([]byte, 32) // AES-256
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	return &MediaEncryption{
		Key: key,
		IV:  iv,
	}, nil
}

// Encrypt encrypts data using AES-256-CBC
func (m *MediaEncryption) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Apply PKCS7 padding
	plaintext = pkcs7Pad(plaintext, aes.BlockSize)

	ciphertext := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, m.IV)
	mode.CryptBlocks(ciphertext, plaintext)

	return ciphertext, nil
}

// Decrypt decrypts data using AES-256-CBC
func (m *MediaEncryption) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, m.IV)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	plaintext, err = pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to unpad: %w", err)
	}

	return plaintext, nil
}

// EncryptFile encrypts a file and returns the encrypted data and metadata
func (m *MediaEncryption) EncryptFile(data []byte) (encrypted []byte, metadata map[string]string, err error) {
	encrypted, err = m.Encrypt(data)
	if err != nil {
		return nil, nil, err
	}

	// Calculate SHA256 hash of encrypted data
	hash := sha256.Sum256(encrypted)

	metadata = map[string]string{
		"encryption_key": base64.StdEncoding.EncodeToString(m.Key),
		"encryption_iv":  base64.StdEncoding.EncodeToString(m.IV),
		"hash":           base64.StdEncoding.EncodeToString(hash[:]),
		"algorithm":      "AES-256-CBC",
	}

	return encrypted, metadata, nil
}

// DecryptFile decrypts a file using provided metadata
func DecryptFile(encrypted []byte, key, iv string) ([]byte, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}

	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("failed to decode IV: %w", err)
	}

	m := &MediaEncryption{
		Key: keyBytes,
		IV:  ivBytes,
	}

	return m.Decrypt(encrypted)
}

// pkcs7Pad applies PKCS7 padding to data
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// pkcs7Unpad removes PKCS7 padding from data
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("invalid padding size")
	}

	padding := int(data[length-1])
	if padding > blockSize || padding == 0 {
		return nil, fmt.Errorf("invalid padding size")
	}

	// Verify padding
	for i := length - padding; i < length; i++ {
		if data[i] != byte(padding) {
			return nil, fmt.Errorf("invalid padding")
		}
	}

	return data[:length-padding], nil
}

// EncryptStream wraps an io.Reader to encrypt data on-the-fly
type EncryptStream struct {
	reader io.Reader
	cipher cipher.BlockMode
	buffer []byte
	eof    bool
}

// NewEncryptStream creates a new encrypting stream
func NewEncryptStream(reader io.Reader, key, iv []byte) (*EncryptStream, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return &EncryptStream{
		reader: reader,
		cipher: cipher.NewCBCEncrypter(block, iv),
		buffer: make([]byte, 0),
		eof:    false,
	}, nil
}

// Read implements io.Reader
func (e *EncryptStream) Read(p []byte) (n int, err error) {
	if len(e.buffer) == 0 && !e.eof {
		// Read and encrypt a block
		plaintext := make([]byte, aes.BlockSize)
		n, err := e.reader.Read(plaintext)
		if err == io.EOF {
			e.eof = true
			// Apply padding on last block
			plaintext = pkcs7Pad(plaintext[:n], aes.BlockSize)
			n = len(plaintext)
		} else if err != nil {
			return 0, err
		}

		if n > 0 {
			ciphertext := make([]byte, n)
			e.cipher.CryptBlocks(ciphertext, plaintext[:n])
			e.buffer = append(e.buffer, ciphertext...)
		}
	}

	if len(e.buffer) == 0 {
		return 0, io.EOF
	}

	n = copy(p, e.buffer)
	e.buffer = e.buffer[n:]
	return n, nil
}
