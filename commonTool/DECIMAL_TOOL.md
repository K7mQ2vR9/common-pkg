# 十进制精度工具使用说明

通用十进制工具独立放在 `commonTool/decimal_tool.go`，入口为：

```text
func FloorDecimal(value string, places int) (string, error)
```

旧 `FloorAmountNum` 已删除，不提供浮点兼容包装；当前工具全程使用十进制字符串和十进制运算，不转换为 float64。

## 输入与精度

| 参数 | 规则 |
|---|---|
| `value` | 最多 256 个 ASCII 字符，正负号和小数点计入长度 |
| 文本格式 | `[+-]?[0-9]+(\.[0-9]+)?`，整数部分至少一位，出现小数点时其后至少一位 |
| `places` | 0～18，必须显式传入；0 返回整数文本 |
| 返回值 | 向负无穷取整后的固定小数位字符串，失败返回空字符串和错误 |

支持前导零和可选正负号；输出移除多余前导零与正号，数值为零时移除负号。拒绝空字符串、空白、Unicode 数字、科学计数法、NaN/Inf、千位分隔符、十六进制和不完整小数，例如 `.5`、`1.`。这类小数须使用 `0.5`、`1.0`。

参数先验证再解析和转换精度类型，避免超大位数转换溢出及科学计数法展开。输入校验不自动去除空白或修正非法小数位数。

## 取整与显示

向负无穷取整规则：结果小于或等于原值，原值与结果之差严格小于 `10^-places`。

| 输入 | 小数位 | 输出 |
|---|---:|---|
| `1.239` | 2 | `1.23` |
| `-1.239` | 2 | `-1.24` |
| `1.2` | 2 | `1.20` |
| `-1.000001` | 0 | `-2` |
| `-0.00001` | 4 | `-0.0001` |
| `-0.0000` | 2 | `0.00` |
| `9007199254740993.1234567890123456789` | 18 | `9007199254740993.123456789012345678` |

实现直接使用有界精度的 RoundFloor 和固定小数位输出，不依赖先放大再除法，也不修改 decimal 包的全局精度配置。结果为字符串，能够保留尾部零以及超过浮点精确范围的有效数字。最大长度的整数补齐 18 位小数后，输出可超过 256 字符；256 限制用于输入。

## 完整调用范例

```go
package main

import (
    "fmt"
    "log"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

func main() {
    examples := []struct {
        value string
        places int
    }{
        {"1.239", 2},
        {"-1.239", 2},
        {"1.2", 2},
        {"9007199254740993.1234567890123456789", 18},
    }
    for _, example := range examples {
        value, err := commonTool.FloorDecimal(example.value, example.places)
        if err != nil {
            log.Printf("normalize decimal: %v", err)
            return
        }
        fmt.Println(value)
    }
}
```

输出：

```text
1.23
-1.24
1.20
9007199254740993.123456789012345678
```

## JSON 与 API/RPC 接入

需要精确十进制语义时，接口从入口就使用字符串传递，并由 API/RPC 的业务契约约定小数位数。例如 JSON 返回 `{"value":"1.20"}`，接收方继续保留字符串或使用十进制类型。

```go
package examples

import (
    "encoding/json"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

// DecimalResponse 通过字符串字段保留固定小数位和精确数值。
type DecimalResponse struct {
    Value string `json:"value"`
}

// BuildDecimalResponse 演示调用方指定精度并序列化返回值，不在业务中重复做浮点转换。
func BuildDecimalResponse(input string, places int) ([]byte, error) {
    value, err := commonTool.FloorDecimal(input, places)
    if err != nil {
        return nil, err
    }
    return json.Marshal(DecimalResponse{Value: value})
}
```

若上游数值已经经过 float64 并丢失精度，后续改成字符串无法恢复丢失的数字；需要在源数据、请求 DTO 和存储契约中统一精确表示。本轮只增加通用基础工具，不修改业务返回结构或数据库模型。

## 错误与迁移

| 错误判断 | 含义 |
|---|---|
| `errors.Is(err, commonTool.ErrInvalidDecimal)` | 文本为空、过长或格式非法 |
| `errors.Is(err, commonTool.ErrInvalidDecimalPlaces)` | 小数位数不在 0～18 |

错误文本不回显输入，失败不返回可用数值；先处理错误再使用返回字符串。旧调用须改为字符串参数并处理 error，负小数位现在返回错误，不再静默改成 0。

函数无共享可变状态，可并发调用。本轮使用 Python Decimal 独立参考、精确有理数性质验证和并发测试，不引入新的第三方依赖；保留当前已使用的 shopspring/decimal。
