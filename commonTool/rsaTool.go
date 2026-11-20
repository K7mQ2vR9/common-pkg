package commonTool

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
)

// minRSAKeyBits 为本项目允许的 RSA 模数最小位数，单位为 bit，签名和验签均要求至少 2048 位。
const minRSAKeyBits = 2048

var (
	// ErrInvalidRSAKey 表示密钥 PEM、类型、数学参数或位数不符合要求。
	ErrInvalidRSAKey = errors.New("invalid RSA key")
	// ErrInvalidRSASignature 表示签名不是规范的标准 Base64，或解码长度与 RSA 模数不符。
	ErrInvalidRSASignature = errors.New("invalid RSA signature encoding or length")
	// ErrRSAUninitialized 表示 nil 或零值实例未通过对应构造函数初始化。
	ErrRSAUninitialized = errors.New("RSA signer or verifier is not initialized")
)

// RSASigner 持有已校验及预计算的私钥，可并发复用；密钥仅在构造时解析，不向外暴露可变指针。
// 固定使用 SHA256 + RSA PKCS#1 v1.5，签名输出标准带填充 Base64。
type RSASigner struct {
	// key 在初始化完成后只读，实例生命周期内不进行密钥替换。
	key *rsa.PrivateKey
}

// RSAVerifier 持有已校验的公钥，可并发复用；算法和签名编码与 RSASigner 一致。
type RSAVerifier struct {
	// key 为构造时解析出的独立公钥，不暴露或修改其数学参数。
	key *rsa.PublicKey
}

// NewRSASigner 接收完整且唯一的未加密 PEM，支持 RSA PRIVATE KEY（PKCS#1）和 PRIVATE KEY（PKCS#8）。
// 拒绝非 RSA、无效私钥、少于 2048 位、PEM 头属性及非空外围内容；外层空白允许，不自动拼接头尾。
func NewRSASigner(privateKeyPEM string) (*RSASigner, error) {
	block, err := decodeRSAKeyPEM(privateKeyPEM)
	if err != nil {
		return nil, rsaKeyError("parse private key PEM", err)
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			key, _ = parsed.(*rsa.PrivateKey)
		}
	default:
		return nil, rsaKeyError("parse private key", nil)
	}
	if err != nil || key == nil {
		return nil, rsaKeyError("parse private key", err)
	}
	if !validRSAPublicKey(&key.PublicKey) {
		return nil, rsaKeyError("validate private key", nil)
	}
	if err := key.Validate(); err != nil {
		return nil, rsaKeyError("validate private key", err)
	}
	// 在交给并发请求之前完成预计算，避免首次签名时出现惰性初始化共享状态。
	key.Precompute()
	return &RSASigner{key: key}, nil
}

// NewRSAVerifier 接收完整且唯一的 PEM，支持 PUBLIC KEY（PKIX）和 RSA PUBLIC KEY（PKCS#1）。
// 仅接受模数至少 2048 位的合法 RSA 公钥，不从证书或私钥中隐式提取，不接受加密 PEM 或头属性。
func NewRSAVerifier(publicKeyPEM string) (*RSAVerifier, error) {
	block, err := decodeRSAKeyPEM(publicKeyPEM)
	if err != nil {
		return nil, rsaKeyError("parse public key PEM", err)
	}
	var key *rsa.PublicKey
	switch block.Type {
	case "RSA PUBLIC KEY":
		key, err = x509.ParsePKCS1PublicKey(block.Bytes)
	case "PUBLIC KEY":
		var parsed any
		parsed, err = x509.ParsePKIXPublicKey(block.Bytes)
		if err == nil {
			key, _ = parsed.(*rsa.PublicKey)
		}
	default:
		return nil, rsaKeyError("parse public key", nil)
	}
	if err != nil || !validRSAPublicKey(key) {
		return nil, rsaKeyError("validate public key", err)
	}
	return &RSAVerifier{key: key}, nil
}

// Sign 对 data 的原始字节做一次 SHA256 后签名；空正文合法，不执行 JSON 编码、排序或文本归一化。
// 失败返回空串并保留错误链；调用方不可在签名过程中并发修改 data。
func (s *RSASigner) Sign(data []byte) (string, error) {
	if s == nil || s.key == nil {
		return "", ErrRSAUninitialized
	}
	digest := sha256.Sum256(data)
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", &rsaToolError{"sign data", err}
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// Verify 校验 data 原始字节及规范的标准带填充 Base64 签名，成功返回 nil。
// 编码或长度错误以 ErrInvalidRSASignature 识别，内容不匹配保留 rsa.ErrVerification；不接受 Base64URL、换行或隐式补齐。
// 不执行业务正文解析或防重放，调用方不可在校验过程中并发修改 data。
func (v *RSAVerifier) Verify(data []byte, signature string) error {
	if v == nil || v.key == nil {
		return ErrRSAUninitialized
	}
	// 先按模数长度检查编码长度，拒绝超大输入后再分配解码空间。
	if len(signature) != base64.StdEncoding.EncodedLen(v.key.Size()) {
		return &rsaToolError{"decode signature", ErrInvalidRSASignature}
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(signature)
	if err != nil {
		return &rsaToolError{"decode signature", errors.Join(ErrInvalidRSASignature, err)}
	}
	if len(decoded) != v.key.Size() || base64.StdEncoding.EncodeToString(decoded) != signature {
		return &rsaToolError{"decode signature", ErrInvalidRSASignature}
	}
	digest := sha256.Sum256(data)
	if err := rsa.VerifyPKCS1v15(v.key, crypto.SHA256, digest[:], decoded); err != nil {
		return &rsaToolError{"verify signature", err}
	}
	return nil
}

// decodeRSAKeyPEM 要求唯一且完整的 PEM，防止 pem.Decode 忽略无关前缀或跳过损坏块后继续解析。
func decodeRSAKeyPEM(value string) (*pem.Block, error) {
	input := bytes.TrimSpace([]byte(value))
	if bytes.Count(input, []byte("-----BEGIN ")) != 1 || bytes.Count(input, []byte("-----END ")) != 1 {
		return nil, ErrInvalidRSAKey
	}
	block, rest := pem.Decode(input)
	if block == nil || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 ||
		!bytes.HasPrefix(input, []byte("-----BEGIN "+block.Type+"-----")) {
		return nil, ErrInvalidRSAKey
	}
	// 部分标准库版本的 PKCS#8 解析会忽略外层 DER 尾部，统一要求只有一个完整 ASN.1 值。
	var valueDER asn1.RawValue
	remaining, err := asn1.Unmarshal(block.Bytes, &valueDER)
	if err != nil {
		return nil, err
	}
	if len(remaining) != 0 {
		return nil, ErrInvalidRSAKey
	}
	return block, nil
}

func validRSAPublicKey(key *rsa.PublicKey) bool {
	return key != nil && key.N != nil && key.N.Sign() > 0 && key.N.BitLen() >= minRSAKeyBits &&
		key.N.Bit(0) == 1 && key.E >= 3 && key.E <= 1<<31-1 && key.E%2 == 1
}

func rsaKeyError(operation string, cause error) error {
	return &rsaToolError{operation, errors.Join(ErrInvalidRSAKey, cause)}
}

// rsaToolError 的公开文本仅包含操作阶段，不输出密钥、签名或正文；底层原因保留供 errors.Is/As 使用。
type rsaToolError struct {
	operation string
	cause     error
}

func (e *rsaToolError) Error() string { return "RSA " + e.operation + " failed" }
func (e *rsaToolError) Unwrap() error { return e.cause }
