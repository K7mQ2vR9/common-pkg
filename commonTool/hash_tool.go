package commonTool

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
)

// MD5Hex 返回 data 原始字节的 32 位小写十六进制 MD5 摘要，nil 与空切片均为合法空输入。
// 仅用于协议指定的非安全校验，不用于密码存储或认证签名；Go 的 FIPS 只允许模式禁用 MD5 时沿用标准库 panic 行为。
// 不修改、去空白或归一化输入，可并发调用；调用方不得在计算过程中并发修改同一 data。
func MD5Hex(data []byte) string {
	digest := md5.Sum(data)
	return hex.EncodeToString(digest[:])
}

// SHA256Hex 返回 data 原始字节的 64 位小写十六进制 SHA256 摘要，nil 与空切片均为合法空输入。
// 用于内容摘要，不提供身份认证或密码存储功能；不修改或归一化输入，可并发调用。
// 调用方不得在计算过程中并发修改同一 data。
func SHA256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
