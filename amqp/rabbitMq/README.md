# RabbitMQ 接入

通过独立 `rabbitmq.Config` 配置。应用公共配置已提供可选的 `ConfigAppServer.RabbitMQ` 字段，资源由服务启动代码初始化并在退出时关闭。新项目使用新接口，无旧构造函数及方法签名兼容层。

请求链路统一采用 Request ID = Trace ID，发布、消费、重试、失败归档和 TG 告警沿用同一个值；生成及校验规则见 [统一请求 ID 规则](../../headInfo/TRACE_ID.md)。`Message.ID` 继续独立标识具体消息。

## 消息流程

1. 业务处理成功后单条 ACK，所有 ACK/NACK 均使用 `multiple=false`。
2. 普通业务错误转入 `<QueueName>.retry`，等待 broker 确认转存成功后才 ACK 原消息。`Permanent(err)` 及默认不可恢复的 JSON 错误直接归档，不进入业务重试。
3. 独立重试消费者等待指定时间后，通过默认交换机定向重投原业务队列，确认重投成功后才 ACK 重试消息。
4. 默认首次处理之外重试 3 次，每次间隔 30s。第 4 次仍失败时，先通过默认交换机定向发布到 `<QueueName>.failed`，确认成功后再定向发布到 `<QueueName>.notify`；两份均确认成功后才 ACK 当前消息。永久错误和元数据错误采用相同的双份转存规则。
5. 失败队列保留原消息；通知消费者在终态归档后发送 TG，分别标明永久错误、重试耗尽或元数据错误。TG 发送成功才 ACK 通知任务，失败则独立等待重试，并遵守 `retry_after`，不占用业务消费者。TG 持续故障期间通知会积压。

业务处理器 panic 按失败处理。内部重试次数或时间头损坏时直接归档并创建通知任务。原消息正文、消息 ID 和常用属性在转存中保留，错误摘要最多 1024 字节；内部转存强制持久化，不沿用原消息的 TTL 和生产者 `UserId`。

## 配置

```yaml
RabbitMQ:
  URL: "amqp://user:password@127.0.0.1:5672/platform"
  QueueName: "platform.jobs"
  Exchange: "platform.events"
  ExchangeType: "direct"
  RoutingKey: "platform.jobs"
  WorkerNum: 4
  PrefetchCount: 4
  HandlerTimeout: 30s
  MaxRetries: 3
  RetryDelay: 30s
  Telegram:
    Enabled: false
```

TG 默认关闭。启用时提供 `Telegram.Enabled: true`、`BotToken` 和 `ChatID`，凭证从部署配置注入。通知内容为服务名、队列、消息 ID、处理次数和错误摘要，不包含消息正文。关闭 TG 时仍保留持久化通知任务，之后启用并注册消费者会处理积压任务。

| 配置 | 默认值及含义 |
|---|---|
| `URL`、`QueueName` | 必填 |
| `Exchange`、`ExchangeType`、`RoutingKey` | 默认交换机、`direct`、业务队列名；具名交换机自动绑定业务队列 |
| `QueueDurable`、`ExchangeDurable`、`Persistent` | `true`；辅助队列始终持久化，终态转存消息强制持久化 |
| `QueueAutoDelete`、`QueueExclusive` | `false` |
| `DialTimeout` | `5s`，连接、握手及拓扑初始化的总时限 |
| `Heartbeat` | `10s`；显式 `0` 采用 broker 间隔，其他值为 1～65535 整秒 |
| `ReconnectInterval` | `5s`，连接失败或消费转存失败后的退避 |
| `ConfirmTimeout` | `5s`，发布串行等待、写入及确认的总时限 |
| `HandlerTimeout` | `30s`，每次业务回调的独立处理时限，`0` 使用默认值 |
| `ShutdownTimeout` | `10s`，等待处理器响应取消并完成收尾 |
| `WorkerNum`、`PrefetchCount` | `1`、跟随业务并发；预取为 0～65535，显式 `0` 不限制 |
| `AutoAck` | `false`，可靠转存需要保持手动确认；设为 `true` 时转存失败无法恢复原消息 |
| `MaxRetries`、`RetryDelay` | `3`、`30s`；显式 `MaxRetries: 0` 在首次处理失败后直接归档 |
| `RetryQueue`、`FailedQueue`、`NotifyQueue` | 业务队列名加 `.retry`、`.failed`、`.notify`，各队列名须不同 |
| `ServiceName` | 业务队列名，用于 TG 通知 |
| `Telegram.Timeout`、`Telegram.RetryInterval` | `5s`、`30s` |

指针配置区分缺省值和显式零值或 `false`。其余时长字段填 `0` 时使用默认值，负值不允许。已有同名队列或交换机的属性须与配置一致，初始化不会删除或改建已有资源。

