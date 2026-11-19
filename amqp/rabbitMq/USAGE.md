# RabbitMQ 使用说明

适用于 `github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq`。配置默认值、重试流程和超时边界见 [配置与运行说明](README.md)。业务统一使用 `Message`，无需导入 AMQP 驱动或操作 ACK/NACK。

## 消息 ID：支持业务自增 ID

`Message.ID` 是字符串。可以把数据库业务自增 ID 用 `strconv.FormatInt(businessID, 10)` 转成字符串，通过 `WithMessageID` 或 `Message.ID` 指定。指定后公共包原样使用，不再生成 UUID。

没有指定时，`NewJSONMessage` 或实际进入消息准备阶段的 `Publish` / `PublishJSON` 自动分配 **36 位 UUIDv7**。它包含时间与随机部分，按时间大致有序，**不是 1、2、3 的自增数字，同一毫秒内不保证递增**。重新构造并发送一个未指定 ID 的消息会获得新 ID；重发同一条逻辑消息须复用原 ID，包内消费重试也会沿用原 ID。

选择 ID 时按逻辑消息区分：

- 一条业务记录只对应一条逻辑消息，可以直接使用自增 ID，例如 `123`。
- 同一个 ID 可能来自不同业务表或不同事件，使用 `task:123:created` 等前缀组合。
- 同一条业务记录会产生多个版本或事件，包含事件名、版本或独立的事件记录 ID，避免不同消息共用 ID。

ID 须非空白且不超过 255 **字节**，不会自动修剪。当前封装不按 ID 自动去重，重复投递仍需由业务通过唯一键、幂等记录或事务处理。Trace ID 继续使用独立的 32 位小写十六进制值；消息 ID 的业务前缀和自增格式不受 Trace ID 的格式约束。一次请求可以发布多条不同 ID 的消息。

## 完整运行范例

以下程序同时演示初始化、注册消费者、使用业务自增 ID 发布 JSON 和退出关闭。将它保存为 `main.go`，在已接入公共包的 Go 项目中运行；从环境变量 `RABBITMQ_URL` 提供真实连接地址。示例默认关闭 TG，业务处理只演示解码和日志，实际数据库/RPC 操作须传入回调的 `ctx` 并实现幂等。

```go
package main

import (
    "context"
    "errors"
    "log"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    rabbitmq "github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq"
    "github.com/K7mQ2vR9/base-pkg/headInfo"
)

type TaskCreated struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}

func handleTask(ctx context.Context, msg rabbitmq.Message) error {
    var event TaskCreated
    if err := msg.DecodeJSON(&event); err != nil {
        return err
    }
    if event.ID <= 0 {
        return rabbitmq.Permanent(errors.New("task ID must be positive"))
    }
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    // 在此调用业务服务：使用 ctx，按逻辑消息或业务标识幂等处理。
    log.Printf("task=%d message=%s retry=%d redelivered=%t trace=%s",
        event.ID, msg.ID, msg.RetryCount, msg.Redelivered, headInfo.GetTrace(ctx))
    return nil
}

func main() {
    appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    startupCtx, cancelStartup := context.WithTimeout(appCtx, 5*time.Second)
    client, err := rabbitmq.NewRabbitMQ(startupCtx, rabbitmq.Config{
        URL: os.Getenv("RABBITMQ_URL"), QueueName: "platform.tasks",
        HandlerTimeout: 30*time.Second,
    })
    cancelStartup()
    if err != nil {
        log.Printf("initialize RabbitMQ: %v", err)
        return
    }
    defer client.Close()
    // 订阅初始化或发布过程中收到退出信号，也能触发关闭；Close 支持重复或并发调用。
    stopClosing := context.AfterFunc(appCtx, func() { _ = client.Close() })
    defer stopClosing()
    if err := client.Consume(handleTask); err != nil {
        log.Printf("register consumer: %v", err)
        return
    }

    event := TaskCreated{ID: 123, Name: "example"}
    publishCtx, err := headInfo.EnsureTrace(appCtx)
    if err != nil {
        log.Printf("initialize task trace: %v", err)
        return
    }
    messageID, err := client.PublishJSON(publishCtx, event,
        rabbitmq.WithMessageID(strconv.FormatInt(event.ID, 10)),
        rabbitmq.WithHeaders(map[string]any{"event_type": "task.created"}),
    )
    if err != nil {
        log.Printf("publish message=%q unknown=%t: %v",
            messageID, errors.Is(err, rabbitmq.ErrPublishUnknown), err)
        // 根据错误保存失败任务及已有 ID，稍后由业务决定重发，不盲目循环重发。
    } else {
        log.Printf("broker confirmed message=%s", messageID)
    }

    // Consume 注册成功后即返回；客户端须保持存活直到服务退出。
    <-appCtx.Done()
    if err := client.Close(); err != nil {
        log.Printf("close RabbitMQ: %v", err)
    }
}
```

