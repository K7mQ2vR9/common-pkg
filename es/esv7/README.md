# Elasticsearch 7 使用说明

模块路径为 `github.com/K7mQ2vR9/base-pkg/es/esv7`，使用 Elasticsearch 7 SDK。配置、TLS、连接池、错误和模型均独立于 `es/esaws`，不共用类型或兼容入口。

## 配置与默认值

```yaml
Addresses:
  - "https://search.example.com:9200"
Username: "app"
Password: "${ELASTICSEARCH_PASSWORD}"
StartupTimeout: 5s
RequestTimeout: 10s
MaxRetries: 3
TLS:
  CAFile: ""
  InsecureSkipVerify: false
Transport:
  DialTimeout: 5s
  TLSHandshakeTimeout: 5s
  ResponseHeaderTimeout: 5s
  MaxIdleConns: 100
  MaxIdleConnsPerHost: 100
  MaxConnsPerHost: 100
  IdleConnTimeout: 90s
```

`Addresses` 必填，接受 HTTP/HTTPS URL，可包含路径前缀；账号密码放在独立字段中，不允许放入 URL，也不接受 query 和 fragment。两项凭证均为空时不发送 Basic Auth。配置解析和环境变量替换由应用负责，直接构造 Go Config 时不会自动替换 `${...}`。

| 配置 | 说明 |
|---|---|
| `StartupTimeout` | 默认 5s，集群 Info 探活总时限，包含 SDK 产品检查、重试及读取正文 |
| `RequestTimeout` | 默认 10s，每次模型操作的独立 I/O 预算，包含所有 SDK 重试和读取正文 |
| `MaxRetries` | `*int`，nil 默认 3；指向 0 关闭 SDK 重试；表示首次请求之外的次数 |
| `TLS.CAFile` | 默认空，使用系统信任证书；指定 PEM 文件时追加信任，无法读取或无有效证书则失败 |
| `TLS.InsecureSkipVerify` | 默认 false，校验证书链及主机名，最低 TLS 1.2；仅测试环境按需开启跳过验证 |
| `Transport` | 阶段时限及连接池默认值如上，`MaxIdleConnsPerHost` 不得超过 `MaxConnsPerHost` |

时长和连接池字段为 0 时使用默认值，负值非法，不用 0 表示无限等待或无限连接。父请求截止时间更早时优先采用父级时间。连接池是服务级共享资源，不随每个请求重新建立。默认沿用 SDK 的网络错误及 502/503/504 重试行为，不额外开启网络超时重试；请求的总时间预算不会随重试重新计算。重试不能证明前一次写入没有发生，写操作仍须使用稳定 ID 并按业务语义处理冲突。

## 完整接入范例

下面代码可保存为 `main.go` 编译运行。提供真实 `ELASTICSEARCH_URL`，按需设置 `ELASTICSEARCH_USERNAME`、`ELASTICSEARCH_PASSWORD`、`ELASTICSEARCH_CA_FILE`。范例执行查询，不创建或删除索引。

```go
package main

import (
    "context"
    "errors"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/K7mQ2vR9/base-pkg/es/esv7/core"
    "github.com/K7mQ2vR9/base-pkg/es/esv7/model"
)

type Record struct {
    ID     string `json:"-"`
    Name   string `json:"name"`
    Number int64  `json:"number"`
}

func (*Record) IndexName() string { return "platform_records" }
func (r *Record) GetId() string   { return r.ID }
func (r *Record) SetId(id string) { r.ID = id }

func main() {
    appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    client, err := core.NewEs(appCtx, &core.Config{
        Addresses: []string{os.Getenv("ELASTICSEARCH_URL")},
        Username: os.Getenv("ELASTICSEARCH_USERNAME"),
        Password: os.Getenv("ELASTICSEARCH_PASSWORD"),
        TLS: core.TLSConfig{CAFile: os.Getenv("ELASTICSEARCH_CA_FILE")},
    })
    if err != nil {
        log.Printf("initialize Elasticsearch 7: %v", err)
        return
    }
    defer client.Close()

    // HTTP/RPC 业务中使用当前请求 ctx；共享 client，每个请求创建自己的模型实例。
    records := model.NewEsModel(appCtx, client)
    record := &Record{}
    if err := records.FindOne("123", record); err != nil {
        if errors.Is(err, core.ErrNotFound) {
            log.Print("record not found")
        } else {
            log.Printf("get record: %v", err)
            return
        }
    } else {
        log.Printf("record ID=%s name=%s", record.ID, record.Name)
    }

    var result []Record
    total, err := records.Search(&Record{}, &result, map[string]any{
        "size": 20,
        "query": map[string]any{"match_all": map[string]any{}},
    })
    if err != nil {
        log.Printf("search records: %v", err)
        return
    }
    log.Printf("total=%d returned=%d", total, len(result))
}
```