## 失败归档与通知的投递确认

失败队列和通知队列按各自队列名定向发送，每次发布独立使用 `mandatory` 和发布确认，分别检查目标是否可路由。归档未确认时不会发送通知；任一目标不存在、发布被拒绝、确认超时或连接中断，均不 ACK 原消息。日志错误区分保存失败消息和创建通知任务两个阶段。默认手动确认模式下，转存失败的原消息由消费流程重新入队；连接已失效时交由 broker 恢复投递。

两次发送分别受 `ConfirmTimeout` 限制，双份转存最多占用两次发布额度，同时受消费生命周期取消约束。两份确认完成后再次检查取消状态，再单条 ACK。TG 关闭时仍投递通知任务并等待确认，以便以后启用后处理积压；TG HTTP 发送在独立消费者中进行，不计入这里的两个发布确认。

两次发布不是原子事务。如果归档已成功、通知失败，重新处理可能再次归档；确认结果不确定或原消息 ACK 失败时，也可能重复归档或通知。重投保留已有消息 ID，归档处理按消息 ID 识别重复，业务回调仍须幂等。外部消息未设置 ID 时，本次处理生成的 ID 在两份副本中一致，但未确认源消息断线重投可能重新生成，因此生产者应预先指定稳定 ID。

已删除专用失败广播交换机的代码，不保留旧投递路径。初始化只声明业务所需交换机和四个队列，不自动删除 broker 上已有的历史资源。

## 业务处理超时

`HandlerTimeout` 从调用业务处理器时开始计时，每条消息、每次重试都获得新的处理额度；不包含消费前排队、重试队列等待、消息转存和 TG 发送。长任务按队列配置更长的时限。业务中的 MySQL、Redis、HTTP 和 RPC 操作须使用处理器传入的 `ctx`，才能响应处理超时及服务关闭。回调返回后该上下文随即取消并释放，不用于回调外的后台任务。

业务在时限内返回 `nil` 才算成功。达到时限会取消业务上下文，并立即记录队列、消息 ID、时限、耗时及当前 Trace ID。处理器退出后按超时失败进入重试；即使超时后返回 `nil`，也按失败处理。重试耗尽后继续按已有规则归档和发送 TG，不在每次超时时重复发送 TG。处理器可通过 `errors.Is(ctx.Err(), context.DeadlineExceeded)` 识别超时，内部超时错误也保留该错误链，消费 Span 标记为错误。`Consume` 返回值仅表示订阅初始化结果，每条消息的超时通过日志、Span 和重试队列观察。

超时后的重试或归档使用消费生命周期上下文，并受独立的 `ConfirmTimeout` 约束，不使用已经过期的业务子上下文。仅在转存收到 broker 确认后单条 ACK 原消息，转存失败保留原消息。服务退出或连接失效优先于处理结果：生命周期已取消时不把任务当作业务失败重试，也不确认取消后的消息，由连接关闭后的 broker 恢复投递。可靠处理要求保持默认的 `AutoAck: false`。

回调在原 worker 内同步执行。Go 无法强制终止不响应 `ctx` 的业务代码，这类任务会继续占用该 worker，并产生超时日志；回调退出前不会提前 ACK、转存或主动重投。它不退出时也无法推进到重试耗尽通知。超过关闭时限仍会断开连接，旧回调须自行退出；broker 因断线重投时仍可能发生重复执行，业务应按稳定标识幂等处理。

## 使用范例

完整可运行范例、业务自增消息 ID、JSON 编解码、稳定 ID 重发和批量部分成功恢复见 [使用说明](USAGE.md)。永久错误、显式重试及 TG 通知示例见 [消费错误处理说明](ERROR_HANDLING.md)。

当前业务接口使用 Message：

```go
messageID, err := client.PublishJSON(ctx, data)
err = client.Consume(func(ctx context.Context, msg rabbitmq.Message) error {
    var event JobEvent
    if err := msg.DecodeJSON(&event); err != nil {
        return err
    }
    return handleJob(ctx, event)
})
```

Publish(ctx, Message) 返回实际消息 ID 和 error；PublishBatch(ctx, []Message) 返回 BatchResult 和 error。业务传入的消息正文、嵌套头及 ID 不会被框架修改，消费处理器拿到独立副本。自动消息 ID 使用 UUIDv7，不是自增数字；同一条消息的重发沿用原 ID，业务仍须保证幂等。旧的驱动 Publishing 发布签名及 []byte 回调不再提供。

TG 发送也可能因响应丢失或 ACK 失败而重复通知。失败队列默认保留消息，归档消费、人工重放和存储保留策略由服务维护。连接中断时旧处理器上下文取消，未确认消息由 broker 恢复投递。超过关闭时限会强制断开连接并返回超时，忽略上下文的业务回调须自行退出。
