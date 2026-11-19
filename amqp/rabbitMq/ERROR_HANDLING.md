# RabbitMQ 消费错误处理说明

适用于 `github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq`。初始化、发布和退出的完整程序见 [使用说明](USAGE.md)，配置见 [配置与运行说明](README.md)。业务回调只返回错误，ACK、重试转存和终态归档由公共包处理。

## 错误分类

| 回调结果 | 处理行为 |
|---|---|
| 在处理时限内返回 `nil` | 单条 ACK |
| 普通错误、panic | 按 `MaxRetries` 重试，耗尽后归档并创建 TG 通知任务 |
| `Permanent(err)` | 直接归档并创建 TG 通知任务，不再进行业务重试 |
| 标准 JSON 语法、字段类型、解码目标错误 | 默认按永久错误处理 |
| `Retryable(err)` | 显式采用普通重试策略，可覆盖 JSON 默认分类或内层永久标记 |

默认 `MaxRetries=3`、`RetryDelay=30s`，即首次处理加最多 3 次重试。显式配置 `MaxRetries=0` 时，可重试错误在首次失败后也直接进入耗尽归档。永久错误可在首次或任意一次重试中返回，不继续消耗剩余次数。

标准 JSON 错误包括错误链中的 `*json.SyntaxError`、`*json.UnmarshalTypeError`、`*json.InvalidUnmarshalError`。最后一种通常表示传入 nil 或非指针等接入错误，重试同一消息无法修复。自定义 `UnmarshalJSON` 返回的其他错误默认仍可重试。JSON 编码失败发生在发布前，不进入消费错误分类。

公共包不按错误文案、HTTP/gRPC 状态码或业务码自动判断是否永久失败。业务根据错误语义选择标记，例如字段不合法、记录已删除且无需等待恢复可用 `Permanent`；依赖暂时不可用直接返回普通错误。

## 业务消费范例

下面代码可保存为业务包的 Go 文件。`TaskService.Execute` 由业务实现，沿用回调的 `ctx` 调用数据库、Redis 或 RPC，并按业务标识保证幂等。

```go
package examples

import (
    "context"
    "errors"
    "fmt"

    rabbitmq "github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq"
)

// ErrTaskGone 表示任务已被永久删除，不需要等待它重新出现。
var ErrTaskGone = errors.New("task permanently deleted")

type TaskEvent struct {
    ID int64 `json:"id"`
}

// TaskService 由业务提供；内部 I/O 使用 ctx，并实现幂等处理。
type TaskService interface {
    Execute(ctx context.Context, event TaskEvent) error
}

// NewTaskHandler 将领域错误转换为消费策略；普通依赖错误保持默认重试。
func NewTaskHandler(service TaskService) rabbitmq.Handler {
    return func(ctx context.Context, msg rabbitmq.Message) error {
        var event TaskEvent
        if err := msg.DecodeJSON(&event); err != nil {
            // 标准 JSON 格式或类型错误默认直接归档。
            return err
        }
        if event.ID <= 0 {
            return rabbitmq.Permanent(errors.New("task ID must be positive"))
        }
        if err := service.Execute(ctx, event); err != nil {
            if errors.Is(err, ErrTaskGone) {
                return rabbitmq.Permanent(err)
            }
            return fmt.Errorf("execute task %d: %w", event.ID, err)
        }
        return nil
    }
}

// RegisterTaskConsumer 在服务启动时注册；客户端保持存活，退出时统一 Close。
func RegisterTaskConsumer(client *rabbitmq.RabbitMQ, service TaskService) error {
    return client.Consume(NewTaskHandler(service))
}
```

字段缺失是否非法仍由业务校验：标准解码可能成功得到零值，需要像示例中的 ID 校验一样明确返回永久错误。业务错误文本会进入日志和通知摘要，请勿包含令牌、完整正文等敏感数据；`DecodeJSON` 的公开错误文本不包含原始正文。

## 显式要求重试

只有业务明确判断错误有恢复机会时才覆盖默认分类。下面函数演示接口写法；对正文固定不变的标准 JSON 错误，重复解码通常不会恢复。

```go
package examples

import rabbitmq "github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq"

// DecodeWithRetry 将解码错误显式交给有上限的业务重试策略。
func DecodeWithRetry(msg rabbitmq.Message, target any) error {
    if err := msg.DecodeJSON(target); err != nil {
        return rabbitmq.Retryable(err)
    }
    return nil
}
```

