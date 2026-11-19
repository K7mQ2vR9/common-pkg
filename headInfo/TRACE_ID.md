# 统一请求 ID 规则

平台只维护一个 **Trace ID**。HTTP 对外通过 `X-Request-ID` 接收和返回该值，响应正文使用 `trace` 字段。RPC 与 RabbitMQ 只通过 W3C `traceparent` 和必要的 `tracestate` 传播链路；不再附加独立 ID 别名。各步骤通过不同的 Span ID 区分操作。

## 生成规则

1. 使用加密安全随机源生成 **16 个随机字节（128 位）**。
2. 编码为 **32 位小写十六进制字符串**，字符范围只能是 `0-9a-f`。
3. 全零值无效，生成时遇到全零应重新生成。随机源读取失败返回错误，不使用时间戳或普通伪随机数降级。
4. 不添加业务前缀、时间戳、空格或连字符，不使用带连字符的 UUID 文本。
5. 每次全新请求在最外层生成一次。已经存在合法 ID 时复用，API、RPC、消息消费和消息重试不得为同一已有链路另生成 ID。

示例：`4bf92f3577b34da6a3ce929d0e0e4736`。

校验条件：匹配 `^[0-9a-f]{32}$` 且不等于 `00000000000000000000000000000000`。大写、前后空格、空字符串和重复字段均拒绝，不自动修剪或转换大小写。规则对应 [W3C Trace Context 的 trace-id 格式](https://www.w3.org/TR/trace-context/#trace-id)。

Go 网关和无上游链路的新任务调用公共方法：

```go
id, err := headInfo.NewTraceID()
if err != nil {
    return err
}
```

需要把 ID 绑定到上下文时使用 `headInfo.SetTrace(ctx, id)` 并处理其返回的 `error`。它返回 `(context.Context, error)`，不会覆盖不同的活动链路。独立后台任务也可先调用一次 `headInfo.EnsureTrace(ctx)`，再把返回的上下文传给该任务的全部 RPC 和 MQ 操作。

前端生成示例，使用浏览器 Web Crypto：

```javascript
function newRequestID() {
  const bytes = new Uint8Array(16);
  do {
    crypto.getRandomValues(bytes);
  } while (bytes.every(value => value === 0));
  return Array.from(bytes, value => value.toString(16).padStart(2, "0")).join("");
}
```

前端可以不传此请求头，由网关生成。自行传入时，将生成值写入 `X-Request-ID`；它不承担认证或业务去重功能。

## 入口处理与冲突

| 输入 | 处理规则 |
|---|---|
| 只有合法 `X-Request-ID` | 原样作为统一 Trace ID，补齐标准链路上下文 |
| 只有合法 `traceparent` | 沿用其中的 Trace ID，并同步 `X-Request-ID` |
| HTTP `X-Request-ID` 与 `traceparent` 同时存在 | 二者的 Trace ID 必须相同 |
| 所有 ID 和标准链路头都缺失 | 网关生成；绕过网关的 HTTP 入口由公共包兜底生成 |
| 字段为空、重复、格式错误或相互冲突 | HTTP 返回真实 `400`，RPC 返回 `InvalidArgument`，不进入业务 |
| RPC 缺少 `traceparent` | 直接拒绝，包括白名单方法；其他字段或已有服务端 Span 均不能替代 |

HTTP 校验失败时可为错误响应建立新的诊断 ID，非法输入不会被当作合法业务 ID 转发。HTTP 随机源失败返回 `500`。网关应在创建自己的服务端 Span 之前执行相同校验和初始化，向下游同时转发 `X-Request-ID` 和标准 `traceparent`。实际网关类型与配置尚未接入本仓库。

## API 接入位置

创建 API Server 时使用公共包入口：

```go
server, err := app.NewServer(restConf)
if err != nil {
    return err
}
// 然后按原流程装配 ServerMiddleware、认证及业务路由。
```

`app.NewServer` 在路由最外层接入统一 ID，先于 go-zero 原生 Trace、日志和业务中间件。应用中间件及 ApiHeaderMiddleware 只读取已初始化的上下文，不再承担链路初始化。

使用自定义路由时，通过 `app.WithRequestTraceRouter(customRouter, restConf.Telemetry.Sampler)` 包装，作为 `rest.NewServer` 的选项；自定义 NotFound、NotAllowed 或 CORS 选项在其后设置。仅有 ID 的新链路按服务配置的采样比例决定是否采样，有标准上游上下文时保留其采样决定，采样结果不改变统一 ID。

普通 HTTP 响应通过 `X-Request-ID` 返回该值，响应正文继续使用 `trace` 字段。公共 CORS 中间件允许浏览器读取 `X-Request-ID`。业务统一调用 `headInfo.GetTrace(ctx)` 获取此 ID，不从未校验的原始请求头再次取值。

## RPC 接入位置

```go
auth, err := appMiddleware.NewRpcAuthMiddleware(checkCaller)
if err != nil {
    return err
}
server.AddOptions(auth.ServerOption(publicMethods))
```

使用 `ServerOption` 将入口校验放在 go-zero 原生 gRPC 链路拦截器之前；同一 Server 只设置一次主 `UnaryInterceptor`。其他拦截器继续按普通链追加，认证中间件只公开此接入入口。

基础包客户端拦截器仅注入 W3C 上下文，服务端严格校验 traceparent 后继续向下游传递。已有活动 Span 和原始 metadata 保持隔离。内部正常调用直接使用 API 或消息处理器传入的上下文；自行启动一组后台 RPC 时先在任务入口初始化一次上下文。

## MQ 传递

RabbitMQ 发布复制消息头，仅写入标准链路字段，不修改调用方的原始消息头。消息 ID `MessageId` 仍独立标识具体消息，同一次请求可以发布多条消息。

消费者从消息头恢复同一个 Trace ID，在消费、重试调度和后续 RPC 调用中沿用；重试转存、失败归档和 TG 告警保留该 ID。生产请求的超时或取消不会被序列化到消费者，消费上下文受当前服务生命周期控制。非法或冲突链路消息不执行业务，直接归档并创建告警任务。

独立发布缺少全部链路字段时生成新链路。外部生产者未携带链路头时，消费者为当前处理生成 ID；要在原消息因断线重新投递时也保持稳定，生产者应在发送前写入链路头。平台的 `Publish` 已自动执行该步骤。

## 当前接口

业务通过 `headInfo.GetTrace(ctx)` 获取唯一 ID，`Head` 只保留 `Trace` 字段，日志中的 Trace ID 由 go-zero 从活动 Span 自动输出。旧的 `Head.RequestID`、`consts.RequestId`、`consts.Trace`、`GetTrance` 和 RPC 公开 `Handle` 已删除；`request_id`、`trace` 等非标准头不作为链路输入，也不由公共包注入。HTTP `X-Request-ID` 是当前对外协议，不用于替代 RPC 的 `traceparent`。
