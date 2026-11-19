package rabbitmq

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/K7mQ2vR9/base-pkg/commonTool"
	"github.com/streadway/amqp"
)

// Message 为发布和消费共用的业务消息，不暴露 ACK/NACK 或驱动连接。
// 正文和嵌套 Headers 在发布及交付处理器时复制，业务修改不会改变待重试的原消息。
type Message struct {
	// ID 标识具体消息，可指定业务自增 ID 的字符串；缺省生成 36 位 UUIDv7，非自增数字。
	// 同一毫秒不保证递增，消费重试沿用，不与 Trace ID 合并。
	ID string
	// Body 为消息正文；DecodeJSON 可将 JSON 正文解码到业务结构体。
	Body []byte
	// Headers 为业务头，支持 AMQP 标量、[]byte、[]any 和嵌套 map[string]any。
	Headers map[string]any
	// ContentType 发布缺省为 application/json；PublishJSON 始终设置此类型。
	ContentType string
	// ContentEncoding 为可选的正文编码，缺省为空。
	ContentEncoding string
	// Persistent 未设置时采用 Config.Persistent；消费时反映消息实际持久化标记。
	Persistent *bool
	// Priority 为消息优先级，默认 0，实际支持范围和调度取决于队列设置。
	Priority uint8
	// CorrelationID 为可选的业务关联 ID，不作为链路标识或自动去重依据。
	CorrelationID string
	// ReplyTo 为可选的回复目标，默认空。
	ReplyTo string
	// Expiration 为可选的非负毫秒数字串，空表示不设置消息 TTL。
	Expiration string
	// Timestamp 发布未设置时采用当前 UTC 时间，消费时为原生产时间。
	Timestamp time.Time
	// Type、AppID 和 UserID 为可选的消息类型、来源应用及 broker 校验的生产者身份。
	Type   string
	AppID  string
	UserID string
	// RetryCount 仅在消费时读取，首次为 0，表示已进入本包业务重试的次数。
	RetryCount int
	// Redelivered 仅在消费时读取，是 broker 的重新投递提示，不等同于 RetryCount 或去重保证。
	Redelivered bool
	// Exchange 和 RoutingKey 仅在消费时读取，发布目标由客户端 Config 决定。
	Exchange   string
	RoutingKey string
}

// DecodeJSON 使用标准 JSON 语义解码到非 nil 指针，不负责业务字段校验。
// 解码失败保留底层错误链，公开错误文本不包含正文。格式、字段类型和无效目标错误默认直接归档。
// 业务可用 Retryable 覆盖默认分类，自定义解码器的其他错误默认重试。
func (m Message) DecodeJSON(target any) error {
	if err := json.Unmarshal(m.Body, target); err != nil {
		return &operationError{"decode JSON", err}
	}
	return nil
}

// PublishOption 在构造期间配置 JSON 消息元数据，不应保留传入的指针。
// JSON 正文及 ContentType 由构造函数统一设置。
type PublishOption func(*Message)

// WithMessageID 指定业务消息 ID；空字符串使用自动生成，重发同一条消息时沿用已有 ID。
func WithMessageID(id string) PublishOption { return func(m *Message) { m.ID = id } }

// WithHeaders 设置业务消息头，构造及发布时复制，不修改调用方提供的 map。
func WithHeaders(headers map[string]any) PublishOption {
	return func(m *Message) { m.Headers = headers }
}

// WithCorrelationID 设置可选的业务关联 ID，缺省为空。
func WithCorrelationID(id string) PublishOption {
	return func(m *Message) { m.CorrelationID = id }
}

// NewJSONMessage 为 JSON 消息分配 ID 并序列化，用于离线准备或批量发布，不连接 broker。
// 已分配 ID 时即使序列化或元数据校验失败也返回该 ID；失败消息不能直接作为有效正文发送。
func NewJSONMessage(data any, options ...PublishOption) (Message, error) {
	var message Message
	for _, option := range options {
		if option == nil {
			return message, errors.New("rabbitmq publish option is nil")
		}
		option(&message)
	}
	var err error
	message.ID, err = resolveMessageID(message.ID)
	if err != nil {
		return message, err
	}
	message.Body, err = json.Marshal(data)
	if err != nil {
		return message, &operationError{"encode JSON", err}
	}
	message.ContentType = "application/json"
	if err := validateMessage(message); err != nil {
		return message, err
	}
	message.Headers, err = cloneHeaders(message.Headers)
	return message, err
}

// BatchResult 描述顺序发布结果，首次失败即停止，不自动重发。
type BatchResult struct {
	// MessageIDs 与输入顺序一致；已尝试项包含实际 ID，未尝试项保留传入 ID，未分配时为空。
	MessageIDs []string
	// ConfirmedCount 是连续收到 broker 成功确认且已路由的前缀数量。
	ConfirmedCount int
	// FailedIndex 为从 0 开始的失败位置；全部成功或批次初始化失败为 -1。
	// 失败位置可能已被 broker 接收，须结合 errors.Is(err, ErrPublishUnknown) 判断。
	FailedIndex int
}

func resolveMessageID(id string) (string, error) {
	if id == "" {
		generated, err := commonTool.NewUUidV7()
		if err != nil {
			return "", &operationError{"generate message ID", err}
		}
		id = generated
	}
	if strings.TrimSpace(id) == "" || len(id) > 255 {
		return id, errors.New("rabbitmq message ID must be non-blank and at most 255 bytes")
	}
	return id, nil
}

