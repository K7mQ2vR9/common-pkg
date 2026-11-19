package rabbitmq

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/streadway/amqp"
)

// Config 配置业务队列、重试、失败归档及 TG 通知，未填写的可选项采用默认值。
type Config struct {
	// URL 为必填 AMQP/AMQPS 地址，包含认证和 Vhost。
	URL string `json:",optional"`
	// QueueName 为必填业务队列名。
	QueueName string `json:",optional"`
	// Exchange 为空时使用默认交换机，RoutingKey 自动使用 QueueName。
	Exchange string `json:",optional"`
	// ExchangeType 默认 direct，声明具名交换机时使用。
	ExchangeType string `json:",optional"`
	// RoutingKey 为具名交换机的发布及绑定键，默认使用 QueueName。
	RoutingKey string `json:",optional"`
	// QueueDurable 和 ExchangeDurable 默认 true，显式 false 可关闭相应持久化。
	QueueDurable    *bool `json:",optional"`
	ExchangeDurable *bool `json:",optional"`
	// QueueAutoDelete 和 QueueExclusive 默认 false。
	QueueAutoDelete bool `json:",optional"`
	QueueExclusive  bool `json:",optional"`
	// DialTimeout 覆盖连接、握手和拓扑初始化，零值默认 5s。
	DialTimeout time.Duration `json:",optional"`
	// Heartbeat 默认 10s，显式 0 采用 broker 的间隔，其他值须为 1～65535 整秒。
	Heartbeat *time.Duration `json:",optional"`
	// ReconnectInterval 重连失败或连接失效后的等待时间，零值默认 5s。
	ReconnectInterval time.Duration `json:",optional"`
	// ConfirmTimeout 覆盖发布的串行等待、网络写入和 broker 确认，零值默认 5s。
	ConfirmTimeout time.Duration `json:",optional"`
	// ShutdownTimeout 为关闭时等待处理协程收尾的时限，零值默认 10s。
	ShutdownTimeout time.Duration `json:",optional"`
	// WorkerNum 为业务消费并发数，默认 1，显式值须大于零。
	WorkerNum *int `json:",optional"`
	// PrefetchCount 默认跟随 WorkerNum，范围 0～65535，显式 0 表示不限制未确认消息数。
	PrefetchCount *int `json:",optional"`
	// AutoAck 默认 false，true 时业务消息在投递时自动确认，转存失败无法恢复原消息。
	// 需要可靠重试和失败归档时保持 false。
	AutoAck bool `json:",optional"`
	// Persistent 默认 true，发布消息未显式指定 DeliveryMode 时采用该设置。
	Persistent *bool `json:",optional"`
	// MaxRetries 默认 3，表示首次处理之外的重试次数，显式 0 直接进入失败归档。
	MaxRetries *int `json:",optional"`
	// RetryDelay 为重试队列等待时间，零值默认 30s。
	RetryDelay time.Duration `json:",optional"`
	// RetryQueue、FailedQueue、NotifyQueue 默认分别为 QueueName 加 .retry、.failed、.notify。
	RetryQueue  string `json:",optional"`
	FailedQueue string `json:",optional"`
	NotifyQueue string `json:",optional"`
	// ServiceName 用于 TG 告警，默认使用 QueueName。
	ServiceName string `json:",optional"`
	// Telegram 控制通知任务的发送，关闭时任务保存在通知队列中。
	Telegram TelegramConfig `json:",optional"`
}

// TelegramConfig 配置重试耗尽后的通知发送，默认关闭。
type TelegramConfig struct {
	// Enabled 默认 false，启用后在注册业务消费者时启动独立通知消费者。
	Enabled bool `json:",optional"`
	// BotToken 和 ChatID 在启用时必填，ChatID 支持群组 ID 或频道用户名。
	BotToken string `json:",optional"`
	ChatID   string `json:",optional"`
	// Timeout 为 HTTP 发送超时，零值默认 5s。
	Timeout time.Duration `json:",optional"`
	// RetryInterval 为 TG 发送失败的重试间隔，零值默认 30s，并遵守服务端 retry_after。
	RetryInterval time.Duration `json:",optional"`
}

