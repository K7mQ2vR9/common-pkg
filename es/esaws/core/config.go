package core

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config 独立配置 OpenSearch，不与 Elasticsearch 7 共用配置或类型别名。
type Config struct {
	// Addresses 必填，使用 http/https 节点地址；认证信息通过 Username/Password 配置。
	Addresses []string `json:",optional"`
	// Username 和 Password 为可选 Basic Auth 凭证，均为空表示不发送认证头。
	Username string `json:",optional"`
	Password string `json:",optional"`
	// StartupTimeout 为初始化探活总时限，包含重试和读取正文；零值默认 5s，负值非法。
	StartupTimeout time.Duration `json:",optional"`
	// RequestTimeout 为模型单次操作总时限，包含重试和读取正文；零值默认 10s，负值非法。
	RequestTimeout time.Duration `json:",optional"`
	// MaxRetries 表示首次请求之外的 SDK 重试次数，未配置默认 3，显式 0 关闭重试，负值非法。
	// 沿用 SDK 默认网络错误及 502/503/504 重试触发条件，不额外设置不可取消的退避，写操作须考虑结果不确定和幂等。
	MaxRetries *int `json:",optional"`
	// TLS 配置 HTTPS 证书验证，缺省使用系统信任证书并验证主机名，最低 TLS 1.2。
	TLS TLSConfig `json:",optional"`
	// Transport 配置独立连接池和阶段超时，零值采用各字段默认值。
	Transport TransportConfig `json:",optional"`
}

// TLSConfig 仅用于 OpenSearch；HTTP 地址不执行 TLS 握手。
type TLSConfig struct {
	// CAFile 为可选 PEM 证书文件，追加到系统信任证书；无法读取或无有效证书时初始化失败。
	CAFile string `json:",optional"`
	// InsecureSkipVerify 默认 false；仅测试环境按需开启，开启后不验证证书链和主机名。
	InsecureSkipVerify bool `json:",optional"`
}

// TransportConfig 时长单位为 time.Duration，零值采用默认值，负数非法。
type TransportConfig struct {
	// DialTimeout 为 TCP 建连时限，默认 5s。
	DialTimeout time.Duration `json:",optional"`
	// TLSHandshakeTimeout 为 TLS 握手时限，默认 5s。
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

func normalizeConfig(conf *Config) (Config, int, error) {
	if conf == nil || len(conf.Addresses) == 0 {
		return Config{}, 0, errors.New("opensearch Addresses are required")
	}
	c := *conf
	c.Addresses = append([]string(nil), conf.Addresses...)
	for _, address := range c.Addresses {
		u, err := url.Parse(address)
		if err != nil || strings.TrimSpace(address) != address || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return Config{}, 0, errors.New("opensearch Addresses must be http/https URLs without credentials, query or fragment")
		}
	}
	retries := 3
	if c.MaxRetries != nil {
		retries = *c.MaxRetries
	}
	if retries < 0 {
		return Config{}, 0, errors.New("opensearch MaxRetries must be non-negative")
	}
	for _, item := range []struct {
		value    *time.Duration
		fallback time.Duration
	}{
		{&c.StartupTimeout, 5 * time.Second}, {&c.RequestTimeout, 10 * time.Second},
		{&c.Transport.DialTimeout, 5 * time.Second}, {&c.Transport.TLSHandshakeTimeout, 5 * time.Second},
		{&c.Transport.ResponseHeaderTimeout, 5 * time.Second}, {&c.Transport.IdleConnTimeout, 90 * time.Second},
	} {
		if *item.value < 0 {
			return Config{}, 0, errors.New("opensearch durations must be non-negative")
		}
		if *item.value == 0 {
			*item.value = item.fallback
		}
	}
	for _, value := range []*int{&c.Transport.MaxIdleConns, &c.Transport.MaxIdleConnsPerHost, &c.Transport.MaxConnsPerHost} {
		if *value < 0 {
			return Config{}, 0, errors.New("opensearch connection limits must be non-negative")
		}
		if *value == 0 {
			*value = 100
		}
	}
	if c.Transport.MaxIdleConnsPerHost > c.Transport.MaxConnsPerHost {
		return Config{}, 0, errors.New("opensearch idle connections per host exceed total connections per host")
	}
	return c, retries, nil
}

func makeTLSConfig(c TLSConfig) (*tls.Config, error) {
	result := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: c.InsecureSkipVerify}
	if c.CAFile == "" {
		return result, nil
	}
	pem, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, &operationError{"load CA certificate", err}
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, &operationError{"load system certificates", err}
	}
	if roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("opensearch CAFile contains no valid certificates")
	}
	result.RootCAs = roots
	return result, nil
}