func validateMessage(m Message) error {
	// 驱动的 shortstr 写入会截断过长字符串，提前校验以保证返回 ID 与 broker 收到的 ID 一致。
	for _, property := range []struct{ name, value string }{
		{"ID", m.ID}, {"ContentType", m.ContentType}, {"ContentEncoding", m.ContentEncoding},
		{"CorrelationID", m.CorrelationID}, {"ReplyTo", m.ReplyTo}, {"Expiration", m.Expiration},
		{"Type", m.Type}, {"AppID", m.AppID}, {"UserID", m.UserID},
	} {
		if len(property.value) > 255 {
			return fmt.Errorf("rabbitmq message %s exceeds 255 bytes", property.name)
		}
	}
	if m.Expiration != "" {
		if _, err := strconv.ParseUint(m.Expiration, 10, 63); err != nil {
			return errors.New("rabbitmq message expiration must be a non-negative millisecond integer")
		}
	}
	return nil
}

func (m Message) publishing(persistent bool) (amqp.Publishing, error) {
	if err := validateMessage(m); err != nil {
		return amqp.Publishing{}, err
	}
	headers, err := cloneHeaders(m.Headers)
	if err != nil {
		return amqp.Publishing{}, err
	}
	mode := uint8(amqp.Transient)
	if configValue(m.Persistent, persistent) {
		mode = amqp.Persistent
	}
	contentType := m.ContentType
	if contentType == "" {
		contentType = "application/json"
	}
	timestamp := m.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	return amqp.Publishing{Headers: wireHeaders(headers), Body: append([]byte(nil), m.Body...), MessageId: m.ID,
		ContentType: contentType, ContentEncoding: m.ContentEncoding, DeliveryMode: mode, Priority: m.Priority,
		CorrelationId: m.CorrelationID, ReplyTo: m.ReplyTo, Expiration: m.Expiration, Timestamp: timestamp,
		Type: m.Type, AppId: m.AppID, UserId: m.UserID}, nil
}

func messageFromDelivery(d amqp.Delivery, count int) (Message, error) {
	headers, err := cloneHeaders(map[string]any(d.Headers))
	if err != nil {
		return Message{}, err
	}
	persistent := d.DeliveryMode == amqp.Persistent
	return Message{ID: d.MessageId, Body: append([]byte(nil), d.Body...), Headers: headers,
		ContentType: d.ContentType, ContentEncoding: d.ContentEncoding, Persistent: &persistent, Priority: d.Priority,
		CorrelationID: d.CorrelationId, ReplyTo: d.ReplyTo, Expiration: d.Expiration, Timestamp: d.Timestamp,
		Type: d.Type, AppID: d.AppId, UserID: d.UserId, RetryCount: count, Redelivered: d.Redelivered,
		Exchange: d.Exchange, RoutingKey: d.RoutingKey}, nil
}

type headerIdentity struct {
	table  string
	slice  uintptr
	length int
}

func cloneHeaders(headers map[string]any) (map[string]any, error) {
	if headers == nil {
		return nil, nil
	}
	value, err := cloneHeaderValue(headers, make(map[headerIdentity]bool))
	if err != nil {
		return nil, &operationError{"copy message headers", err}
	}
	return value.(map[string]any), nil
}

// cloneHeaderValue 隔离嵌套表、数组及二进制值，拒绝循环引用和驱动不支持的类型。
func cloneHeaderValue(value any, path map[headerIdentity]bool) (any, error) {
	if table, ok := value.(amqp.Table); ok {
		value = map[string]any(table)
	}
	switch v := value.(type) {
	case map[string]any:
		identity := headerIdentity{table: fmt.Sprintf("%p", v)}
		if path[identity] {
			return nil, errors.New("cyclic message headers")
		}
		path[identity] = true
		defer delete(path, identity)
		result := make(map[string]any, len(v))
		for key, item := range v {
			if len(key) == 0 || len(key) > 255 {
				return nil, errors.New("message header keys must contain 1..255 bytes")
			}
			copied, err := cloneHeaderValue(item, path)
			if err != nil {
				return nil, err
			}
			result[key] = copied
		}
		return result, nil
	case []any:
		identity := headerIdentity{slice: reflect.ValueOf(v).Pointer(), length: len(v)}
		if path[identity] {
			return nil, errors.New("cyclic message headers")
		}
		path[identity] = true
		defer delete(path, identity)
		result := make([]any, len(v))
		for index, item := range v {
			copied, err := cloneHeaderValue(item, path)
			if err != nil {
				return nil, err
			}
			result[index] = copied
		}
		return result, nil
	case []byte:
		return append([]byte(nil), v...), nil
	case int:
		if int64(v) < math.MinInt32 || int64(v) > math.MaxInt32 {
			return nil, errors.New("message header int exceeds int32 range; use int64")
		}
		return v, nil
	case nil, bool, byte, int16, int32, int64, float32, float64, string, amqp.Decimal, time.Time:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported message header type %T", value)
	}
}

// wireHeaders 只在驱动边界转换本包已复制的 map/数组，不修改业务消息。
func wireHeaders(headers map[string]any) amqp.Table {
	for key, value := range headers {
		headers[key] = wireHeaderValue(value)
	}
	return amqp.Table(headers)
}

func wireHeaderValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return wireHeaders(v)
	case []any:
		for index, item := range v {
			v[index] = wireHeaderValue(item)
		}
	}
	return value
}
