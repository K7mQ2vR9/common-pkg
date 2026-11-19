package dbM

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisStartupTimeout = 5 * time.Second

// RedisConfig 配置 Redis 的部署模式、地址、认证和 TLS。
type RedisConfig struct {
	// Type 默认 node，可选 cluster；Go 直接传参时空值也使用 node。
	Type string `json:",default=node,options=node|cluster"`
	// Addr 为 node 模式的必填地址，格式为 host:port。
	Addr string `json:",optional"`
	// Addrs 为 cluster 模式的种子地址列表，至少填写一个 host:port。
	Addrs []string `json:",optional"`
	// Username 为 ACL 用户名，默认空值，由服务端认证方式决定是否填写。
	Username string `json:",optional"`
	// Password 为认证密码，默认空值。
	Password string `json:",optional"`
	// DB 默认 0，node 模式允许非负值，cluster 模式使用 0。
	DB int `json:",optional"`
	// TLS 默认 false，开启后使用 TLS 并默认校验证书。
	TLS bool `json:",optional"`
	// InsecureSkipVerify 默认 false，仅在启用 TLS 时允许显式跳过证书校验。
	InsecureSkipVerify bool `json:",optional"`
}

// NewRedisClient 按部署模式创建客户端，启动 PING 最多等待 5 秒并继承调用方上下文。
// 启动失败或取消时关闭客户端，成功后由调用方在服务退出时调用 Close。
func NewRedisClient(ctx context.Context, config RedisConfig) (redis.UniversalClient, error) {
	if ctx == nil {
		return nil, errors.New("redis startup context is required")
	}
	ctx, cancel := context.WithTimeout(ctx, redisStartupTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	mode := strings.TrimSpace(config.Type)
	if mode == "" {
		mode = "node"
	}
	addrs, err := config.addresses(mode)
	if err != nil {
		return nil, err
	}
	if config.InsecureSkipVerify && !config.TLS {
		return nil, errors.New("redis InsecureSkipVerify requires TLS")
	}

	options := &redis.UniversalOptions{
		Addrs:                 addrs,
		Username:              config.Username,
		Password:              config.Password,
		DB:                    config.DB,
		ContextTimeoutEnabled: true,
	}
	if config.TLS {
		options.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: config.InsecureSkipVerify,
		}
		// 使用支持上下文取消的 TLS 拨号器，覆盖连接及握手过程。
		dialer := &tls.Dialer{
			NetDialer: &net.Dialer{Timeout: redisStartupTimeout},
			Config:    options.TLSConfig,
		}
		options.Dialer = dialer.DialContext
	}

	var client redis.UniversalClient
	if mode == "cluster" {
		client = redis.NewClusterClient(options.Cluster())
	} else {
		client = redis.NewClient(options.Simple())
	}

	// go-redis 的 I/O deadline 不会因 ctx 提前取消而立即更新。
	// 启动取消时关闭连接中断读取，PING 完成后撤销回调，交给调用方管理客户端。
	stopClose := context.AfterFunc(ctx, func() {
		_ = client.Close()
	})
	pingErr := client.Ping(ctx).Err()
	stopClose()
	if err := ctx.Err(); err != nil {
		pingErr = err
	}
	if pingErr != nil {
		_ = client.Close()
		if errors.Is(pingErr, context.Canceled) || errors.Is(pingErr, context.DeadlineExceeded) {
			return nil, fmt.Errorf("redis %s startup ping: %w", mode, pingErr)
		}
		return nil, &redisStartupError{mode: mode, cause: pingErr}
	}
	return client, nil
}

func (config RedisConfig) addresses(mode string) ([]string, error) {
	var addresses []string
	switch mode {
	case "node":
		if config.DB < 0 {
			return nil, errors.New("redis DB must be non-negative")
		}
		addresses = []string{config.Addr}
	case "cluster":
		if config.DB != 0 {
			return nil, errors.New("redis cluster requires DB 0")
		}
		if len(config.Addrs) == 0 {
			return nil, errors.New("redis cluster requires at least one seed address in Addrs")
		}
		addresses = config.Addrs
	default:
		return nil, errors.New("redis Type must be node or cluster")
	}

	normalized := make([]string, len(addresses))
	for i, address := range addresses {
		address = strings.TrimSpace(address)
		host, port, err := net.SplitHostPort(address)
		portNumber, portErr := strconv.ParseUint(port, 10, 16)
		if err != nil || host == "" || portErr != nil || portNumber == 0 {
			if mode == "node" {
				return nil, errors.New("redis Addr must be a host:port address")
			}
			return nil, fmt.Errorf("redis Addrs[%d] must be a host:port address", i)
		}
		normalized[i] = address
	}
	return normalized, nil
}

type redisStartupError struct {
	mode  string
	cause error
}

func (err *redisStartupError) Error() string {
	return "redis " + err.mode + " startup ping failed"
}

func (err *redisStartupError) Unwrap() error {
	return err.cause
}
