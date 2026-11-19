package headInfo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"strings"

	ztrace "github.com/zeromicro/go-zero/core/trace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// RequestIDHeader 为 HTTP 对外请求头名，值直接使用统一 Trace ID。
const RequestIDHeader = "X-Request-ID"

var (
	// ErrTraceRequired 表示缺少标准链路头，HTTP 入口可为全新请求生成 ID，RPC 必须拒绝。
	ErrTraceRequired = errors.New("trace ID is required")
	// ErrTraceInvalid 表示 ID 格式错误、字段为空或同一字段重复出现。
	ErrTraceInvalid = errors.New("trace metadata is invalid")
	// ErrTraceConflict 表示 HTTP 请求 ID 与标准链路或活动上下文的 Trace ID 不同。
	ErrTraceConflict = errors.New("request ID and trace ID must match")
)

// NewTraceID 为全新请求生成统一 ID：crypto/rand 读取 16 个随机字节，编码为 32 位小写十六进制。
// 不含时间戳、业务前缀或连字符；全零时重新生成，随机源失败直接返回错误。
// 前端与网关采用相同格式；入口已有合法 ID 时复用，下游不重新生成。
func NewTraceID() (string, error) {
	var id trace.TraceID
	for !id.IsValid() {
		if _, err := io.ReadFull(rand.Reader, id[:]); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(id[:]), nil
}

// GetTrace 返回本次调用唯一的 Trace ID，供业务、日志和响应共用。
func GetTrace(ctx context.Context) string {
	span := trace.SpanContextFromContext(ctx)
	if !span.IsValid() {
		return ""
	}
	return span.TraceID().String()
}

// ExtractTrace 仅从 W3C traceparent/tracestate 恢复 RPC 或消息链路，不读取其他 ID 别名。
// traceparent 缺失返回 ErrTraceRequired，空值、重复和非法格式返回 ErrTraceInvalid。
// 原始 headers 不修改；当前活动 Span 属于同一 Trace 时保留它。
func ExtractTrace(ctx context.Context, headers map[string][]string) (context.Context, error) {
	parents, present := headerValues(headers, "traceparent")
	if !present {
		return ctx, ErrTraceRequired
	}
	if len(parents) != 1 || parents[0] == "" {
		return ctx, ErrTraceInvalid
	}
	carrier := propagation.MapCarrier{"traceparent": parents[0]}
	if states, _ := headerValues(headers, "tracestate"); len(states) > 0 {
		carrier["tracestate"] = strings.Join(states, ",")
	}
	extracted := (propagation.TraceContext{}).Extract(context.Background(), carrier)
	parent := trace.SpanContextFromContext(extracted)
	if !parent.IsValid() {
		return ctx, ErrTraceInvalid
	}
	if active := trace.SpanContextFromContext(ctx); active.IsValid() && active.TraceID() == parent.TraceID() {
		return ctx, nil
	}
	return trace.ContextWithRemoteSpanContext(ctx, parent), nil
}

// ExtractRequestTrace 校验 HTTP 的 X-Request-ID 和标准 traceparent，二者同时存在时必须相同。
// 请求 ID 必须是非零的 32 位小写十六进制；不接收任意字符串、空值或重复值。
// 只有请求 ID 时生成独立的 8 字节非零 Span ID 补齐传播上下文，全部缺失返回 ErrTraceRequired。
func ExtractRequestTrace(ctx context.Context, headers map[string][]string) (context.Context, error) {
	extracted, err := ExtractTrace(ctx, headers)
	if err != nil && !errors.Is(err, ErrTraceRequired) {
		return ctx, err
	}
	ids, present := headerValues(headers, RequestIDHeader)
	if !present {
		return extracted, err
	}
	if len(ids) != 1 {
		return ctx, ErrTraceInvalid
	}
	id, parseErr := trace.TraceIDFromHex(ids[0])
	if parseErr != nil {
		return ctx, ErrTraceInvalid
	}
	if err == nil {
		if GetTrace(extracted) != id.String() {
			return ctx, ErrTraceConflict
		}
		return extracted, nil
	}
	return contextWithTraceID(ctx, id)
}

// EnsureTrace 沿用活动上下文或出站 W3C 链路，缺失时才生成统一 ID，再同步标准传播字段。
// RPC 和 MQ 仅使用 traceparent/tracestate；活动链路与 metadata 冲突时返回错误。
func EnsureTrace(ctx context.Context) (context.Context, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	extracted, err := ExtractTrace(ctx, md)
	if err == nil {
		if active := GetTrace(ctx); active != "" && active != GetTrace(extracted) {
			return ctx, ErrTraceConflict
		}
		ctx = extracted
	} else if !errors.Is(err, ErrTraceRequired) {
		return ctx, err
	} else if GetTrace(ctx) == "" {
		id, err := NewTraceID()
		if err != nil {
			return ctx, err
		}
		parsed, err := trace.TraceIDFromHex(id)
		if err != nil {
			return ctx, err
		}
		ctx, err = contextWithTraceID(ctx, parsed)
		if err != nil {
			return ctx, err
		}
	}
	return traceMetadata(ctx), nil
}

// SetTrace 校验并绑定指定 ID，同步 RPC 出站 W3C 链路；活动 Trace 不同则拒绝覆盖。
func SetTrace(ctx context.Context, value string) (context.Context, error) {
	id, err := trace.TraceIDFromHex(value)
	if err != nil {
		return ctx, ErrTraceInvalid
	}
	if active := GetTrace(ctx); active != "" && active != value {
		return ctx, ErrTraceConflict
	}
	ctx, err = contextWithTraceID(ctx, id)
	if err != nil {
		return ctx, err
	}
	return traceMetadata(ctx), nil
}

func traceMetadata(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	// tracestate 以当前 Span 为准，避免缺少父级时沿用其他链路的状态。
	md.Delete("tracestate")
	ztrace.Inject(ctx, propagation.TraceContext{}, &md)
	return metadata.NewOutgoingContext(ctx, md)
}

func contextWithTraceID(ctx context.Context, id trace.TraceID) (context.Context, error) {
	if active := trace.SpanContextFromContext(ctx); active.IsValid() && active.TraceID() == id {
		return ctx, nil
	}
	var span trace.SpanID
	for !span.IsValid() {
		if _, err := io.ReadFull(rand.Reader, span[:]); err != nil {
			return ctx, err
		}
	}
	return trace.ContextWithRemoteSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{TraceID: id, SpanID: span, Remote: true})), nil
}

func headerValues(headers map[string][]string, name string) ([]string, bool) {
	var values []string
	present := false
	for key, list := range headers {
		if strings.EqualFold(key, name) {
			present = true
			values = append(values, list...)
		}
	}
	return values, present
}