`Retryable` 不会无限重试，也不会跳过处理超时或服务关闭规则。普通依赖错误直接返回即可，无需每处包一层 `Retryable`。

## 错误包装与优先级

两个标记都保留原始错误文本和错误链，支持 `errors.Is`、`errors.As` 及 `fmt.Errorf("...: %w", err)`。`Permanent(nil)` 和 `Retryable(nil)` 都返回 nil，不会把成功结果变成失败。

```go
package examples

import (
    "errors"
    "fmt"

    rabbitmq "github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq"
)

// WrappedPolicy 演示保留错误链，以及外层策略覆盖内层策略。
func WrappedPolicy(cause error) (permanent error, retry error, sameCause bool) {
    permanent = rabbitmq.Permanent(cause)
    if permanent != nil {
        permanent = fmt.Errorf("validate task: %w", permanent)
    }
    retry = rabbitmq.Retryable(permanent)
    sameCause = errors.Is(retry, cause)
    return permanent, retry, sameCause
}
```

分类先查找显式标记，再判断默认 JSON 类型。普通 `%w` 包装保留策略；嵌套标记由最外层决定。`errors.Join` 等多分支错误按标准 `errors.As` 的深度优先、分支从左到右顺序找到第一个显式标记；没有任何显式标记时，任一分支包含上述标准 JSON 错误即归为永久错误。避免组合相互冲突的策略。

服务关闭或连接失效导致的生命周期取消优先：不按业务失败转存，也不 ACK 取消后的消息，交由 broker 在连接关闭后恢复投递。达到 `HandlerTimeout` 后返回的结果按超时处理，即使返回 nil 或 `Permanent(err)` 也按可重试超时处理；须等原回调退出后才转存，忽略 ctx 的回调会继续占用 worker。

## 归档、确认和 TG

以下可靠确认语义要求保持默认 `AutoAck: false`：

1. 永久错误、重试耗尽或内部元数据错误归档时，先经默认交换机按队列名定向发送到 `<QueueName>.failed`，收到确认后再定向发送到 `<QueueName>.notify`。
2. 两份都收到 broker 的成功确认，且消费生命周期仍有效，才单条 ACK 当前消息。每份发送分别受 ConfirmTimeout 限制，任一步失败或结果不确定都不会 ACK 原消息。
3. 通知消费者独立发送 TG，成功才 ACK 通知任务；发送失败独立重试，不重新执行业务处理。通知可能重复，业务执行也仍需幂等。

两份消息使用相同 ID、正文和失败原因。两次发布不是原子事务，归档成功而通知失败后重投，可能再次归档；发布确认丢失或源消息 ACK 失败，也可能重复归档或通知。归档处理按消息 ID 识别重复，业务回调须幂等；外部生产者预先设置稳定 ID，避免未确认源消息重投后重新生成 ID。原先通过失败广播交换机一次投递两份的代码已删除。

启用 `Telegram.Enabled` 并配置 `BotToken`、`ChatID` 后，永久错误归档便可通知，普通可重试错误只在耗尽后通知。关闭 TG 仍保存通知任务，之后启用并注册消费者可处理积压。消息 ID、正文和合法 Trace ID 在转存中保持；缺失或损坏的内部链路元数据会直接归档，本次不进入业务处理，通知使用可用的诊断链路 ID。

失败消息的内部头 `x-base-failure-reason` 用于排查：

| 值 | 含义 | TG 标题 |
|---|---|---|
| `permanent` | 显式永久错误或默认 JSON 永久错误 | 消息不可重试错误 |
| `retries_exhausted` | 可重试错误已达到次数限制 | 消息重试耗尽 |
| `invalid_metadata` | 链路、重试次数、重试时间等内部元数据无效 | 消息元数据错误 |

该头由公共包在归档时写入，业务或生产者传入同名值不决定错误分类，继续业务重试时会移除旧的终态原因。通知中的处理次数按本包 `RetryCount+1` 计算，不包含 broker 断线重投的额外执行次数；元数据错误显示“本次未执行”，不会推断历史总执行次数。缺失或未知归档原因显示通用失败信息。

本次范例经过编译，分类、确认时序和通知使用模拟 broker/HTTP 验证；实际 RabbitMQ、TG 凭证和部署环境仍需联调。
