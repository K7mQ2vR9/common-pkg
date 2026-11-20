package commonTool

import (
	"errors"

	"github.com/shopspring/decimal"
)

const (
	// maxDecimalTextLength 为输入的最大字节数，普通十进制只含 ASCII，符号和小数点也计入 256 字符限制。
	maxDecimalTextLength = 256
	// maxDecimalPlaces 为输出允许的最大小数位数，单位为十进制小数位。
	maxDecimalPlaces = 18
)

var (
	// ErrInvalidDecimal 表示输入为空、超长或不符合普通十进制格式，错误文本不包含输入值。
	ErrInvalidDecimal = errors.New("decimal value must be plain decimal text of at most 256 characters")
	// ErrInvalidDecimalPlaces 表示小数位数不在 0～18 内，不进行静默截断或默认修正。
	ErrInvalidDecimalPlaces = errors.New("decimal places must be between 0 and 18")
)

// FloorDecimal 将十进制文本向负无穷取整，并返回恰好 places 位小数的字符串，全程不转换成 float64。
// places 必须在 0～18 内；value 最多 256 个 ASCII 字符，格式为可选正负号、至少一位整数及可选的点和至少一位小数。
// 不接受科学计数法、空白、NaN 或 Inf；支持前导零，输出移除多余前导零和正号，零统一为无负号形式。
// 例如 FloorDecimal("1.239", 2) 返回 "1.23"，负数 "-1.239" 返回 "-1.24"；非法输入返回空串及可用 errors.Is 判断的错误。
func FloorDecimal(value string, places int) (string, error) {
	// 在转换为 int32 或解析前校验精度和文本边界，限制指数及整数计算空间。
	if places < 0 || places > maxDecimalPlaces {
		return "", ErrInvalidDecimalPlaces
	}
	if !validDecimalText(value) {
		return "", ErrInvalidDecimal
	}
	number, err := decimal.NewFromString(value)
	if err != nil {
		// 底层解析错误可能包含原始输入，统一返回不含输入值的分类错误。
		return "", ErrInvalidDecimal
	}
	// 直接按位数向负无穷取整，避免先放大再 Div 受库级 DivisionPrecision 影响。
	precision := int32(places)
	return number.RoundFloor(precision).StringFixed(precision), nil
}

func validDecimalText(value string) bool {
	if len(value) == 0 || len(value) > maxDecimalTextLength {
		return false
	}
	index := 0
	if value[0] == '+' || value[0] == '-' {
		index++
	}
	integerStart := index
	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}
	if index == integerStart {
		return false
	}
	if index == len(value) {
		return true
	}
	if value[index] != '.' {
		return false
	}
	index++
	if index == len(value) {
		return false
	}
	for ; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}
