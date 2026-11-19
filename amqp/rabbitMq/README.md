# RabbitMQ 接入

通过独立 `rabbitmq.Config` 配置。应用公共配置已提供可选的 `ConfigAppServer.RabbitMQ` 字段，资源由服务启动代码初始化并在退出时关闭。新项目使用新接口，无旧构造函数及方法签名兼容层。

请求链路统一采用 Request ID = Trace ID，发布、消费、重试、失败归档和 TG 告警沿用同一个值；生成及校验规则见 [统一请求 ID 规则](../../headInfo/TRACE_ID.md)。`MessageId` 继续独立标识具体消息。

## 消息流程

1. 业务处理成功后单条 ACK，所有 ACK/NACK 均使用 `multiple=false`。
2. 业务处理失败时转入 `<QueueName>.retry`，等待 broker 确认转存成功后才 ACK 原消息。
3. 独立重试消费者等待指定时间后，通过默认交换机定向重投原业务队列，确认重投成功后才 ACK 重试消息。
4. 默认首次处理之外重试 3 次，每次间隔 30s。第 4 次仍失败时发布到专用失败交换机，分别进入 `<QueueName>.failed` 和 `<QueueName>.notify`，收到发布确认后才 ACK 当前消息。
5. 失败队列保留原消息；通知消费者在重试耗尽后发送 TG。TG 发送成功才 ACK 通知任务，失败则独立等待重试，并遵守 `retry_after`，不占用业务消费者。TG 持续故障期间通知会积压。

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
| `QueueDurable`、`ExchangeDurable`、`Persistent` | `true`；辅助队列和失败交换机始终持久化 |
| `QueueAutoDelete`、`QueueExclusive` | `false` |
| `DialTimeout` | `5s`，连接、握手及拓扑初始化的总时限 |
| `Heartbeat` | `10s`；显式 `0` 采用 broker 间隔，其他值为 1～65535 整秒 |
| `ReconnectInterval` | `5s`，连接失败或消费转存失败后的退避 |
| `ConfirmTimeout` | `5s`，发布串行等待、写入及确认的总时限 |
| `ShutdownTimeout` | `10s`，等待处理器响应取消并完成收尾 |
| `WorkerNum`、`PrefetchCount` | `1`、跟随业务并发；预取为 0～65535，显式 `0` 不限制 |
| `AutoAck` | `false`，可靠转存需要保持手动确认；设为 `true` 时转存失败无法恢复原消息 |
| `MaxRetries`、`RetryDelay` | `3`、`30s`；显式 `MaxRetries: 0` 在首次处理失败后直接归档 |
| `RetryQueue`、`FailedQueue`、`NotifyQueue` | 业务队列名加 `.retry`、`.failed`、`.notify`，各队列名须不同 |
| `ServiceName` | 业务队列名，用于 TG 通知 |
| `Telegram.Timeout`、`Telegram.RetryInterval` | `5s`、`30s` |

指针配置区分缺省值和显式零值或 `false`。其余时长字段填 `0` 时使用默认值，负值不允许。已有同名队列或交换机的属性须与配置一致，初始化不会删除或改建已有资源。

## 调用

```go
// startupCtx 仅控制初始化；初始化成功后由 Close 管理客户端生命周期。
client, err := rabbitmq.NewRabbitMQ(startupCtx, *config.RabbitMQ)
if err != nil {
    return err
}

// 消费服务注册一次，重连后自动恢复；消费者同时启动重试和已启用的 TG 通知处理。
err = client.Consume(func(ctx context.Context, body []byte) error {
    return handleJob(ctx, body)
})
if err != nil {
    _ = client.Close()
    return err
}

// 生产者提供稳定的 MessageId，重发同一业务消息时沿用。
err = client.Publish(requestCtx, amqp.Publishing{
    MessageId: "job-identifier",
    ContentType: "application/json",
    Body: payload,
})

// 服务退出时调用，回调应及时响应 ctx 取消。
closeErr := client.Close()
```

`Publish` 成功表示 broker 确认且已匹配队列，默认启用 mandatory 和发布确认。连接恢复期间返回 `ErrNotReady`；超时或断线导致结果不确定时返回 `ErrPublishUnknown`，业务须处理可能重复投递。`PublishBatch` 顺序发布，失败时先前消息可能已经成功，调用方按稳定标识处理重发。正文应携带业务幂等标识，处理器使用该标识防止重复执行业务。

TG 发送也可能因响应丢失或 ACK 失败而重复通知。失败队列默认保留消息，归档消费、人工重放和存储保留策略由服务维护。连接中断时旧处理器上下文取消，未确认消息由 broker 恢复投递。超过关闭时限会强制断开连接并返回超时，忽略上下文的业务回调须自行退出。