取消初始化上下文不会关闭已创建的客户端。服务退出要调用 `Close`；不要在创建消费者的短函数里 `defer Close` 后立即返回。

## 发布接口与 ID 返回值

| 接口 | 用途 |
|---|---|
| `Publish(ctx, Message) (string, error)` | 发布已有字节正文和消息元数据 |
| `PublishJSON(ctx, data, options...) (string, error)` | 自动 JSON 编码并发布 |
| `NewJSONMessage(data, options...) (Message, error)` | 不连接 broker，先生成 ID 并准备 JSON 消息，适合保存或批量发布 |
| `PublishBatch(ctx, []Message) (BatchResult, error)` | 顺序发送，首次失败停止，返回部分成功结果 |

已分配 ID 后，发送失败也会返回该 ID，包括 broker 拒绝、未路由和结果不确定。失败发生在上下文预检查、选项校验或 ID 分配之前时，返回的 ID 可以为空；不要把空 ID 当作稳定的重发标识。JSON 编码失败不会发送消息，标准 JSON 错误仍可通过 `errors.As` 检查。

`Publish` 默认 JSON ContentType，发送其他格式时显式填写类型。以下函数可放入业务服务中：

```go
package examples

import (
    "context"
    rabbitmq "github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq"
)

func PublishBytes(ctx context.Context, client *rabbitmq.RabbitMQ, id string, payload []byte) (string, error) {
    return client.Publish(ctx, rabbitmq.Message{
        ID: id, Body: payload, ContentType: "application/octet-stream",
        CorrelationID: "task-group", Headers: map[string]any{"source": "api"},
    })
}
```

正文和消息头在发布时复制。元数据中的短字符串最多 255 字节，`Expiration` 使用非负毫秒数字串，空表示不设置 TTL。`Persistent` 不设置时沿用客户端配置。普通 Headers 使用字符串、布尔、int16/int32/int64、float32/float64、[]byte、time.Time、[]any 和嵌套 map；普通 `int` 须处于 int32 范围，大整数使用 int64。循环引用及驱动不支持的头类型会在发送前拒绝。

## 先准备消息，再发布或重发

使用 `NewJSONMessage`，在请求 broker 前获得稳定 ID 和 JSON 正文；可将消息保存到业务任务记录，重试时复用同一 ID、正文和业务头。

```go
package examples

import (
    "context"
    rabbitmq "github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq"
)

func PrepareAndPublish(ctx context.Context, client *rabbitmq.RabbitMQ, data any, businessMessageID string) (rabbitmq.Message, error) {
    msg, err := rabbitmq.NewJSONMessage(data, rabbitmq.WithMessageID(businessMessageID))
    if err != nil {
        return msg, err
    }
    actualID, err := client.Publish(ctx, msg)
    msg.ID = actualID
    // 失败时把 msg 与错误返回给任务调度层，之后可直接 client.Publish(retryCtx, msg)。
    return msg, err
}
```

不传 `WithMessageID` 或传入空字符串时自动生成 UUIDv7。使用 `PublishJSON` 的自动 ID 发送失败后，也可以在下一次调用里加 `WithMessageID(previousID)` 复用它，不能省略 ID 再构造一条新消息。

## 批量部分成功及恢复

`BatchResult.MessageIDs` 对应输入顺序；已尝试项包含实际 ID，未尝试项保留原 ID，未分配时为空。`ConfirmedCount` 是已收到 broker 成功确认且已路由的连续前缀数量；`FailedIndex` 从 0 开始，全部成功或批次初始化失败时为 -1。**失败那条消息可能已被接收，特别是 `ErrPublishUnknown`；其后的消息在本次调用中尚未尝试。** 重发时跳过已确认的前缀，并保留失败消息的 ID。

