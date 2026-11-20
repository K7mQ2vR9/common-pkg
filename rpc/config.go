package rpc

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

// Config 配置一个服务级共享 RPC 客户端；时长均为 time.Duration，零值采用注明的默认值。
type Config struct {
	// Target 为必填 gRPC 目标，例如 127.0.0.1:9000、dns:///service:9000 或 direct:///host:9000。
	Target string
	// RpcName 为必填的非空白可打印 ASCII 下游名称，写入 rpc_name metadata，不参与 Trace ID 生成。
	RpcName string
	// CallTimeout 为每次一元调用的总预算，默认 10s，包含等待连接和响应；父级更早截止时间优先，负值非法。
	CallTimeout time.Duration `json:",optional"`
	// ConnectTimeout 为启动连接等待预算，默认 5s，与启动 ctx 较早的截止时间共同生效；负值非法。
	ConnectTimeout time.Duration `json:",optional"`
	// NonBlock 默认 false，等待传输连接 Ready 后返回；true 只创建连接，不保证服务可用，调用仍受 CallTimeout 约束。
	NonBlock bool `json:",optional"`
	// Keepalive 独立配置 HTTP/2 PING，默认关闭，仅 Enabled=true 时应用其余字段。
	Keepalive KeepaliveConfig `json:",optional"`
}

// KeepaliveConfig 配置连接探测，须与服务端及代理允许的策略一致，不代表业务健康检查。
type KeepaliveConfig struct {
	// Enabled 默认 false，不主动开启客户端 HTTP/2 Keepalive。
	Enabled bool `json:",optional"`
	// Time 为没有传输活动后发送 PING 的间隔，零值默认 5m；启用时不得低于 gRPC 最小间隔 10s，负值非法。
	Time time.Duration `json:",optional"`
	// Timeout 为 PING 后等待响应的时限，零值默认 20s，负值非法。
	Timeout time.Duration `json:",optional"`
	// PermitWithoutStream 默认 false，无活跃 RPC 时不发送 PING；仅在服务端明确允许时开启。
	PermitWithoutStream bool `json:",optional"`
}

func (config Config) normalized() (Config, error) {
	if config.Target == "" || strings.ContainsFunc(config.Target, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return config, errors.New("RPC Target is required and must not contain whitespace or control characters")
	}
	if strings.TrimSpace(config.RpcName) == "" || strings.ContainsFunc(config.RpcName, func(r rune) bool { return r < 0x20 || r > 0x7e }) {
		return config, errors.New("RPC RpcName must contain non-blank printable ASCII text")
	}
	for _, item := range []struct {
		value    *time.Duration
		fallback time.Duration
	}{
		{&config.CallTimeout, 10 * time.Second},
		{&config.ConnectTimeout, 5 * time.Second},
		{&config.Keepalive.Time, 5 * time.Minute},
		{&config.Keepalive.Timeout, 20 * time.Second},
	} {
		if *item.value < 0 {
			return config, errors.New("RPC timeouts and intervals must be non-negative")
		}
		if *item.value == 0 {
			*item.value = item.fallback
		}
	}
	if config.Keepalive.Enabled && config.Keepalive.Time < 10*time.Second {
		return config, errors.New("RPC keepalive interval must be at least 10s")
	}
	return config, nil
}
