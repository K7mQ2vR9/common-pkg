# OpenSearch 使用说明

模块路径为 `github.com/K7mQ2vR9/base-pkg/es/esaws`，使用 OpenSearch SDK。配置、TLS、连接池、错误和模型均独立于 `es/esv7`，不共用类型或兼容入口。

## 配置与默认值

```yaml
Addresses:
  - "https://search.example.com:9200"
Username: "app"
Password: "${OPENSEARCH_PASSWORD}"
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
| `StartupTimeout` | 默认 5s，集群 Info 探活总时限，包含受控 Info 探活、重试及读取正文 |
| `RequestTimeout` | 默认 10s，每次模型操作的独立 I/O 预算，包含所有 SDK 重试和读取正文 |
| `MaxRetries` | `*int`，nil 默认 3；指向 0 关闭 SDK 重试；表示首次请求之外的次数 |
| `TLS.CAFile` | 默认空，使用系统信任证书；指定 PEM 文件时追加信任，无法读取或无有效证书则失败 |
| `TLS.InsecureSkipVerify` | 默认 false，校验证书链及主机名，最低 TLS 1.2；仅测试环境按需开启跳过验证 |
| `Transport` | 阶段时限及连接池默认值如上，`MaxIdleConnsPerHost` 不得超过 `MaxConnsPerHost` |

时长和连接池字段为 0 时使用默认值，负值非法，不用 0 表示无限等待或无限连接。父请求截止时间更早时优先采用父级时间。连接池是服务级共享资源，不随每个请求重新建立。默认沿用 SDK 的网络错误及 502/503/504 重试触发条件，不额外开启网络超时重试；删除了旧的硬编码随机指数退避，因为当前 SDK 的等待使用不可取消的 time.Sleep；请求的总时间预算不会随重试重新计算。重试不能证明前一次写入没有发生，写操作仍须使用稳定 ID 并按业务语义处理冲突。

## 完整接入范例

下面代码可保存为 `main.go` 编译运行。提供真实 `OPENSEARCH_URL`，按需设置 `OPENSEARCH_USERNAME`、`OPENSEARCH_PASSWORD`、`OPENSEARCH_CA_FILE`。范例执行查询，不创建或删除索引。

```go
package main

import (
    "context"
    "encoding/json"
    "errors"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/K7mQ2vR9/base-pkg/es/esaws/core"
    "github.com/K7mQ2vR9/base-pkg/es/esaws/model"
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
        Addresses: []string{os.Getenv("OPENSEARCH_URL")},
        Username: os.Getenv("OPENSEARCH_USERNAME"),
        Password: os.Getenv("OPENSEARCH_PASSWORD"),
        TLS: core.TLSConfig{CAFile: os.Getenv("OPENSEARCH_CA_FILE")},
    })
    if err != nil {
        log.Printf("initialize OpenSearch: %v", err)
        return
    }
    defer client.Close()

    // HTTP/RPC 业务中使用当前请求 ctx；共享 client，每个请求创建自己的模型实例。
    records := model.NewEsModel(appCtx, client, "10003002")
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
    total, aggregations, err := records.Search(&Record{}, &result, map[string]any{
        "size": 20,
        "query": map[string]any{"match_all": map[string]any{}},
        "aggs": map[string]any{
            "by_name": map[string]any{"terms": map[string]any{"field": "name.keyword"}},
        },
    })
    if err != nil {
        log.Printf("search records: %v", err)
        return
    }
    log.Printf("total=%d returned=%d", total, len(result))
    if raw, ok := aggregations["by_name"]; ok {
        var group struct {
            Buckets []struct {
                Key string `json:"key"`
                Count int64 `json:"doc_count"`
            } `json:"buckets"`
        }
        if err := json.Unmarshal(raw, &group); err != nil {
            log.Printf("decode aggregation: %v", err)
            return
        }
        log.Printf("aggregation buckets=%d", len(group.Buckets))
    }
}
```

`NewEs(ctx, conf)` 返回初始化错误，`MustNewEs(ctx, conf)` 在失败时 panic。初始化 ctx 取消不关闭已经成功创建的客户端。`Close()` 可并发或重复调用，拒绝后续网络请求并释放空闲连接；它不主动强制取消在途请求，服务先停止接收新任务并等待业务退出，再关闭客户端。

当前 SDK 的内部产品探活使用 Background。本包关闭该隐式探活，改为直接调用携带 StartupTimeout 上下文的 Info 检查连接、认证、HTTP 状态及 JSON 正文，不依赖 Elasticsearch 产品响应头。SDK 重试会重复追加节点路径前缀，本包按操作保存原始路径并在每次网络调用前恢复，节点前缀及文档 ID 的转义在重试中保持。

## 模型操作

| 接口 | 行为 |
|---|---|
| `IndexAuto(index) error` | 创建索引，索引已存在时返回服务端错误，不删除或重建 |
| `InsertSchema(data) error` | 使用 create 语义写入，实现 IndexTable/Schema；空 ID 自动生成 UUIDv7，已有 ID 冲突返回 409 |
| `FindOne(id, target) error` | 解码到目标指针，并向实现 Schema 的目标设置文档 ID |
| `UpdateSchema(data) error` | 使用 `{"doc": data}` 正文执行局部更新，要求非空 ID |
| `IndexSchema(data) error` | 按稳定 ID 全量覆盖，不存在时创建，保留 refresh=true 行为 |
| `Delete(id, target) error` | 按目标的 IndexName 删除指定文档 |
| `Search(indexTarget, target, query) (int64, core.Aggregations, error)` | 结果写入目标切片指针，返回总数、聚合与错误，空命中为空切片，缺少聚合时返回空 map |

所有操作沿用模型的 `Ctx`，不使用 `context.Background()` 替代业务上下文；每次操作派生新的 RequestTimeout，父级取消/截止时间更早时优先。JSON 编解码是同步计算，不会被强制中断。模型实例不跨请求复用或并发修改 Ctx。

`Search` 设置 `track_total_hits=true` 获取准确总数，大数据量查询会增加统计成本；通过 `size` 和分页控制单次读取量。解析使用 JSON 原始片段直接解码到业务类型，避免先转换 float64 导致大 int64 丢失精度。旧的已关闭原始响应返回值已经删除。

空 create ID 使用 `commonTool.NewUUidV7()` 生成 36 字符 UUIDv7；生成失败保留错误链并直接返回，不发送写入请求。业务已指定 ID 时原样使用。自动生成的 create ID 不写回调用方对象，同一次 SDK 请求重试复用该 ID，再次调用 `InsertSchema` 会重新生成；需要稳定重发标识时业务提前设置 ID。生成值不是业务自增序号，同一毫秒内不保证递增。

局部更新会提交对象编码后的字段，是否包含零值由业务 JSON 标签决定，不提供自动字段差异计算。

`BusinessCode` 非空时，模型的文档和搜索操作使用 `IndexName() + "_" + BusinessCode`；为空时使用基础索引名。上例实际查询 `platform_records_10003002`。`IndexAuto(index)` 使用调用方传入的完整索引名，不自动追加后缀，创建业务索引时须显式传入最终名称。

## 聚合结果

`core.Aggregations` 是本套独立定义的 `map[string]json.RawMessage`，不是 Elasticsearch 类型别名。键为聚合名称，值保留服务端原始 JSON，包括嵌套桶、未知扩展字段和 null。使用 `json.Unmarshal(aggregations[name], &target)` 按业务结构解码，计数等大整数直接解码到 int64；需要通用数值时使用 json.Number，避免先解码为 map[string]any 后转换 float64。客户端保留的是服务端实际返回的数值，不能恢复服务端计算时已经丢失的精度。

聚合查询也传入切片指针接收命中；只需要聚合时设置 `size: 0`，目标获得空切片，聚合仍正常返回。模型固定请求准确 total，不使用 track_total_hits=false。

## 全量覆盖写入

```go
package examples

