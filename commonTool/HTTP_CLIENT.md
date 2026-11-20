# HTTP 请求工具使用说明

使用 `github.com/K7mQ2vR9/base-pkg/commonTool` 中的 `HTTPClient`。旧 `GetRequest(url)` 和 `PostRequest(host, map)` 已删除，不保留兼容包装；调用方改为共享客户端并显式传入业务上下文。

## 配置与默认值

```yaml
Timeout: 10s
MaxResponseBytes: 4194304
Transport:
  DialTimeout: 5s
  TLSHandshakeTimeout: 5s
  ResponseHeaderTimeout: 5s
  MaxIdleConns: 100
  MaxIdleConnsPerHost: 100
  MaxConnsPerHost: 100
  IdleConnTimeout: 90s
```

应用可以将 `HTTPConfig` 嵌入自己的配置结构。JSON/YAML 解析由应用负责，客户端构造时只校验参数并创建独立连接池，不发起请求、不修改进程全局默认客户端。

| 配置 | 含义 |
|---|---|
| `Timeout` | 默认 10s，单次调用总预算，覆盖 JSON 编码耗时、建连、响应头和正文读取 |
| `MaxResponseBytes` | 默认 4MiB，单个响应正文大小上限；不允许负值或 MaxInt64 |
| `DialTimeout` | 默认 5s，TCP 建连时限 |
| `TLSHandshakeTimeout` | 默认 5s，TLS 握手时限 |
| `ResponseHeaderTimeout` | 默认 5s，等待响应头的时限，不包含读取正文 |
| `MaxIdleConns` | 默认 100，所有节点最大空闲连接总数 |
| `MaxIdleConnsPerHost` | 默认 100，每个节点最大空闲连接数，不得超过 MaxConnsPerHost |
| `MaxConnsPerHost` | 默认 100，每个节点连接总数上限 |
| `IdleConnTimeout` | 默认 90s，空闲连接保留时间 |

字段为 0 时采用默认值，负值非法，不使用 0 表示无限等待或无限正文。总预算与父上下文中更早的截止时间优先，阶段时限也可能更早结束请求。HTTPS 默认使用系统信任证书、校验主机名，最低 TLS 1.2；代理遵循 `HTTP_PROXY/HTTPS_PROXY/NO_PROXY` 环境配置。

JSON 编码是同步计算，无法强制终止自定义 MarshalJSON；编码结束时已取消或超时则不发起网络请求，也不会为编码另建后台 goroutine。

## 完整 GET 范例

提供实际 JSON 接口地址 `HTTP_API_URL`，按需提供 `HTTP_API_TOKEN`；下例只在运行时才发起 GET，本次仅编译范例。接口返回结构按业务调整。

```go
package main

import (
    "context"
    "encoding/json"
    "errors"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

func main() {
    client, err := commonTool.NewHTTPClient(commonTool.HTTPConfig{})
    if err != nil {
        log.Printf("initialize HTTP client: %v", err)
        return
    }
    defer client.Close()

    // HTTP/RPC 服务中应使用当前业务 ctx；本例演示独立调用的 5s 上游预算。
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    headers := http.Header{"Accept": {"application/json"}}
    if token := os.Getenv("HTTP_API_TOKEN"); token != "" {
        headers.Set("Authorization", "Bearer "+token)
    }
    response, err := client.Get(ctx, os.Getenv("HTTP_API_URL"), headers)
    if err != nil {
        var status *commonTool.HTTPStatusError
        if errors.As(err, &status) {
            log.Printf("remote HTTP status=%d body_complete=%t", status.StatusCode, response.BodyComplete)
        } else {
            log.Printf("HTTP call failed: %v", err)
        }
        return
    }
    if response.StatusCode == http.StatusNoContent {
        log.Print("request completed without a body")
        return
    }
    var item struct {
        ID int64 `json:"id"`
        Name string `json:"name"`
    }
    if err := json.Unmarshal(response.Body, &item); err != nil {
        log.Printf("decode remote JSON: %v", err)
        return
    }
    log.Printf("received item ID=%d name=%s", item.ID, item.Name)
}
```

在服务启动时创建客户端并注入业务，多次请求复用连接池；不要每次调用都 New，也不要在单个 HTTP 业务处理器里 Close 共享客户端。请求结束不需要关闭 `HTTPResponse`，其正文已被公共包读取并释放。

## POST JSON

`PostJSON(ctx, url, data, headers)` 接受任何可 JSON 编码的 Go 值，默认 Content-Type 为 `application/json`，显式提供其他 Content-Type 时保留调用方设置。字段精度由业务结构决定，大整数使用 int64，不先转换为 float64。

