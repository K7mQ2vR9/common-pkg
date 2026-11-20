package commonTool

import (
	"crypto"
	"encoding/hex"

	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
)

func Md5(str string) string {
	h := crypto.MD5.New()
	_, _ = h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

// 浮点数保留多少位
func FloorAmountNum(amount float64, len int) float64 {
	if len < 0 {
		len = 0
	}
	d := decimal.NewFromFloat(amount)
	factor := decimal.NewFromInt(1).Shift(int32(len)) // 10^len
	d = d.Mul(factor).Floor().Div(factor)
	// 固定保留 len 位（避免 1.2 变成 1.199999）
	return cast.ToFloat64(d.StringFixed(int32(len)))
}
