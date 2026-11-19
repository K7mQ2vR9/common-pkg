package appMiddleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result"
	"github.com/zeromicro/go-zero/rest/httpx"
	"go.opentelemetry.io/otel/propagation"
)

// EnsureRequestTrace 在创建服务端 Span 之前初始化统一 ID，Request ID 与 Trace ID 使用同一个值。
// 沿用合法 X-Request-ID 或 traceparent，全部缺失时使用加密随机数生成 32 位小写十六进制 ID。
// 空值、重复值、非法格式或 ID 冲突返回 HTTP 400；随机源失败返回 500，不进入业务处理。
// go-zero 服务通过 app.NewServer 或 app.WithRequestTraceRouter 在路由外层接入。
func EnsureRequestTrace(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, err := headInfo.ExtractRequestTrace(r.Context(), r.Header)
		if errors.Is(err, headInfo.ErrTraceRequired) {
			ctx, err = headInfo.EnsureTrace(r.Context())
		} else if err == nil {
			ctx, err = headInfo.EnsureTrace(ctx)
		}
		if err != nil {
			code, message := http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)
			if errors.Is(err, headInfo.ErrTraceInvalid) || errors.Is(err, headInfo.ErrTraceConflict) {
				code, message = http.StatusBadRequest, err.Error()
			}
			// 非法输入不会被转发；错误响应使用已有链路或新建诊断 ID，便于排查入口校验失败。
			ctx, _ = headInfo.EnsureTrace(r.Context())
			setCORSHeaders(w.Header(), r)
			if id := headInfo.GetTrace(ctx); id != "" {
				w.Header().Set(headInfo.RequestIDHeader, id)
			}
			httpx.WriteJsonCtx(ctx, w, code, result.Error(code, message, headInfo.GetTrace(ctx)))
			return
		}
		id := headInfo.GetTrace(ctx)
		// 克隆请求头，写入统一 ID 及 W3C 父上下文，供后续 go-zero 原生链路中间件创建同链路 Span。
		r = r.Clone(ctx)
		// 先清除不同大小写的旧别名，再写入单值，避免再次转发时出现重复链路头。
		for key := range r.Header {
			switch strings.ToLower(key) {
			case "x-request-id", "traceparent", "tracestate":
				delete(r.Header, key)
			}
		}
		r.Header.Set(headInfo.RequestIDHeader, id)
		(propagation.TraceContext{}).Inject(ctx, propagation.HeaderCarrier(r.Header))
		w.Header().Set(headInfo.RequestIDHeader, id)
		next(w, r)
	}
}
