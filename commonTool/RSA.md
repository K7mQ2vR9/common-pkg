# RSA 签名与验签使用说明

使用 `commonTool.RSASigner` 和 `commonTool.RSAVerifier`，密钥在初始化时解析校验，实例可并发复用。旧 `GenerateSignature`、`VerifySignature` 已删除，无兼容包装。

算法固定为 **SHA256 + RSA PKCS#1 v1.5**，签名编码为标准 Base64，按标准规则保留需要的 `=` 填充；模数字节数可被 3 整除时无需 `=`。正文按调用方传入的原始字节签名，不执行 JSON 编码、字段排序、换行处理或字符归一化，也不要求调用方预先计算摘要。

## 接口

```text
NewRSASigner(privateKeyPEM string) (*RSASigner, error)
(*RSASigner).Sign(data []byte) (string, error)

NewRSAVerifier(publicKeyPEM string) (*RSAVerifier, error)
(*RSAVerifier).Verify(data []byte, signature string) error
```

服务启动时创建并注入实例，避免每个请求重复解析密钥。实例不暴露可变密钥指针，私钥在返回实例前完成校验和预计算，业务无需调用 Close。密钥轮换时构造并校验新实例，再由应用替换引用；本工具不管理密钥发布或轮换流程。

## 密钥格式与校验

构造函数接收包含完整头尾的 PEM 文本内容，文件读取由应用负责，不传文件名或只传中间的 Base64。

| 用途 | PEM 类型 | 格式 |
|---|---|---|
| 私钥 | `RSA PRIVATE KEY` | PKCS#1 |
| 私钥 | `PRIVATE KEY` | 未加密 PKCS#8，内部必须为 RSA |
| 公钥 | `PUBLIC KEY` | PKIX SubjectPublicKeyInfo，内部必须为 RSA |
| 公钥 | `RSA PUBLIC KEY` | PKCS#1 |

项目要求模数至少 **2048 位**，支持更大密钥。私钥校验数学参数，公钥检查正奇数模数和有效奇数指数范围。以下输入初始化失败：

- 空值、损坏 PEM、非 RSA 密钥、证书、用途不匹配的 PEM 类型。
- 加密 PKCS#8、带传统加密头或其他 PEM 头属性的块；本工具不接收密码或解密密钥。
- 多个 PEM 块、非空白前缀/尾部内容、DER 尾部额外数据。PEM 外层空白和 LF/CRLF 换行允许。
- 不足 2048 位、无效指数/模数或损坏的私钥数学参数。

构造错误可通过 `errors.Is(err, commonTool.ErrInvalidRSAKey)` 判断。错误文本仅包含操作阶段，不输出 PEM、正文或签名值；底层原因保留在错误链中，日志不要自行展开并附带密钥输入。

## 完整范例

将实际配对的私钥和公钥文件路径分别放入 `RSA_PRIVATE_KEY_FILE`、`RSA_PUBLIC_KEY_FILE`。范例只进行本地签名和验签，不输出密钥或签名，不发起外部请求。

```go
package main

import (
    "log"
    "os"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

func main() {
    privatePEM, err := os.ReadFile(os.Getenv("RSA_PRIVATE_KEY_FILE"))
    if err != nil {
        log.Printf("load private key file: %v", err)
        return
    }
    publicPEM, err := os.ReadFile(os.Getenv("RSA_PUBLIC_KEY_FILE"))
    if err != nil {
        log.Printf("load public key file: %v", err)
        return
    }
    signer, err := commonTool.NewRSASigner(string(privatePEM))
    if err != nil {
        log.Printf("initialize RSA signer: %v", err)
        return
    }
    verifier, err := commonTool.NewRSAVerifier(string(publicPEM))
    if err != nil {
        log.Printf("initialize RSA verifier: %v", err)
        return
    }
    // 实际业务使用双方约定的相同原始字节，JSON 不先解码再重新编码。
    body := []byte(`{"id":9007199254740993,"name":"example"}`)
    signature, err := signer.Sign(body)
    if err != nil {
        log.Printf("sign request: %v", err)
        return
    }
    if err := verifier.Verify(body, signature); err != nil {
        log.Printf("verify request: %v", err)
        return
    }
    log.Print("RSA signature verified")
}
```

签名方和验签方各自加载自己需要的密钥；两个构造函数独立，不要求每个服务都持有私钥。上例使用配对密钥演示完整流程。空正文合法，nil 与空切片对应相同签名输入；调用期间不要从其他 goroutine 修改同一正文切片。

## 验签错误处理

| 检查方式 | 含义 |
|---|---|
| `errors.Is(err, commonTool.ErrInvalidRSAKey)` | 初始化密钥不符合要求 |
| `errors.Is(err, commonTool.ErrRSAUninitialized)` | nil 或零值签名/验签实例未初始化 |
| `errors.Is(err, commonTool.ErrInvalidRSASignature)` | Base64 编码不规范或解码长度与模数不符 |
| `errors.Is(err, rsa.ErrVerification)` | 规范签名未通过密码学验证，例如正文/签名篡改或同位数的错误公钥 |
| `var decodeErr base64.CorruptInputError; errors.As(err, &decodeErr)` | 进一步检查底层 Base64 解码错误 |

验签先限制编码长度，再严格解码并检查规范编码；拒绝多余空白、CR/LF、缺失填充、非零尾部填充位、Base64URL 专用字符和超大输入。不同位数公钥对应不同签名字节数，可能在长度检查阶段失败。所有失败都不能继续当作已认证请求。

```go
package examples

import (
    "crypto/rsa"
    "errors"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

// VerifyIncoming 分类处理不合法签名；verifier 由服务启动时初始化后注入。
func VerifyIncoming(verifier *commonTool.RSAVerifier, body []byte, signature string) (bool, error) {
    err := verifier.Verify(body, signature)
    if err == nil {
        return true, nil
    }
    if errors.Is(err, commonTool.ErrInvalidRSASignature) || errors.Is(err, rsa.ErrVerification) {
        return false, nil
    }
    return false, err
}
```

PKCS#1 v1.5 签名对同一密钥和正文产生相同结果；它不提供请求去重或一次性消费能力。业务协议按用途约定正文序列化、时间戳、nonce 和验证后的消费状态，工具仅对字节签名/验签，不自动添加或解释这些字段。API/RPC 各自按职责注入实例，不把原始密钥或签名正文写入请求日志。

本轮定向测试使用临时生成的 1024/2048/3072 位测试密钥，并与标准库及本机 `openssl` 命令（LibreSSL 3.3.6）双向验证；未对接外部签名协议。密钥管理、传输协议和部署模式由接入业务联调。