```go
package examples

import (
    "context"
    "net/http"
    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

// CreateItem 演示业务结构、认证头和 POST；调用方按响应状态和业务语义决定后续处理。
func CreateItem(ctx context.Context, client *commonTool.HTTPClient, url, token string, id int64, name string) (*commonTool.HTTPResponse, error) {
    payload := struct {
        ID int64 `json:"id"`
        Name string `json:"name"`
    }{ID: id, Name: name}
    headers := http.Header{"Authorization": {"Bearer " + token}}
    return client.PostJSON(ctx, url, payload, headers)
}
```

nil 数据编码为 JSON `null`。编码失败保留标准 JSON 错误链，且不向服务端发送请求。调用前已经取消时也不会执行自定义 MarshalJSON。

## 响应与错误

`Get`、`PostJSON` 均返回 `(*HTTPResponse, error)`：

| 字段 | 含义 |
|---|---|
| `StatusCode` | 原始 HTTP 状态码 |
| `Header` | 独立复制的响应头，包括多值头 |
| `Body` | 完整正文，或读取失败/超限时已获得的前缀 |
| `BodyComplete` | 已读到 EOF 且未超限；不代表 HTTP 状态成功或正文是合法 JSON |

所有 2xx 均按成功状态处理，包括 201、202、204 和 206。空正文合法，204 不应强行按 JSON 解码。完整指本次响应正文，不代表 206 等响应包含整个资源。

非 2xx 返回 `*HTTPStatusError`，通过 `errors.As` 获取 StatusCode，仍返回有界的响应头和正文供业务判断。错误正文可能包含敏感信息，不要直接当作用户文案或完整写入日志。网络请求未收到响应头时 response 可以为 nil。

| 检查方式 | 用途 |
|---|---|
| `errors.As(err, &statusError)` | 非 2xx，包括未跟随的 3xx |
| `errors.Is(err, commonTool.ErrHTTPResponseTooLarge)` | 正文超过上限，Body 保留上限内前缀、BodyComplete=false |
| `errors.Is(err, context.Canceled/DeadlineExceeded)` | 上游取消或总预算到期 |
| `errors.As(err, &netError)`、`netError.Timeout()` | TCP、TLS、响应头等阶段超时也可通过 net.Error 识别 |
| `errors.Is(err, commonTool.ErrHTTPClientClosed)` | 客户端已关闭 |

同一错误可以同时包含状态码、读取错误、超限或关闭错误，使用 errors.Is/As，不匹配错误字符串。即使正文已完整读取，关闭正文失败时仍返回错误，因此 BodyComplete=true 不代表 err=nil。网络、TLS 和 JSON 错误链保留，公开错误文本不直接输出 URL、凭证、请求头值或服务端正文。

超限检测最多读取上限加一个字节，不无限 drain 剩余内容；读取、失败状态和取消均释放正文。默认自动 gzip 解压时按解压后大小计数，手动指定编码后的处理由业务负责。

## 请求头、链路与重发

调用方请求头会被复制并规范头名称大小写，不修改原 map 或多值切片。需要 Host 覆盖时设置一个非空 Host 头，工具写入标准库的 Request.Host；Content-Length 等传输控制字段由标准库根据正文管理。调用期间不要从另一个 goroutine 并发修改同一传入 map。

工具沿用 ctx 的取消、截止时间和已有值，不自行生成或注入 Trace ID；跨 HTTP 传递已有统一 ID 时由业务显式设置请求头。API/RPC 的当前上下文应继续使用，不能换成 Background：

```go
package examples

import (
    "context"
    "net/http"
    "github.com/K7mQ2vR9/base-pkg/commonTool"
    "github.com/K7mQ2vR9/base-pkg/headInfo"
)

// GetForRequest 将当前请求已有的统一 ID 带给另一个 HTTP API，不重新生成 ID。
func GetForRequest(ctx context.Context, client *commonTool.HTTPClient, url string) (*commonTool.HTTPResponse, error) {
    headers := make(http.Header)
    if id := headInfo.GetTrace(ctx); id != "" {
        headers.Set("X-Request-ID", id)
    }
    return client.Get(ctx, url, headers)
}
```

完整 ID 规则见 [统一请求 ID](../headInfo/TRACE_ID.md)。是否把链路头发给某个外部接口由业务决定。

客户端不自动跟随重定向，3xx 连同 Location 返回业务，避免 POST 被自动改为 GET 或重复发送。工具不增加业务重试循环，POST 不提供已发送正文的自动重建或重放，即使有 Idempotency-Key 也不增加业务重试。尚未发送请求时的底层连接恢复仍由标准库控制，GET 也保留标准库自身的连接恢复行为，均受同一次调用总预算限制。传输失败不证明服务端未处理请求，写操作按业务稳定标识保证幂等。

## 关闭

`Close()` 可重复或并发调用，拒绝后续请求并释放空闲连接，不强制取消在途调用。在途请求由自己的 ctx/总预算完成或取消。服务应先停止新业务、等待在途请求收尾再关闭客户端。

本轮以真实本机 HTTP/TLS、故障响应替身验证，不连接外部业务接口；目标接口认证、响应结构和部署网络仍需业务联调。
