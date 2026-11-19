package rabbitmq

import (
	"encoding/json"
	"errors"
)

// Permanent 标记不可恢复的消费错误，直接归档并创建 TG 通知任务，不占用业务重试次数。
// nil 返回 nil；保留原错误文本及错误链，支持 errors.Is/As 和 fmt.Errorf 的 %w 包装。
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &processingError{cause: err, retry: false}
}

// Retryable 显式要求按 MaxRetries 重试，可覆盖 JSON 默认分类或更内层的 Permanent 标记。
// nil 返回 nil；不解除最大重试次数、处理超时或服务生命周期取消的限制。
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return &processingError{cause: err, retry: true}
}

type processingError struct {
	cause error
	retry bool
}

func (e *processingError) Error() string { return e.cause.Error() }
func (e *processingError) Unwrap() error { return e.cause }

// shouldRetry 优先采用 errors.As 找到的第一个显式标记，最外层覆盖内层。
// 多错误分支遵循标准库深度优先、分支从左到右的查找顺序；无标记时 JSON 格式/类型/目标错误永久失败。
// 自定义 JSON 解码器的其他错误与普通业务错误默认重试，调用方可显式指定策略。
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	var marked *processingError
	if errors.As(err, &marked) && marked != nil {
		return marked.retry
	}
	var syntax *json.SyntaxError
	var fieldType *json.UnmarshalTypeError
	var target *json.InvalidUnmarshalError
	return !errors.As(err, &syntax) && !errors.As(err, &fieldType) && !errors.As(err, &target)
}

type failureReason string

const (
	// 归档原因仅用于终态失败诊断，不从消息头反向决定业务错误是否可重试。
	failurePermanent        failureReason = "permanent"
	failureRetriesExhausted failureReason = "retries_exhausted"
	failureInvalidMetadata  failureReason = "invalid_metadata"
)
