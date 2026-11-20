# RPC 客户端使用说明

使用 `github.com/K7mQ2vR9/base-pkg/rpc` 提供的共享 `Client`，保留当前 go-zero 目标解析/负载均衡以及统一 Trace、metadata 和业务错误协议。

```text
NewClient(ctx context.Context, conf Config) (*Client, error)
MustNewClient(ctx context.Context, conf Config) *Client
(*Client).Conn() *grpc.ClientConn
(*Client).Close() error
```

`Client` 实现 `zrpc.Client`，可传给接收该接口的业务客户端构造函数；原生 protobuf 客户端使用 `client.Conn()`。服务启动时初始化一次并复用，服务退出时关闭，不为每个请求重新创建连接。

## 独立配置与默认值

```yaml
Target: "dns:///users-rpc:9000"
RpcName: "users-rpc"
CallTimeout: 10s
ConnectTimeout: 5s
NonBlock: false
Keepalive:
  Enabled: false
  Time: 5m
  Timeout: 20s
  PermitWithoutStream: false
```

| 配置 | 用途和默认值 |
|---|---|
| `Target` | 必填，gRPC 目标，例如 `127.0.0.1:9000`、`dns:///users-rpc:9000`、`direct:///host:9000`；禁止空白和控制字符，目标语法由底层解析器处理 |
| `RpcName` | 必填，非空白可打印 ASCII 名称，用于 `rpc_name` metadata |
| `CallTimeout` | 一元调用总预算，默认 10s，包含等待连接和响应；父级更早截止时间优先 |
| `ConnectTimeout` | 启动连接等待预算，默认 5s，与启动 ctx 较早的截止时间共同限制 Ready 等待 |
| `NonBlock` | 默认 false，等待传输连接 Ready；true 仅创建连接，不保证下游已可用 |
| `Keepalive.Enabled` | 默认 false，不主动开启 HTTP/2 PING |
| `Keepalive.Time` | 开启后的无传输活动探测间隔，零值默认 5m，不得小于 10s |
| `Keepalive.Timeout` | PING 等待响应时限，零值默认 20s |
| `Keepalive.PermitWithoutStream` | 默认 false，无活跃 RPC 时不发送 PING，仅在服务端明确允许时开启 |

所有时长使用 `time.Duration`，0 采用默认值，负值拒绝；不开启无限调用或无限启动等待。Keepalive 的其余字段仅在 Enabled=true 时应用，负值即使未开启也拒绝。配置以值复制方式处理，不修改调用方对象。

Keepalive 不是健康检查，需与服务端和代理的 PING 策略配合。原 `KeepaliveTime: 30000` 实际为 30 微秒，新接口使用 `5*time.Minute` 等明确单位，并拒绝开启时过小的间隔。

底层沿用当前 go-zero 的默认明文传输配置，本轮没有增加 TLS 或身份凭证配置；应用既有 metadata 和认证协议继续使用。

## 初始化与独立调用范例

下例从 `RPC_TARGET` 获取提供标准 gRPC Health 接口的服务地址，按需通过 `RPC_HEALTH_SERVICE` 选择健康检查服务。业务调用替换为对应生成的客户端和请求。本轮仅编译范例，不连接外部服务。

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/K7mQ2vR9/base-pkg/headInfo"
    platformrpc "github.com/K7mQ2vR9/base-pkg/rpc"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
    startup, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    client, err := platformrpc.NewClient(startup, platformrpc.Config{
        Target: os.Getenv("RPC_TARGET"),
        RpcName: "example-rpc",
    })
    cancel()
    if err != nil {
        log.Printf("initialize RPC client: %v", err)
        return
    }
    defer client.Close()

    // 独立后台工作先初始化一次链路；API/RPC 业务中使用已有请求 ctx。
    ctx, err := headInfo.EnsureTrace(context.Background())
    if err != nil {
        log.Printf("initialize request trace: %v", err)
        return
    }
    response, err := healthpb.NewHealthClient(client.Conn()).Check(ctx, &healthpb.HealthCheckRequest{
        Service: os.Getenv("RPC_HEALTH_SERVICE"),
    })
    if err != nil {
        log.Printf("check service: %v", err)
        return
    }
    log.Printf("health status=%s", response.Status.String())
}
```

默认启动流程先用 go-zero 非阻塞创建连接，再通过本次启动上下文等待连接 Ready，绕开其内部固定 3s 阻塞等待。启动取消/超时立即关闭本次连接并返回错误，`errors.Is` 可识别 `context.Canceled`/`DeadlineExceeded`。Ready 表示传输就绪，不证明业务健康或认证成功。

初始化 ctx 只控制启动，成功返回后取消它不关闭客户端；请求使用自己的业务 ctx。`MustNewClient` 复用相同逻辑，失败时 panic，不调用进程退出函数。可恢复的启动错误使用 `NewClient`。

## 一元调用与上下文

客户端显式在一元拦截器中为每次调用派生 CallTimeout，包含等待连接和响应，保留父级值/取消及更早截止时间，结束时释放派生上下文。此预算不依赖 go-zero 配置中间件是否开启，不使用毫秒整数转换。

```go
package examples