type settings struct {
	Config
	workerNum, prefetch, maxRetries           int
	queueDurable, exchangeDurable, persistent bool
	heartbeat                                 time.Duration
	failedExchange                            string
}

func configValue[T any](value *T, fallback T) T {
	if value != nil {
		return *value
	}
	return fallback
}

func (config Config) settings() (settings, error) {
	s := settings{Config: config}
	if strings.TrimSpace(s.URL) == "" || strings.TrimSpace(s.QueueName) == "" {
		return s, errors.New("rabbitmq URL and QueueName are required")
	}
	if _, err := amqp.ParseURI(s.URL); err != nil {
		return s, &operationError{"parse URL", err}
	}
	if s.ExchangeType == "" {
		s.ExchangeType = "direct"
	}
	if s.RoutingKey == "" || s.Exchange == "" {
		s.RoutingKey = s.QueueName
	}
	if s.RetryQueue == "" {
		s.RetryQueue = s.QueueName + ".retry"
	}
	if s.FailedQueue == "" {
		s.FailedQueue = s.QueueName + ".failed"
	}
	if s.NotifyQueue == "" {
		s.NotifyQueue = s.QueueName + ".notify"
	}
	names := map[string]bool{}
	for _, name := range []string{s.QueueName, s.RetryQueue, s.FailedQueue, s.NotifyQueue} {
		if names[name] {
			return s, errors.New("rabbitmq queue names must be distinct")
		}
		names[name] = true
	}
	if s.ServiceName == "" {
		s.ServiceName = s.QueueName
	}
	s.failedExchange = s.QueueName + ".failed.exchange"
	if s.Exchange == s.failedExchange {
		return s, errors.New("rabbitmq business and failure exchanges must be distinct")
	}
	s.workerNum = configValue(s.WorkerNum, 1)
	s.prefetch = configValue(s.PrefetchCount, s.workerNum)
	s.maxRetries = configValue(s.MaxRetries, 3)
	s.queueDurable = configValue(s.QueueDurable, true)
	s.exchangeDurable = configValue(s.ExchangeDurable, true)
	s.persistent = configValue(s.Persistent, true)
	s.heartbeat = configValue(s.Heartbeat, 10*time.Second)
	if s.workerNum <= 0 || s.prefetch < 0 || s.prefetch > math.MaxUint16 || s.maxRetries < 0 || s.maxRetries > math.MaxInt32 {
		return s, errors.New("rabbitmq worker count must be positive and prefetch/retries must be valid non-negative integers")
	}
	for _, value := range []struct {
		target   *time.Duration
		fallback time.Duration
	}{
		{&s.DialTimeout, 5 * time.Second}, {&s.ReconnectInterval, 5 * time.Second}, {&s.ConfirmTimeout, 5 * time.Second},
		{&s.ShutdownTimeout, 10 * time.Second}, {&s.RetryDelay, 30 * time.Second},
		{&s.Telegram.Timeout, 5 * time.Second}, {&s.Telegram.RetryInterval, 30 * time.Second},
	} {
		if *value.target < 0 {
			return s, errors.New("rabbitmq durations must be non-negative")
		}
		if *value.target == 0 {
			*value.target = value.fallback
		}
	}
	if s.heartbeat < 0 || s.heartbeat > math.MaxUint16*time.Second || s.heartbeat%time.Second != 0 {
		return s, errors.New("rabbitmq Heartbeat must be zero or 1..65535 whole seconds")
	}
	if s.Telegram.Enabled && (strings.TrimSpace(s.Telegram.BotToken) == "" || strings.TrimSpace(s.Telegram.ChatID) == "") {
		return s, errors.New("telegram BotToken and ChatID are required when enabled")
	}
	if strings.ContainsAny(s.Telegram.BotToken, "/?#\r\n") {
		return s, errors.New("telegram BotToken contains invalid characters")
	}
	return s, nil
}

// operationError 的公开文本省略连接串和凭证，底层原因供 errors.Is/As 检查。
type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return "rabbitmq " + e.operation + " failed" }
func (e *operationError) Unwrap() error { return e.cause }
