package commonTool

import (
	"crypto/rand"
	"fmt"
	"io"
)

const (
	randomStringCharset   = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	maxRandomStringLength = 4096
)

// GenerateRandomString 使用加密安全随机源生成指定长度的等概率字母数字字符串，支持并发调用。
// length 为字符数，必须在 1～4096 之间，无默认长度；非法长度或随机源读取失败返回空串及错误。
// 仅传播随机源以 error 返回的失败，Go 运行时自身的致命失败不转换为普通错误。
// 字符集固定为 A-Z、a-z、0-9；随机值不保证唯一，也不能替代统一 Trace ID 的专用生成规则。
func GenerateRandomString(length int) (string, error) {
	return generateRandomString(length, rand.Reader)
}

// generateRandomString 按批读取随机字节，不修改全局随机源；读取失败时不返回已生成的前缀。
func generateRandomString(length int, source io.Reader) (string, error) {
	if length < 1 || length > maxRandomStringLength {
		return "", fmt.Errorf("random string length must be between 1 and %d", maxRandomStringLength)
	}
	result := make([]byte, length)
	var buffer [128]byte
	// 仅接收 0～247，每个字符恰好对应四个字节值，避免直接对 256 个值取模导致偏差。
	const acceptedByteLimit = 256 - 256%len(randomStringCharset)
	for written := 0; written < length; {
		batchSize := min(length-written, len(buffer))
		if _, err := io.ReadFull(source, buffer[:batchSize]); err != nil {
			return "", fmt.Errorf("generate random string: %w", err)
		}
		for _, value := range buffer[:batchSize] {
			if int(value) >= acceptedByteLimit {
				continue
			}
			result[written] = randomStringCharset[int(value)%len(randomStringCharset)]
			written++
		}
	}
	return string(result), nil
}
