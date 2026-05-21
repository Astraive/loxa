package atrest

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strings"
)

const Prefix = "enc:"

func EncryptString(plain []byte, key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return string(plain), nil
	}
	aead, err := buildAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, plain, nil)
	return Prefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func EncryptBytes(plain []byte, key string) ([]byte, error) {
	text, err := EncryptString(plain, key)
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

func buildAEAD(key string) (cipher.AEAD, error) {
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
