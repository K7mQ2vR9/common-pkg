# XXL-JOB 使用说明

本包管理 HTTP 监听、注册心跳、任务队列、取消和关闭，不再调用依赖执行器的 `Init/Run/Stop`，也不安装进程信号处理器或退出进程。原依赖仅复用 `RunReq`、`LogReq` 等协议数据结构。

本轮沿用已有 `/run`、`/kill`、`/beat`、`/idleBeat`、`/log` 与数组回调协议，以 [XXL-JOB 2.4.1 执行器协议实现](https://github.com/xuxueli/xxl-job/blob/2.4.1/xxl-job-core/src/main/java/com/xxl/job/core/biz/impl/ExecutorBizImpl.java) 和 [回调数据结构](https://github.com/xuxueli/xxl-job/blob/2.4.1/xxl-job-core/src/main/java/com/xxl/job/core/biz/model/HandleCallbackParam.java) 为校验基准。仅接受已注册的 Go BEAN 任务，不执行 GLUE 脚本。其他协议版本须单独联调，不自动猜测或切换协议。

## 配置与默认值

```yaml
AdminAddress: "https://scheduler.example.com/xxl-job-admin"
AccessToken: "${XXL_JOB_ACCESS_TOKEN}"
RegistryKey: "platform-jobs"
ListenAddress: ":9999"
ExecutorAddress: "http://worker.example.com:9999"
RequestTimeout: 5s
RegistryInterval: 20s
ShutdownTimeout: 10s
MaxBodyBytes: 1048576
MaxTasks: 1024
```

配置解析和环境变量替换由应用负责，直接构造 Go Config 时不会自动展开 `${...}`。所有新增配置都有中文注释；时长和容量字段零值采用默认值，负值非法。

| 配置 | 含义 |
|---|---|
| `AdminAddress` | 必填，调度中心根地址，可带 `/xxl-job-admin` 等路径，不包含 `/api/registry` |
| `AccessToken` | 必填，与调度中心一致；入站五个接口均检查令牌，拒绝缺失、错误和重复令牌头 |
| `RegistryKey` | 必填，为执行器 AppName |
| `ListenAddress` | 默认 `:9999`，本机绑定地址；允许 `127.0.0.1:0` 动态端口，但注册地址仍需单独配置 |
| `ExecutorAddress` | 必填，调度中心能访问的根地址，可经过反向代理；不能填 `0.0.0.0` 或 `::` |
| `RequestTimeout` | 默认 `5s`，每次注册/摘除/回调的总时限，也作为入站 HTTP 读写时限 |
| `RegistryInterval` | 默认 `20s`，首次注册后按此间隔续期；配置应短于调度中心的失效判定时间 |
| `ShutdownTimeout` | 默认 `10s`，整个关闭流程共用的预算，不为每个步骤重新计时 |
| `MaxBodyBytes` | 默认 `1MiB`，单次入站请求正文上限 |
| `MaxTasks` | 默认 `1024`，已接受但尚未完成的调度上限，包含运行、排队和等待回调 |

监听地址负责本机绑定，注册地址负责告诉调度中心如何访问当前执行器，两者不能混用。反向代理有路径前缀时需将 `/run` 等路径转发到本包根路由。监听本身为 HTTP，HTTPS 注册地址适用于外部代理终止 TLS 的部署。

## 接入范例

完整程序见 [demo/main.go](demo/main.go)。程序从环境读取连接信息，应用自己处理 SIGINT/SIGTERM；范例不自动运行，也不连接真实调度中心进行验证。

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/K7mQ2vR9/base-pkg/headInfo"
    "github.com/K7mQ2vR9/base-pkg/xxlJob"
    xxl "github.com/xxl-job/xxl-job-executor-go"
)

type ExampleTask struct{}

func (ExampleTask) Pattern() string { return "task.example" }
func (ExampleTask) Handler() xxlJob.HandlerFun {
    return func(ctx context.Context, param *xxl.RunReq) string {
        select {
        case <-ctx.Done():
            return "canceled"
        default:
        }
        // 数据库、RPC、MQ 操作传入 ctx；不要直接记录未经处理的任务参数或令牌。
        log.Printf("job=%d log=%d trace=%s", param.JobID, param.LogID, headInfo.GetTrace(ctx))
        return "finished"
    }
}

func main() {
    appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    client, err := xxlJob.NewClient(xxlJob.Config{
        AdminAddress: os.Getenv("XXL_JOB_ADMIN_ADDRESS"),
        AccessToken: os.Getenv("XXL_JOB_ACCESS_TOKEN"),
        ExecutorAddress: os.Getenv("XXL_JOB_EXECUTOR_ADDRESS"),
        ListenAddress: os.Getenv("XXL_JOB_LISTEN_ADDRESS"),
        RegistryKey: "platform-jobs",
    })
    if err != nil {
        log.Printf("initialize executor: %v", err)
        return
    }
    if err := client.Register(ExampleTask{}); err != nil {
        log.Printf("register task: %v", err)
        return
    }
    if err := client.Start(appCtx); err != nil {
        log.Printf("start executor: %v", err)
        return
    }
    if err := client.Wait(context.Background()); err != nil {
        log.Printf("executor stopped: %v", err)
    }
}
```

## 启动与关闭

- `NewClient(Config) (*Client, error)` 只校验配置，不监听、不注册；缺失令牌、注册地址或 AppName 会失败。
- `Register(...Handler) error` 在启动前原子注册一组处理器，拒绝 nil、空名称、重复任务名及 nil 函数。`SetLogHandler` 也必须在启动前调用。
- `Start(ctx) error` 先绑定监听，再向调度中心首次注册；两者成功才返回 nil。端口冲突直接返回错误，注册失败关闭监听并尝试摘除可能已接受的注册，返回前完成有界清理。
- **Start 的 ctx 是整个实例生命周期，不是只用于启动。** 不要传入一个成功后立即 cancel 的短上下文；该上下文取消会自动发起关闭。
- `Stop(ctx) error` 立即拒绝新调度、关闭监听并取消运行任务；停止心跳后再摘除注册，等待 HTTP、队列任务和回调收尾。未启动、重复和并发调用均安全。
- Stop 的调用方 ctx 只限制等待时间，取消等待不会放弃内部清理；后台仍使用本次 ShutdownTimeout 总预算。需要观察最终结果可继续 `Wait`。
- `Wait(ctx) error` 等待实际关闭，返回异步监听或关闭错误；取消 Wait 不会关闭服务。正常运行期间不会提前返回。

实例一旦开始启动就不能再次启动或追加注册，停止后需要新建实例。旧 `NewClient(ctx, Config)`、链式 Register、MustStart、无参数 Stop、SDK 旧启动范例已删除，不保留兼容入口。

调用方主动停止的简短范例：

```go
package examples