import (
    "context"

    platformrpc "github.com/K7mQ2vR9/base-pkg/rpc"
    "google.golang.org/grpc"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// CheckForRequest 沿用当前 API/RPC ctx；共享 client 由服务初始化后注入。
func CheckForRequest(ctx context.Context, client *platformrpc.Client, service string) (*healthpb.HealthCheckResponse, error) {
    return healthpb.NewHealthClient(client.Conn()).Check(ctx, &healthpb.HealthCheckRequest{
        Service: service,
    }, grpc.WaitForReady(true))
}
```

`grpc.WaitForReady(true)` 可以在连接未就绪时等待，但仍受本次调用预算及父上下文限制。默认调用保持 gRPC 原生的失败处理方式；`NonBlock=true` 的创建成功不代表立即调用会成功。

现有拦截器继续复制 metadata，设置 RpcName，并沿用已有统一 Trace ID，通过 W3C `traceparent`/`tracestate` 传递。多个 RPC 调用应共享当前请求 ctx；独立后台工作先一次性 EnsureTrace 再复用，避免每次在全新 Background 上分别创建链路。规则见 [统一请求 ID](../headInfo/TRACE_ID.md)。

错误仍按现有 `result/xcode` 映射：取消/超时保留标准 context 错误链及 499/504，业务错误保留业务码和公开文案，认证/服务不可用按标准协议映射。使用 `errors.Is/As`，不匹配错误字符串。

自动预算目前覆盖一元 RPC；底层流式 RPC 的整个流生命周期由调用方提供上下文和截止时间，本轮未新增流式 Trace/错误映射封装。

## 重试与关闭

删除原固定三次 UNAVAILABLE 重试配置及 RetryDialOption，关闭服务配置驱动的策略重试。gRPC 对未写入网络或远端尚未处理请求的透明恢复仍保留，业务写操作继续按稳定标识保证幂等；连接断线后的后台重连是连接生命周期行为，不等同于重新执行业务请求。

`Close()` 可并发或重复调用，nil/零值实例安全，释放共享连接并取消在途 RPC；之后的新一元调用返回 `ErrClientClosed`。服务应先停止新业务，按需要等待在途业务收尾后再 Close，工具不实现业务排空。成功实例的 `Conn()` 在关闭后仍返回原连接，底层状态为 Shutdown；nil/零值实例 Conn() 返回 nil，不能用于构造有效业务客户端。

优先使用 Client.Close 管理连接；若直接关闭底层 Conn，客户端包装层无法在那一刻更新自身关闭标记，后续调用会由 gRPC 拒绝。此后再次调用 Client.Close 仍安全。

## 接口迁移与验证范围

原 `Config.Host` 改为 `Target`，删除无处理逻辑的 `GenRpcTarget`；NewClient/MustNewClient 增加启动 ctx 并返回自有 Client，不保留旧签名。现有业务生成代码可继续通过 zrpc.Client 接口或 Conn() 使用连接。

定向测试使用真实本机 gRPC、延迟就绪/握手不响应服务、可配置服务解析器和内存预算检查；不连接外部 RPC。Keepalive 验证参数校验和启用后的正常通信，未等待默认 5 分钟间隔验证线上 PING 时序；生产服务发现、认证和代理策略仍需接入联调。

底层连接创建使用当前 go-zero 的非阻塞接口，本包负责后续 Ready 等待及失败回收。若另外注册的第三方 resolver 在同步 Build 等回调中长时间阻塞，Go 无法通过启动 ctx 强制终止该回调，解析器需要自行实现时限。