`NewEs(ctx, conf)` 返回初始化错误，`MustNewEs(ctx, conf)` 在失败时 panic。初始化 ctx 取消不关闭已经成功创建的客户端。`Close()` 可并发或重复调用，拒绝后续网络请求并释放空闲连接；它不主动强制取消在途请求，服务先停止接收新任务并等待业务退出，再关闭客户端。

## 模型操作

| 接口 | 行为 |
|---|---|
| `IndexAuto(index) error` | 创建索引，索引已存在时返回服务端错误，不删除或重建 |
| `InsertSchema(data) error` | 使用 create 语义写入，实现 IndexTable/Schema；空 ID 自动生成 UUIDv7，已有 ID 冲突返回 409 |
| `FindOne(id, target) error` | 解码到目标指针，并向实现 Schema 的目标设置文档 ID |
| `UpdateSchema(data) error` | 使用 `{"doc": data}` 正文执行局部更新，要求非空 ID |
| `Delete(id, target) error` | 按目标的 IndexName 删除指定文档 |
| `Search(indexTarget, target, query) (int64, error)` | 结果写入目标切片指针，只返回总数和错误，空命中为空切片 |

所有操作沿用模型的 `Ctx`，不使用 `context.Background()` 替代业务上下文；每次操作派生新的 RequestTimeout，父级取消/截止时间更早时优先。JSON 编解码是同步计算，不会被强制中断。模型实例不跨请求复用或并发修改 Ctx。

`Search` 设置 `track_total_hits=true` 获取准确总数，大数据量查询会增加统计成本；通过 `size` 和分页控制单次读取量。解析使用 JSON 原始片段直接解码到业务类型，避免先转换 float64 导致大 int64 丢失精度。旧的已关闭原始响应返回值已经删除。

空 create ID 使用 `commonTool.NewUUidV7()` 生成 36 字符 UUIDv7；生成失败保留错误链并直接返回，不发送写入请求。业务已指定 ID 时原样使用。自动生成的 create ID 不写回调用方对象，同一次 SDK 请求重试复用该 ID，再次调用 `InsertSchema` 会重新生成；需要稳定重发标识时业务提前设置 ID。生成值不是业务自增序号，同一毫秒内不保证递增。

局部更新会提交对象编码后的字段，是否包含零值由业务 JSON 标签决定，不提供自动字段差异计算。

## 错误与响应体

`errors.Is(err, core.ErrNotFound)` 识别 HTTP 404 或 Get 的 `found=false`；搜索没有命中但索引存在时返回零条，不返回该错误。`errors.As(err, &responseError)` 获取 `*core.ResponseError` 的 StatusCode 和最多 128 字节的 ErrorType。错误响应最多读取 4KiB 提取类型，不直接输出原始正文或 error.reason。

`ErrInvalidResponse` 表示缺失响应、正文或必要查询字段；JSON 语法/类型及网络错误保留原始错误链，使用 errors.Is/As 检查。`ErrClosed` 表示客户端已关闭。查询和写入的正常、404、错误状态、读取失败、解码失败分支均释放响应体，错误状态不会无限读取剩余正文。

## 直接使用底层 SDK

高级用法可以使用嵌入 SDK，但调用方负责操作上下文和响应体。使用 `OperationContext` 获得相同的模型默认预算；响应处理完成后再取消上下文，不能拿到响应头就提前取消。

```go
package examples

import (
    "context"
    "github.com/K7mQ2vR9/base-pkg/es/esv7/core"
)

// ClusterInfo 使用 SDK，同时复用公共包的超时和响应体清理。
func ClusterInfo(ctx context.Context, client *core.Es) error {
    operation, cancel, err := client.OperationContext(ctx)
    if err != nil {
        return err
    }
    defer cancel()
    response, err := client.Info(client.Info.WithContext(operation))
    if err != nil {
        if response != nil && response.Body != nil {
            _ = response.Body.Close()
        }
        return err
    }
    return core.CheckResponse(operation, response)
}
```

直接调用 SDK 而不传上下文时，RequestTimeout 不会自动生效；TCP/TLS/响应头阶段时限仍生效，但不能替代整个操作和读取正文的总预算。`ReadResponse`、`CheckResponse`、`ParseGetResponse` 和 `ParseSearchResponse` 接管响应体后均会关闭，调用方不要再次读取或关闭同一正文。
