package appMiddleware

import (
	"net/http"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result"
	"go.opentelemetry.io/otel/propagation"
)

// EnsureRequestTrace 在进入业务中间件前沿用已有链路或初始化请求的链路上下文。
func EnsureRequestTrace(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if headInfo.GetTrace(r.Context()) == "" {
			ctx := (propagation.TraceContext{}).Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, err := headInfo.EnsureTrace(ctx)
			if err != nil {
				result.HttpErrorResult(ctx, w, err)
				return
			}
			r = r.WithContext(ctx)
		}
		next(w, r)
	}
}
