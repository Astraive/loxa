package atrest

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
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
	derived := make([]byte, 32)
	reader := hkdf.New(sha256.New, []byte(key), []byte("loxa-at-rest-v1"), []byte("aes-256-gcm"))
	if _, err := io.ReadFull(reader, derived); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
