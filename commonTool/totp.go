package commonTool

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// totpSecretBytes 为新密钥的固定字节数，20 字节对应 160 位及 32 个 Base32 字符。
	totpSecretBytes = 20
	// minTOTPSecretBytes 为导入密钥的最小解码字节数，按 RFC 4226 要求至少 128 位。
	minTOTPSecretBytes = 16
	// totpDigits 为验证码固定字符数，包含前导零。
	totpDigits = 6
	// totpPeriodSeconds 为固定窗口长度，单位为秒，T0 为 Unix 起始时间。
	totpPeriodSeconds = 30
)

var (
	// ErrInvalidTOTPSecret 表示密钥不是规范的无填充大写 Base32，或解码后不足 16 字节。
	ErrInvalidTOTPSecret = errors.New("invalid TOTP secret")
	// ErrInvalidTOTPCode 表示验证码不是恰好六位 ASCII 数字，不自动补零或去除空白。
	ErrInvalidTOTPCode = errors.New("TOTP code must contain exactly six ASCII digits")
	// ErrTOTPMismatch 表示验证码未匹配当前或上一个 30 秒窗口，不含输入值。
	ErrTOTPMismatch = errors.New("TOTP code mismatch")
)

// GenerateTOTPSecret 读取 20 字节加密安全随机数据，返回 32 字符的无填充大写 Base32 密钥。
// 读取失败返回空串并保留错误链；不修改全局随机源，Go 运行时自身的致命失败不转换为普通错误。
func GenerateTOTPSecret() (string, error) {
	return generateTOTPSecret(rand.Reader)
}

// generateTOTPSecret 按实际随机字节进行 Base32 编码，不通过取模缩减随机空间。
func generateTOTPSecret(source io.Reader) (string, error) {
	var key [totpSecretBytes]byte
	if _, err := io.ReadFull(source, key[:]); err != nil {
		return "", fmt.Errorf("generate TOTP secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key[:]), nil
}

// BuildTOTPURI 返回供本地二维码组件编码的 otpauth URI，不生成图片或访问外部服务。
// 固定 SHA1、六位、30 秒；secret 须为规范大写无填充 Base32，解码后至少 16 字节。
// account、issuer 必须为非空 UTF-8 文本，禁止冒号及控制字符；保留原文并分别进行 URL 编码。
func BuildTOTPURI(secret, account, issuer string) (string, error) {
	if _, err := decodeTOTPSecret(secret); err != nil {
		return "", err
	}
	for _, field := range []struct{ name, value string }{{"account", account}, {"issuer", issuer}} {
		if !validTOTPLabel(field.value) {
			return "", fmt.Errorf("TOTP %s must be non-empty UTF-8 text without colons or control characters", field.name)
		}
	}
	query := url.Values{
		"secret":    {secret},
		"issuer":    {issuer},
		"algorithm": {"SHA1"},
		"digits":    {"6"},
		"period":    {"30"},
	}
	// Path 保留解码后的标签，RawPath 分段转义斜杠等字符；唯一冒号为发行方和账号的分隔符。
	uri := url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + issuer + ":" + account,
		RawPath:  "/" + url.PathEscape(issuer) + ":" + url.PathEscape(account),
		RawQuery: strings.ReplaceAll(query.Encode(), "+", "%20"),
	}
	return uri.String(), nil
}

// VerifyTOTP 使用服务端当前 Unix 时间校验六位字符串，保留前导零，算法固定 HMAC-SHA1。
// 接受当前及上一个 30 秒窗口，不计算未来窗口；所有候选均采用常量时间比较，不提前命中返回。
// 只做无状态校验，同码重复使用和尝试次数限制由业务层结合用户状态原子控制。
func VerifyTOTP(code, secret string) error {
	return verifyTOTPAt(code, secret, time.Now())
}

func verifyTOTPAt(code, secret string, now time.Time) error {
	if len(code) != totpDigits {
		return ErrInvalidTOTPCode
	}
	for i := range code {
		if code[i] < '0' || code[i] > '9' {
			return ErrInvalidTOTPCode
		}
	}
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return err
	}
	seconds := now.Unix()
	if seconds < 0 {
		return errors.New("TOTP time must not precede Unix epoch")
	}
	counter := uint64(seconds / totpPeriodSeconds)
	matched := subtle.ConstantTimeCompare([]byte(code), []byte(totpCode(key, counter)))
	// Unix 起始窗口没有上一个合法窗口，避免 uint64 减一回绕。
	if counter > 0 {
		matched |= subtle.ConstantTimeCompare([]byte(code), []byte(totpCode(key, counter-1)))
	}
	if matched != 1 {
		return ErrTOTPMismatch
	}
	return nil
}

func decodeTOTPSecret(secret string) ([]byte, error) {
	if secret == "" {
		return nil, ErrInvalidTOTPSecret
	}
	// 标准库 Base32 解码会忽略 CR/LF，此处先严格限制字符，禁止隐式清理或填充。
	for i := range secret {
		if (secret[i] < 'A' || secret[i] > 'Z') && (secret[i] < '2' || secret[i] > '7') {
			return nil, ErrInvalidTOTPSecret
		}
	}
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	key, err := encoding.DecodeString(secret)
	if err != nil || len(key) < minTOTPSecretBytes || encoding.EncodeToString(key) != secret {
		return nil, ErrInvalidTOTPSecret
	}
	return key, nil
}

func validTOTPLabel(value string) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || strings.ContainsRune(value, ':') {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

// totpCode 实现 RFC 4226 动态截断，使用 8 字节大端计数器，始终返回六位文本。
func totpCode(key []byte, counter uint64) string {
	var value [8]byte
	binary.BigEndian.PutUint64(value[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(value[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	number := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", number%1000000)
}
