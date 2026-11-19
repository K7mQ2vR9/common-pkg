package rabbitmq

import (
	"context"
	"errors"
	"strings"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel/propagation"
)

func isTraceHeader(key string) bool {
	switch strings.ToLower(key) {
	case "traceparent", "tracestate":
		return true
	}
	return false
}

// messageTraceContext 从消息恢复统一 ID，已有活动链路与消息头冲突时拒绝发送或执行业务。
// 独立任务或外部无链路消息仅在全部 ID 缺失时生成新 ID，不继承生产请求的取消和截止时间。
func messageTraceContext(ctx context.Context, headers amqp.Table) (context.Context, error) {
	values := make(map[string][]string)
	for key, value := range headers {
		if !isTraceHeader(key) {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return ctx, headInfo.ErrTraceInvalid
		}
		key = strings.ToLower(key)
		values[key] = append(values[key], text)
	}
	extracted, err := headInfo.ExtractTrace(ctx, values)
	if errors.Is(err, headInfo.ErrTraceRequired) {
		return headInfo.EnsureTrace(ctx)
	}
	if err != nil {
		return ctx, err
	}
	if active := headInfo.GetTrace(ctx); active != "" && active != headInfo.GetTrace(extracted) {
		return ctx, headInfo.ErrTraceConflict
	}
	return headInfo.EnsureTrace(extracted)
}

// messageTraceHeaders 复制原始头并写入当前 Span 的 W3C 上下文，链路只使用标准传播字段。
func messageTraceHeaders(ctx context.Context, headers amqp.Table) amqp.Table {
	result := make(amqp.Table, len(headers)+2)
	for key, value := range headers {
		if !isTraceHeader(key) {
			result[key] = value
		}
	}
	carrier := propagation.MapCarrier{}
	(propagation.TraceContext{}).Inject(ctx, carrier)
	for key, value := range carrier {
		result[key] = value
	}
	return result
}
