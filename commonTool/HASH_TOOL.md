# 哈希摘要工具使用说明

摘要函数独立放在 `commonTool/hash_tool.go`：

```text
MD5Hex(data []byte) string
SHA256Hex(data []byte) string
```

| 接口 | 输出 |
|---|---|
| `MD5Hex` | 32 个小写十六进制字符，16 字节 MD5 摘要 |
| `SHA256Hex` | 64 个小写十六进制字符，32 字节 SHA256 摘要 |

直接使用标准库 `md5.Sum`、`sha256.Sum256` 及十六进制编码，不依赖 `crypto.Hash.New` 的间接注册。输出不含前缀、空格、分隔符或换行。

## 原始字节规则

- 输入为原始字节，UTF-8 文本可显式用 `[]byte(text)` 传入，二进制数据可直接传切片。
- 不自动去空白、规范换行、调整大小写、解析 JSON 或进行 Unicode 归一化。
- nil 与空切片均对应合法的空内容摘要，不返回空字符串。
- 不修改或持有调用方的输入切片；可并发调用，调用期间不能并发修改同一切片。
- 函数只计算传入数据，不执行文件或网络读取。

## 完整范例

```go
package main

import (
    "fmt"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

func main() {
    content := []byte("abc")
    fmt.Println(commonTool.MD5Hex(content))
    fmt.Println(commonTool.SHA256Hex(content))
}
```

输出：

```text
900150983cd24fb0d6963f7d28e17f72
ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
```

## 用途与运行约束

一般内容摘要使用 SHA256。MD5 仅用于协议明确要求的非安全校验，不用于密码存储或认证签名；其安全限制见 [Go MD5 文档](https://pkg.go.dev/crypto/md5)。摘要自身不提供来源认证，业务需要认证时继续采用约定的签名协议。

在支持并启用 FIPS 只允许模式的 Go 环境中，标准库会禁止 MD5 并 panic。`MD5Hex` 沿用该运行约束，不恢复 panic、不静默切换算法；部署模式必须允许所选摘要算法。本轮在普通运行模式验证，未对 FIPS 部署作认证验证。

## 迁移与接入

旧 `Md5(text string)` 和只包含它的 `tool.go` 已删除，不保留兼容包装。需要保留原 MD5 计算语义的调用使用 `MD5Hex([]byte(text))`，相同原始字节得到相同摘要；改用 SHA256 时须同时调整业务协议及摘要字段长度，不能直接替换已有 MD5 字段的期望值。

API/RPC 的业务契约明确算法、原始字节和编码格式，字段长度分别按 32/64 个字符处理。本轮只提供基础函数及接入约定，不修改实际业务 DTO、认证流程或存储结构。

定向验证覆盖 MD5/SHA256 标准向量、百万字符 SHA256、128 组 Python hashlib 独立参考、中文/Unicode 不同表示/二进制/换行、输入所有权及并发调用，不依赖外部服务。
