# 时间工具使用说明

时间工具集中在 `commonTool/time_tool.go`，由 `TimeTool` 管理。`tool.go` 中的旧时间接口已删除，不保留兼容包装。

## 时区规则

**未指定时区时，一律按 UTC（零时区）计算和格式化。**

- `NewTimeTool("")`、`NewTimeTool("UTC")`、`TimeTool{}` 和 nil 实例均使用 UTC。
- 显式指定时区时使用 IANA 名称，例如 `Asia/Shanghai`、`Asia/Ho_Chi_Minh`、`Asia/Kolkata`。
- 非法名称返回错误，不自动回退；`Local` 被拒绝，避免计算依赖服务器时区。构造失败必须先处理错误。
- 标准库 `time/tzdata` 已内置，系统缺少 zoneinfo 时仍可加载时区；时区规则版本取决于实际使用的系统数据或 Go 内置数据。
- 每个实例保存只读时区，可并发复用，不修改全局 `time.Local`。方法显式接收基准时间，按该实际时刻在工具时区的日期计算。

```go
package main

import (
    "fmt"
    "log"
    "time"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

func main() {
    // 未配置时区，使用 UTC。实际服务可将统一配置中的 IANA 名称传入。
    tool, err := commonTool.NewTimeTool("")
    if err != nil {
        log.Fatal(err)
    }
    base := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
    fmt.Println(tool.DayStart(base).Format(time.RFC3339))
    fmt.Println(tool.LastMonthStart(base).Format(time.RFC3339))
    fmt.Println(tool.FormatUnix(0))
}
```

输出：

```text
2026-03-31T00:00:00Z
2026-02-01T00:00:00Z
1970-01-01 00:00:00
```

## 接口与单位

| 接口 | 行为 |
|---|---|
| `NewTimeTool(timezone string) (*TimeTool, error)` | 加载显式时区，空字符串默认 UTC |
| `DayStart(base time.Time) time.Time` | 基准时刻所属日的起点 |
| `YesterdayStart(base time.Time) time.Time` | 前一个日历日的起点 |
| `MonthStart(base time.Time) time.Time` | 本月起点 |
| `LastMonthStart(base time.Time) time.Time` | 上月起点，先定位月初，避免月底减月归一化错误 |
| `Date(base time.Time) string` | 指定时区的 `YYYY-MM-DD` 日期 |
| `FormatUnix(seconds int64) string` | 将秒级 Unix 时间戳格式化为 `YYYY-MM-DD HH:MM:SS` |
| `Parse(value string) (time.Time, error)` | 严格按工具时区解析秒级日期时间 |
| `DiffSeconds(start, end string) (int64, error)` | 实际经过的 `end-start` 秒数，可为负数 |

日历边界返回携带工具时区的 `time.Time`；使用 `.Unix()` 取秒级时间戳、`.UnixMilli()` 取毫秒。Unix 时间戳标识实际时刻，时区决定其日期和显示形式。`FormatUnix(0)` 格式化 Unix 起始时刻，不返回空字符串。

需要当前时间直接使用 `time.Now()`，当前秒级时间戳使用 `time.Now().Unix()`。同一次业务计算建议获取一次 `base` 后复用，避免跨日或跨月时多次读取时钟得到不同基准。

## 日历边界与时区切换

昨日按日历回退，不固定减 24 小时。纽约 2024-03-10 对应 23 小时日，2024-11-03 对应 25 小时日；月初也按工具时区确定。

零点不存在时返回该日期首个有效时刻，例如圣保罗 2018-11-04 从 01:00 开始；零点重复时选择较早的实际时刻。若时区变更跳过整个日期，则该日期边界为跳变后的首个有效时刻，该日对应空时间区间，例如阿皮亚跳过 2011-12-30。业务统计使用相邻日历边界组成半开区间 `[开始, 下一边界)`，不要假定每天都包含 86400 秒。

## 解析与时间差

`DateTimeLayout` 为 `2006-01-02 15:04:05`，`DateLayout` 为 `2006-01-02`。解析不接受小数秒、时区后缀、额外空白或非规范宽度。非法日历日期和跳变中不存在的本地时刻返回错误；底层 `time.ParseError` 保留 `errors.As` 链。

时钟回拨可能使同一本地日期时间对应两个实际时刻，返回可用 `errors.Is` 判断的 `ErrAmbiguousLocalTime`。需要表示这类实际时刻时，跨服务协议使用带偏移的 RFC3339，并通过标准库 `time.Parse` 解析后传入日历方法；不要丢掉偏移后再按无时区字符串解析。

```go
package examples

import (
    "errors"
    "time"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

// ElapsedSeconds 演示显式时区、秒级差值和可识别的歧义错误。
func ElapsedSeconds(zone, start, end string) (int64, error) {
    tool, err := commonTool.NewTimeTool(zone)
    if err != nil {
        return 0, err
    }
    seconds, err := tool.DiffSeconds(start, end)
    if errors.Is(err, commonTool.ErrAmbiguousLocalTime) {
        return 0, err
    }
    var parseError *time.ParseError
    if errors.As(err, &parseError) {
        return 0, err
    }
    return seconds, err
}
```

时间差单位明确为秒，按解析后的实际 Unix 秒计算，不返回纳秒，也不依赖 `time.Duration` 表达跨数百年的差值。任一输入解析失败时返回 0 和错误，不把非法时间当作零值继续计算。

## 旧接口迁移

| 已删除接口 | 新用法 |
|---|---|
| `TimeToString` | `tool.FormatUnix(seconds)`；0 现在是合法 Unix 起点 |
| `DiffTimeUnix` | `tool.DiffSeconds(start, end)`，处理错误，单位为秒 |
| `GetNowTime` | `time.Now().Unix()` |
| `GetTodayZeroTimeInt` | `tool.DayStart(base).Unix()` |
| `GetYesterdayZeroTimeInt` | `tool.YesterdayStart(base).Unix()` |
| `GetMonthZeroTimeInt` | `tool.MonthStart(base).Unix()` |
| `GetLastMonthZeroTimeInt` | `tool.LastMonthStart(base).Unix()` |
| `GetXZeroTodayTimeInt` | 使用明确 IANA 时区构造工具，再调用 `tool.Date(base)` |

API/RPC 按业务契约选择时区并传给工具；持久化实际时刻和跨服务时间优先使用约定的 Unix 单位或带偏移时间。工具不自动读取请求头、不修改服务全局时区。本轮仅提供公共工具与接入约定，不新增业务 API。
