package aesGCM

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

// Encrypt 每次生成随机 nonce，加密结果为 URL-safe Base64 编码的 nonce、密文和认证标签。
func Encrypt(key, plaintext []byte) (string, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}
	return encrypt(aead, plaintext)
}

// Decrypt 校验认证标签后返回明文，支持带 padding 和无 padding 的 URL-safe Base64。
func Decrypt(key []byte, encryptedText string) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return decrypt(aead, encryptedText)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encrypt(aead cipher.AEAD, plaintext []byte) (string, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

func decrypt(aead cipher.AEAD, encryptedText string) ([]byte, error) {
	encoding := base64.RawURLEncoding
	if strings.Contains(encryptedText, "=") {
		encoding = base64.URLEncoding
	}
	ciphertext, err := encoding.DecodeString(encryptedText)
	if err != nil {
		return nil, err
	}
	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize+aead.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	return aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}