批量 JSON 可以先逐条调用 `NewJSONMessage`，再把得到的 `[]Message` 交给 `PublishBatch`。以下恢复函数同时适用于未提前分配 ID 的字节消息：

```go
package examples

import (
    "context"
    rabbitmq "github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq"
)

func PublishWithResume(ctx context.Context, client *rabbitmq.RabbitMQ, messages []rabbitmq.Message) ([]rabbitmq.Message, rabbitmq.BatchResult, error) {
    result, err := client.PublishBatch(ctx, messages)
    if err == nil {
        return nil, result, nil
    }
    start := result.FailedIndex
    if start < 0 {
        start = 0 // 如上下文预检查失败，尚未开始发送任何一条。
    }
    pending := append([]rabbitmq.Message(nil), messages[start:]...)
    for offset := range pending {
        if id := result.MessageIDs[start+offset]; id != "" {
            pending[offset].ID = id
        }
    }
    // 保存 pending 和失败原因；需要重试时只发送 pending，跳过已确认的前缀。
    return pending, result, err
}
```

公共包不会修改传入切片，也不会自动重发。调用方保存待重发消息时保留原始正文、业务头和 ID，不要以同一个 ID 发送不同业务内容。

## 消费 Message 与确认规则

以下确认规则以默认 `AutoAck: false` 为前提；开启自动确认时只能尽力转存，无法恢复已由 broker 自动确认的源消息。

- `ID`、`Body`、`Headers` 和常用内容属性来自当前消息。业务拿到独立副本，修改正文、嵌套头或属性不会改变框架待重试、待归档的源消息。
- `RetryCount` 首次为 0，表示本包业务重试次数。`Redelivered` 是 broker 重新投递提示，两者不同，均不能替代幂等处理。
- `Exchange`、`RoutingKey` 是当前投递来源，仅供消费读取。发布目标由客户端 Config 决定，设置这些消费字段不会覆盖发送目标。
- `DecodeJSON` 采用标准 JSON 解码语义，不做业务字段校验。回调返回的标准 JSON 语法、字段类型或解码目标错误默认直接归档；自定义解码器的其他错误默认重试。业务参数无效可返回 `Permanent(err)`，需要覆盖默认分类时返回 `Retryable(err)`，仍受最大重试次数限制。完整范例见 [消费错误处理说明](ERROR_HANDLING.md)。
- 不调用 ACK/NACK。回调在处理时限内返回 nil 且生命周期有效时，框架单条 ACK；失败转存获得 broker 确认后才 ACK。
- 终态失败先定向发送失败队列，确认后再发送通知队列，两份都确认且生命周期有效才 ACK。任一步失败不 ACK；两次发送非原子，部分成功后的重投可能重复归档或通知，详见 [双份投递与确认](README.md#失败归档与通知的投递确认)。
- 每次处理有独立的 HandlerTimeout，默认 30s。使用传入的 ctx 调用数据库、Redis、HTTP 和 RPC。超时后返回 nil 也按失败处理；回调结束后 ctx 取消，不用于回调外的后台任务。
- `Consume` 的返回值只表示订阅初始化；处理器的后续错误通过日志、Span、重试和失败队列观察。不响应取消的处理器仍会占用原 worker。

## 发布错误与链路

| 错误 | 含义 |
|---|---|
| `ErrClosed` | 客户端已关闭 |
| `ErrNotReady` | 连接正在恢复，本次尚未进入发布 |
| `ErrUnroutable` | mandatory 发布没有匹配到队列 |
| `ErrPublishRejected` | broker 返回发布 NACK |
| `ErrPublishUnknown` | 无法确定是否被 broker 接收，重发须沿用 ID 并保证消费幂等 |

使用 `errors.Is` 检查错误；批量包装错误也保留该能力。成功确认只表示 broker 已接收并路由，不表示业务已经处理完成。

API 或消息回调的上下文已包含 Trace ID，继续传给发布或下游 RPC 即可；独立后台任务在入口先调用一次 `headInfo.EnsureTrace`，再复用返回的上下文。内部只通过标准 W3C 头传递链路，不添加旧 ID 别名。Message.ID 和业务 `CorrelationID` 不替代 Trace ID。
