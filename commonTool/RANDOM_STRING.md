# 随机字符串使用说明

`commonTool.GenerateRandomString(length)` 使用 `crypto/rand.Reader`，生成由 `A-Z`、`a-z`、`0-9` 组成的字符串。长度单位为字符，也等于返回字符串的字节数。

长度必须为 **1～4096**，不设默认长度。非法长度在读取随机源或分配结果空间前返回错误；读取随机源失败时返回空字符串和可通过 `errors.Is/As` 检查的错误链，不返回部分结果，不回退到时间戳或伪随机数。

错误返回针对随机源以 `error` 返回的失败。部分 Go 版本的系统随机源会在底层致命失败时直接终止进程，此类运行时行为无法由本工具转成普通错误。

```go
package main

import (
    "log"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

func main() {
    value, err := commonTool.GenerateRandomString(32)
    if err != nil {
        log.Printf("generate random string failed: %v", err)
        return
    }
    // 不记录实际随机值；业务按用途存储或返回。
    log.Printf("generated %d characters", len(value))
}
```

接口已从 `GenerateRandomString(length int) string` 改为 `GenerateRandomString(length int) (string, error)`，调用方必须处理错误；不提供旧签名的兼容包装。

实现采用拒绝采样：丢弃随机字节中的 248～255，仅将 0～247 映射到 62 个字符，每个字符对应四个字节值。每次调用独立使用局部缓冲区，不重新播种或修改进程全局随机状态，可并发调用。

字符空间为 `62^length`。加密安全随机数不保证唯一，短字符串也不自动具备足够的猜测难度；业务需要唯一值时采用唯一约束及冲突处理，需要令牌时按用途选择足够长度并管理有效期。此工具不提供存储、校验、限流或一次性消费能力。

统一请求 ID 继续采用 [Trace ID 专用规则](../headInfo/TRACE_ID.md)，不使用本工具替换。TOTP 密钥、URI 和 Redis 分片属于独立工具，本次不作修改。