import (
    "context"
    "github.com/K7mQ2vR9/base-pkg/es/esaws/core"
    "github.com/K7mQ2vR9/base-pkg/es/esaws/model"
)

type Snapshot struct {
    ID string `json:"-"`
    Name string `json:"name"`
}

func (*Snapshot) IndexName() string { return "snapshots" }
func (s *Snapshot) GetId() string { return s.ID }
func (s *Snapshot) SetId(id string) { s.ID = id }

// SaveSnapshot 覆盖完整文档，稳定 ID 由业务提供；缺少的旧字段会被移除。
func SaveSnapshot(ctx context.Context, client *core.Es, businessCode, id, name string) error {
    snapshots := model.NewEsModel(ctx, client, businessCode)
    return snapshots.IndexSchema(&Snapshot{ID: id, Name: name})
}
```

`IndexSchema` 要求非空 ID，不自动生成，`refresh=true` 保留原先写后可见行为并增加写入成本；局部修改使用 `UpdateSchema`。需要只创建而不覆盖时使用 `InsertSchema`，处理 409 冲突。以上写入仍须考虑重试结果不确定。

## 错误与响应体

`errors.Is(err, core.ErrNotFound)` 识别 HTTP 404 或 Get 的 `found=false`；搜索没有命中但索引存在时返回零条，不返回该错误。`errors.As(err, &responseError)` 获取 `*core.ResponseError` 的 StatusCode 和最多 128 字节的 ErrorType。错误响应最多读取 4KiB 提取类型，不直接输出原始正文或 error.reason。

`ErrInvalidResponse` 表示缺失响应、正文或必要查询字段；JSON 语法/类型及网络错误保留原始错误链，使用 errors.Is/As 检查。`ErrClosed` 表示客户端已关闭。查询和写入的正常、404、错误状态、读取失败、解码失败分支均释放响应体，错误状态不会无限读取剩余正文。

## 直接使用底层 SDK

高级用法可以使用嵌入 SDK，但调用方负责操作上下文和响应体。使用 `OperationContext` 获得相同的模型默认预算；响应处理完成后再取消上下文，不能拿到响应头就提前取消。

```go
package examples

import (
    "context"
    "github.com/K7mQ2vR9/base-pkg/es/esaws/core"
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


## 接口替换

旧 InitESClient 和不带 ctx 的 MustNewEs 已删除，统一使用 NewEs/MustNewEs(ctx, conf)。旧 EsNotFound/Es500Err 改为本套 ErrNotFound 和 ResponseError，不保留别名；HTTP 404 不再当作查询成功。Search 删除原始响应返回值，聚合改为 core.Aggregations；不再导入 Elasticsearch v7/v8 响应和聚合类型。根包声明为 esaws，IndexTable/Schema 自行定义。目录名 esaws 继续保留，本套不引用 es/esv7。