import (
    "context"
    "time"
    "github.com/K7mQ2vR9/base-pkg/xxlJob"
)

// Shutdown 限制当前调用的等待时间；内部关闭总预算由 Config.ShutdownTimeout 决定。
func Shutdown(client *xxlJob.Client) error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return client.Stop(ctx)
}
```

摘除失败会作为关闭错误返回；网络中断时调度中心还可能暂时保留旧记录，需依靠其过期判定或管理操作。本包保证本地心跳停止后不再周期注册，不保证远端取消请求尚未生效的瞬间状态。

## 任务、队列与取消

`HandlerFun` 保留现有 `func(context.Context, *xxl.RunReq) string` 签名，普通返回按成功回调，返回的字符串是结果内容。该签名没有 error 返回值；取消、超时及 panic 按失败回调，即使业务随后返回“成功”字符串也不会改为成功。

任务 ctx 与 HTTP `/run` 请求取消分离，接口响应后任务继续执行，但 ctx 会响应执行器停止、`/kill`、覆盖策略和 `ExecutorTimeout`。调度参数中的超时单位为秒，大于 0 时从实际开始执行计时，不包含串行排队；为 0 时不增加执行时限，仍响应服务关闭。业务 I/O 必须沿用该 ctx。回调返回后释放当前执行上下文。

| 阻塞策略 | 当前行为 |
|---|---|
| 空或 `SERIAL_EXECUTION` | 同 JobID 按接收顺序执行；不同 JobID 可并行 |
| `DISCARD_LATER` | 同 JobID 已运行或排队时拒绝新调度 |
| `COVER_EARLY` | 取消运行任务，取消并回调旧排队任务；等旧处理器退出后执行最新调度 |

Go 无法强制终止不响应 ctx 的代码。因此 kill/覆盖仅发出取消，旧任务退出前仍视为忙碌，不提前并发启动同 JobID 的新任务。关闭预算耗尽会释放监听、强制关闭 HTTP 并取消未完成回调，然后返回超时；忽略取消的业务代码仍须自行退出。

每次执行使用独立任务状态和参数副本，业务修改参数不会污染回调的原 LogID、LogDateTime 或其他调度。只在同一实例的未完成调度中按 LogID 识别完全相同的重复请求；完成后或进程重启后不提供持久化去重，业务须按稳定标识保证幂等。

排队和回调是内存状态，受 MaxTasks 统一限额，达到上限返回调度失败。正常关闭会为已接受的排队任务回调失败；进程崩溃或关闭总预算耗尽时不保证所有回调送达。

## 链路与回调

任务入口接受合法 `X-Request-ID`/W3C 链路，缺失时按平台规则生成独立的 32 位小写 Trace ID，不把启动上下文的 ID 当作所有任务共同的 ID。回调、业务 RPC 和 MQ 沿用该任务链路，不另建 Request ID 别名。

回调使用现有数组协议 `[{logId, logDateTim, handleCode, handleMsg}]`，成功代码 200，失败 500。字符串结果最多 1024 字节并保留 UTF-8 边界。任务取消后，失败回调使用独立网络预算并保留任务 Trace；RequestTimeout 约束每次 HTTP 请求，关闭总预算耗尽则取消剩余回调。

回调在独立执行流程中进行，不占用同 JobID 的业务串行队列，但计入 MaxTasks 和关闭等待。发送失败记录日志，本轮不增加回调持久化或自动重试队列；错误日志不包含访问令牌、完整任务参数或调度中心原始错误正文。

## 日志查询

`/log` 未配置提供器时，明确提示使用应用日志系统，不伪造历史业务日志。若需要在调度中心查看日志，可在 Start 前调用 `SetLogHandler`：

```go
package examples

import (
    "context"
    "github.com/K7mQ2vR9/base-pkg/xxlJob"
    xxl "github.com/xxl-job/xxl-job-executor-go"
)

// RegisterLogReader 将业务日志存储接入查询，read 须响应 ctx 并返回请求行范围的真实内容。
func RegisterLogReader(client *xxlJob.Client, read func(context.Context, int64, int) (string, int, bool, error)) error {
    return client.SetLogHandler(func(ctx context.Context, req *xxl.LogReq) (*xxl.LogResContent, error) {
        content, to, end, err := read(ctx, req.LogID, req.FromLineNum)
        if err != nil {
            return nil, err
        }
        return &xxl.LogResContent{
            FromLineNum: req.FromLineNum, ToLineNum: to,
            LogContent: content, IsEnd: end,
        }, nil
    })
}
```

本轮通过本地调度中心替身和真实本机 HTTP 监听验证，不连接实际调度中心；部署前仍需与目标版本及网络注册地址联调。
