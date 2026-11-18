package headInfo

import (
	"context"
	"crypto/rand"

	"github.com/K7mQ2vR9/base-pkg/consts"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// GetTrace 读取当前 OpenTelemetry Trace ID。
func GetTrace(ctx context.Context) string {
	span := trace.SpanContextFromContext(ctx)
	if !span.IsValid() {
		return ""
	}
	return span.TraceID().String()
}

// EnsureTrace 保留当前链路。没有有效 SpanContext 时生成可传播的 Trace ID 和 Span ID。
func EnsureTrace(ctx context.Context) (context.Context, error) {
	if trace.SpanContextFromContext(ctx).IsValid() {
		return ctx, nil
	}
	var ids [24]byte
	for {
		if _, err := rand.Read(ids[:]); err != nil {
			return ctx, err
		}
		span := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: trace.TraceID(ids[:16]),
			SpanID:  trace.SpanID(ids[16:]),
		})
		if span.IsValid() {
			return trace.ContextWithSpanContext(ctx, span), nil
		}
	}
}

// SetTrace 更新 RPC metadata 中的兼容字段，已有 OpenTelemetry Trace ID 时优先使用它。
func SetTrace(ctx context.Context, value string) context.Context {
	if id := GetTrace(ctx); id != "" {
		value = id
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set(consts.Trace, value)
	return metadata.NewOutgoingContext(ctx, md)
}
