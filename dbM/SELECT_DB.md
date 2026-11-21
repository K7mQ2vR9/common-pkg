# MySQL 多库管理器

`SelectDb` 在服务启动时创建并持有多个 MySQL 连接池，业务通过 `DbKey` 选择数据库。RPC 数据模型服务负责数据库访问，API 调用 RPC 组织业务。

## 初始化与请求上下文

公开函数签名保持不变：

```text
NewSelectDb(ctx context.Context, config []SelectDbConfig) (*SelectDb, error)
(*SelectDb).GetDb(ctx context.Context, dbKey string) (*gorm.DB, error)
(*SelectDb).Close() error
```

`NewSelectDb` 先校验全部配置，再依次建立连接。空配置合法；空白或重复的 `DbKey`、无效 MySQL 配置会在建连前拒绝。某个库初始化失败时，尝试关闭全部已创建的连接池，返回的错误同时保留初始化错误和关闭错误，可用 `errors.Is`/`errors.As` 判断。

启动上下文只控制初始化，成功后取消它不会关闭管理器。每个库沿用 [MySQLConfig](mysqlConfig.go) 的配置和默认值，启动超时默认 5s，父级更早的截止时间优先；本轮没有新增配置项。

`GetDb` 使用 `WithContext(ctx)` 返回独立 GORM 会话，保留请求的值、取消信号及截止时间，查询和事务继续继承该上下文，不修改共享根会话。请求需要超时时，由业务派生带时限的上下文；管理器不额外设置统一查询超时。

## 状态与错误

| 状态或输入 | `GetDb` 结果 |
|---|---|
| `ctx == nil` | 返回上下文必填错误 |
| 请求已取消或超时 | 返回 `ctx.Err()`，优先于管理器状态校验 |
| nil 或零值管理器 | `ErrSelectDbUninitialized` |
| 已初始化，指定 key 不存在（含空配置） | 包装 `ErrDbNotConfigured` |
| 内部连接条目为空 | 包装 `ErrSelectDbUninitialized` |
| 已初始化且开始关闭或关闭完成 | `ErrSelectDbClosed` |
| 正常且 key 存在 | 返回绑定请求上下文的会话 |

使用 `errors.Is(err, dbM.ErrSelectDbClosed)` 等分类处理，不依赖错误文本。零值管理器关闭后仍属于未初始化；空配置构造的管理器关闭后属于已关闭。

## 关闭与并发

- 管理器供服务共享，使用后不可按值复制。
- `Close` 支持 nil、零值、重复和并发调用。关闭过程只执行一次，并发调用等待同一结果；后续调用返回第一次完整关闭的结果，包括错误。
- 开始关闭时先标记状态，后续 `GetDb` 立即返回关闭错误，不等待慢连接池回收。与关闭重叠且已通过状态检查的取库调用仍可能取得会话。
- 按 `DbKey` 排序尝试关闭全部连接池，部分失败不会跳过剩余库；通过 `errors.Join` 汇总错误。重复关闭不重试第一次失败的连接池。
- 管理器不强制取消在途查询，也不撤销已返回的会话；这些会话遵循 `database/sql` 的关闭行为。应用退出应先停止新业务，并按业务要求等待在途工作收尾，再关闭管理器。
- 不要按请求关闭管理器或调用会话底层的 `sql.DB.Close()`。直接关闭底层池不会同步管理器状态，也不应把 `GetDb` 成功视为数据库实时探活。

## 启动示例

以下示例从环境变量读取 DSN，启动时初始化一次。实际服务应在停止接收请求、处理在途工作后执行关闭。

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/K7mQ2vR9/base-pkg/dbM"
)

func main() {
    startupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    manager, err := dbM.NewSelectDb(startupCtx, []dbM.SelectDbConfig{
        {DbKey: "primary", MySQL: dbM.MySQLConfig{DataSource: os.Getenv("MYSQL_DSN")}},
    })
    cancel()
    if err != nil {
        log.Print("MySQL 管理器初始化失败")
        return
    }
    defer func() {
        if err := manager.Close(); err != nil {
            log.Print("MySQL 管理器关闭失败")
        }
    }()

    // 实际服务在这里注入 manager，并等待服务退出；请求沿用各自的 ctx。
    requestCtx, release := context.WithTimeout(context.Background(), time.Second)
    defer release()
    session, err := manager.GetDb(requestCtx, "primary")
    if err != nil {
        log.Print("获取数据库会话失败")
        return
    }
    var value int64
    if err := session.Raw("SELECT 1").Scan(&value).Error; err != nil {
        log.Print("数据库查询失败")
    }
}
```

## RPC 模型方法示例

将服务级管理器注入模型，请求使用当前 RPC 的上下文，继续保留链路和调用预算。下面按业务已存在的 `records` 表查询；实际模型自行定义字段和错误映射。

```go
package model

import (
    "context"

    "github.com/K7mQ2vR9/base-pkg/dbM"
)

// Record 展示查询结果字段，实际表结构由业务管理。
type Record struct {
    ID int64 `gorm:"column:id"`
}

// LoadRecord 使用服务级管理器，查询继承调用方上下文。
func LoadRecord(ctx context.Context, manager *dbM.SelectDb, id int64) (Record, error) {
    db, err := manager.GetDb(ctx, "primary")
    if err != nil {
        return Record{}, err
    }
    var record Record
    err = db.Table("records").Where("id = ?", id).Take(&record).Error
    return record, err
}
```

本轮使用内存 SQL 驱动和真实 GORM 会话验证生命周期；初始化回收通过临时连接函数替身验证。没有连接实际 MySQL 服务，认证、网络和部署退出流程仍需在应用环境联调。
