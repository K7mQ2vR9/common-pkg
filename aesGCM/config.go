package aesGCM

import (
	"crypto/cipher"
	"sync/atomic"
)

// Config 配置当前进程共用的 AES-GCM 开关和密钥。
type Config struct {
	// Enabled 默认为 false，启用后校验密钥并加密成功响应。
	Enabled bool `json:",optional"`
	// Key 为原始 UTF-8 密钥字符串，启用时须为 16、24 或 32 字节。
	Key string `json:",optional"`
}

// Cipher 保存初始化后只读的加密器，可供请求并发使用。
type Cipher struct {
	aead cipher.AEAD
}

// New 按实例配置创建加密器，未启用时返回 nil，启用且密钥无效时返回错误。
func New(config Config) (*Cipher, error) {
	if !config.Enabled {
		return nil, nil
	}
	aead, err := newAEAD([]byte(config.Key))
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 使用实例密钥和新的随机 nonce 加密明文。
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	return encrypt(c.aead, plaintext)
}

// Decrypt 使用实例密钥校验并解密 URL-safe Base64 密文。
func (c *Cipher) Decrypt(encryptedText string) ([]byte, error) {
	return decrypt(c.aead, encryptedText)
}

var configuredCipher atomic.Pointer[Cipher]

// Configure 在初始化时应用进程级配置，默认关闭，配置无效时返回错误并保留当前配置。
// 开关和密钥作为同一份只读配置发布，供请求解密与响应加密直接读取。
func Configure(config Config) error {
	c, err := New(config)
	if err != nil {
		return err
	}
	configuredCipher.Store(c)
	return nil
}

// ConfiguredCipher 读取当前进程的加密器，nil 表示配置开关已关闭。
func ConfiguredCipher() *Cipher {
	return configuredCipher.Load()
}
