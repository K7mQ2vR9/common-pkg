package app

import (
	"context"
	"net/http"
	"strings"

	"github.com/K7mQ2vR9/base-pkg/appMiddleware"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/router"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// NewServer 创建统一请求 ID 的 API Server，入口校验先于 go-zero 原生链路、日志和业务中间件。
// 其余配置沿用 rest.NewServer；自定义 Router 用 WithRequestTraceRouter 接入，勿以裸 Router 覆盖。
func NewServer(config rest.RestConf, options ...rest.RunOption) (*rest.Server, error) {
	return rest.NewServer(config, append([]rest.RunOption{WithRequestTraceRouter(nil, config.Telemetry.Sampler)}, options...)...)
}

// WithRequestTraceRouter 在自定义路由外层接入统一 ID，nil 使用默认路由。
// 可传给 rest.NewServer；自定义 NotFound、NotAllowed 或 CORS 选项在此选项之后设置。
// samplingRate 对应 RestConf.Telemetry.Sampler：0 不采样、1 全采样；已有 W3C 上下文沿用原决定。
func WithRequestTraceRouter(base httpx.Router, samplingRate float64) rest.RunOption {
	if base == nil {
		base = router.NewRouter()
	}
	sampler := sdktrace.TraceIDRatioBased(samplingRate)
	wrapped := &requestTraceRouter{Router: base}
	wrapped.serve = func(w http.ResponseWriter, r *http.Request) {
		inherited := trace.SpanContextFromContext(r.Context()).IsValid()
		for key := range r.Header {
			if strings.EqualFold(key, "traceparent") {
				inherited = true
			}
		}
		appMiddleware.EnsureRequestTrace(func(w http.ResponseWriter, request *http.Request) {
			if !inherited {
				// 只有请求 ID 时尚无上游采样决定，按服务的原采样比例建立传播上下文。
				// 避免人为补齐的未采样父上下文使 go-zero 的 ParentBased 采样器丢弃所有新请求。
				ctx := request.Context()
				sc := trace.SpanContextFromContext(ctx)
				decision := sampler.ShouldSample(sdktrace.SamplingParameters{ParentContext: context.Background(), TraceID: sc.TraceID()})
				sc = sc.WithTraceFlags(sc.TraceFlags().WithSampled(decision.Decision == sdktrace.RecordAndSample))
				ctx = trace.ContextWithRemoteSpanContext(ctx, sc)
				ctx, _ = headInfo.EnsureTrace(ctx)
				request = request.WithContext(ctx)
				(propagation.TraceContext{}).Inject(ctx, propagation.HeaderCarrier(request.Header))
			}
			base.ServeHTTP(w, request)
		})(w, r)
	}
	return func(server *rest.Server) {
		rest.WithRouter(wrapped)(server)
		rest.WithNotFoundHandler(nil)(server)
	}
}

type requestTraceRouter struct {
	httpx.Router
	serve http.HandlerFunc
}

func (r *requestTraceRouter) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	r.serve(w, request)
}
