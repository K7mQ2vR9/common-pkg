package commonTool

import (
	"errors"
	"math"
	"time"
)

// HTTPConfig 配置可复用的 HTTP 工具客户端，不修改进程级 http.DefaultClient/DefaultTransport。
type HTTPConfig struct {
	// Timeout 为一次调用的总预算，包含 JSON 编码耗时、建连、响应头及正文读取，零值默认 10s，负值非法。
	// JSON 编码为同步计算，无法强制中断；编码结束后已超时则不发起网络请求。
	Timeout time.Duration `json:",optional"`
	// MaxResponseBytes 为单个响应正文的最大读取字节数，零值默认 4MiB；负值或 MaxInt64 非法。
	// 自动解压的响应按解压后字节计数，超限保留上限内前缀并返回 ErrHTTPResponseTooLarge。
	MaxResponseBytes int64 `json:",optional"`
	// Transport 独立配置阶段超时和共享连接池，未配置时采用各字段默认值。
	Transport HTTPTransportConfig `json:",optional"`
}

// HTTPTransportConfig 的时长单位为 time.Duration，容量单位为连接数；零值采用默认值，负数非法。
type HTTPTransportConfig struct {
	// DialTimeout 为 TCP 建连时限，默认 5s，父调用更早的截止时间优先。
	DialTimeout time.Duration `json:",optional"`
	// TLSHandshakeTimeout 为 TLS 握手时限，默认 5s；HTTPS 默认验证系统信任证书与主机名。
	TLSHandshakeTimeout time.Duration `json:",optional"`
	// ResponseHeaderTimeout 为发送请求后等待响应头的时限，默认 5s，不包含读取正文。
	ResponseHeaderTimeout time.Duration `json:",optional"`
	// MaxIdleConns 为连接池最大空闲连接总数，默认 100。
	MaxIdleConns int `json:",optional"`
	// MaxIdleConnsPerHost 为每个节点最大空闲连接数，默认 100，不得超过 MaxConnsPerHost。
	MaxIdleConnsPerHost int `json:",optional"`
	// MaxConnsPerHost 为每个节点连接总数上限，默认 100，包含建连、使用及空闲连接。
	MaxConnsPerHost int `json:",optional"`
	// IdleConnTimeout 为空闲连接保留时间，默认 90s。
	IdleConnTimeout time.Duration `json:",optional"`
}

func (config HTTPConfig) normalized() (HTTPConfig, error) {
	for _, item := range []struct {
		value    *time.Duration
		fallback time.Duration
	}{
		{&config.Timeout, 10 * time.Second},
		{&config.Transport.DialTimeout, 5 * time.Second},
		{&config.Transport.TLSHandshakeTimeout, 5 * time.Second},
		{&config.Transport.ResponseHeaderTimeout, 5 * time.Second},
		{&config.Transport.IdleConnTimeout, 90 * time.Second},
	} {
		if *item.value < 0 {
			return config, errors.New("http client durations must be non-negative")
		}
		if *item.value == 0 {
			*item.value = item.fallback
		}
	}
	if config.MaxResponseBytes < 0 || config.MaxResponseBytes == math.MaxInt64 {
		return config, errors.New("http client response limit must be non-negative and below MaxInt64")
	}
	if config.MaxResponseBytes == 0 {
		config.MaxResponseBytes = 4 << 20
	}
	for _, value := range []*int{&config.Transport.MaxIdleConns, &config.Transport.MaxIdleConnsPerHost, &config.Transport.MaxConnsPerHost} {
		if *value < 0 {
			return config, errors.New("http client connection limits must be non-negative")
		}
		if *value == 0 {
			*value = 100
		}
	}
	if config.Transport.MaxIdleConnsPerHost > config.Transport.MaxConnsPerHost {
		return config, errors.New("http client idle connections per host exceed total connections per host")
	}
	return config, nil
}
