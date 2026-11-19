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

type RedisConfig struct {
	Type               string   `json:",default=node,options=node|cluster"`
	Addr               string   `json:",optional"`
	Addrs              []string `json:",optional"`
	Username           string   `json:",optional"`
	Password           string   `json:",optional"`
	DB                 int      `json:",optional"`
	TLS                bool     `json:",optional"`
	InsecureSkipVerify bool     `json:",optional"`
}

// NewRedisClient connects to the configured server and checks it with PING.
// The caller owns the returned client and closes it at shutdown.
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
		// The default TLS dialer in go-redis v9.7.0 does not use the caller context.
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

	pingErr := client.Ping(ctx).Err()
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
