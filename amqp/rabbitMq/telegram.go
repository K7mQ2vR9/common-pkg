package rabbitmq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/streadway/amqp"
	"github.com/zeromicro/go-zero/core/logc"
)

func (r *RabbitMQ) processNotification(ctx context.Context, d amqp.Delivery) error {
	traced, err := messageTraceContext(ctx, d.Headers)
	if err != nil {
		// 非法消息链路仍允许发送失败告警，诊断使用本地 ID，不能使通知永久无法处理。
		traced, err = headInfo.EnsureTrace(ctx)
		if err != nil {
			return err
		}
	}
	ctx = traced
	text := r.failureNotification(ctx, d)
	for {
		delay, err := r.sendTelegram(ctx, text)
		if err == nil {
			return d.Ack(false)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logc.Errorf(ctx, "Telegram notification failed: %v", err)
		if delay < r.config.Telegram.RetryInterval {
			delay = r.config.Telegram.RetryInterval
		}
		if !wait(ctx, delay) {
			return ctx.Err()
		}
	}
}

// failureNotification 按持久化的终态原因生成告警，不把永久错误或内部元数据错误误报为重试耗尽。
func (r *RabbitMQ) failureNotification(ctx context.Context, d amqp.Delivery) string {
	reason, _ := d.Headers[failureReasonHeader].(string)
	title, label, attempts := "消息处理失败", "未知归档原因", "未知"
	switch failureReason(reason) {
	case failurePermanent:
		title, label = "消息不可重试错误", "永久错误"
	case failureRetriesExhausted:
		title, label = "消息重试耗尽", "重试次数耗尽"
	case failureInvalidMetadata:
		title, label, attempts = "消息元数据错误", "内部元数据无效", "本次未执行"
	}
	if reason == string(failurePermanent) || reason == string(failureRetriesExhausted) {
		if count, err := retryCount(d.Headers[retryCountHeader]); err == nil {
			attempts = fmt.Sprintf("%d", int64(count)+1)
		}
	}
	return fmt.Sprintf("%s\n归档原因：%s\n服务：%s\n队列：%s\n消息 ID：%s\nTrace ID：%s\n处理次数：%s\n错误摘要：%v",
		title, label, r.config.ServiceName, r.config.QueueName, d.MessageId, headInfo.GetTrace(ctx), attempts, d.Headers[errorHeader])
}

func (r *RabbitMQ) sendTelegram(parent context.Context, text string) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(parent, r.config.Telegram.Timeout)
	defer cancel()
	payload, err := json.Marshal(map[string]string{"chat_id": r.config.Telegram.ChatID, "text": telegramText(text)})
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+r.config.Telegram.BotToken+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return 0, errors.New("telegram request construction failed")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := r.httpClient.Do(request)
	if err != nil {
		return 0, &operationError{"telegram transport", err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(body) > 65536 {
		return 0, errors.New("telegram response read failed")
	}
	var reply struct {
		OK         bool `json:"ok"`
		ErrorCode  int  `json:"error_code"`
		Parameters struct {
			RetryAfter int64 `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return 0, errors.New("telegram response is invalid")
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 && reply.OK {
		return 0, nil
	}
	var retryAfter time.Duration
	if reply.Parameters.RetryAfter > 0 && reply.Parameters.RetryAfter <= math.MaxInt64/int64(time.Second) {
		retryAfter = time.Duration(reply.Parameters.RetryAfter) * time.Second
	}
	return retryAfter, fmt.Errorf("telegram send failed (HTTP %d, API %d)", response.StatusCode, reply.ErrorCode)
}

// telegramText 按 UTF-16 单元限制 Telegram 纯文本长度，避免切断多字节字符。
func telegramText(text string) string {
	var result strings.Builder
	units := 0
	for _, character := range text {
		size := utf16.RuneLen(character)
		if units+size > 4096 {
			break
		}
		result.WriteRune(character)
		units += size
	}
	return result.String()
}
