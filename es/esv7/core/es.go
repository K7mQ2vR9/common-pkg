package core

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	es7 "github.com/elastic/go-elasticsearch/v7"
)

// ErrClosed 表示 Elasticsearch 7 客户端已经关闭，后续请求被拒绝。
var ErrClosed = errors.New("elasticsearch7 client is closed")

// Es 持有独立 Elasticsearch 7 SDK 和连接池，可由多个请求共享，服务退出时调用 Close。
// 直接使用嵌入 SDK 须自行提供请求 ctx、截止时间并关闭响应体；模型操作自动管理这些资源。
type Es struct {
	*es7.Client
	transport      *http.Transport
	requestTimeout time.Duration
	closed         atomic.Bool
}

// NewEs 校验配置后，在 ctx 和 StartupTimeout 中较早的截止时间前完成集群 Info 探活。
// 初始化上下文取消不影响已创建客户端；初始化失败释放空闲连接，成功后由服务统一 Close。
func NewEs(ctx context.Context, conf *Config) (*Es, error) {
	if ctx == nil {
		return nil, errors.New("elasticsearch7 startup context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, retries, err := normalizeConfig(conf)
	if err != nil {
		return nil, err
	}
	startup, cancel := context.WithTimeout(ctx, c.StartupTimeout)
	defer cancel()
	tlsConfig, err := makeTLSConfig(c.TLS)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		TLSClientConfig:       tlsConfig,
		MaxIdleConns:          c.Transport.MaxIdleConns,
		MaxIdleConnsPerHost:   c.Transport.MaxIdleConnsPerHost,
		MaxConnsPerHost:       c.Transport.MaxConnsPerHost,
		IdleConnTimeout:       c.Transport.IdleConnTimeout,
		TLSHandshakeTimeout:   c.Transport.TLSHandshakeTimeout,
		ResponseHeaderTimeout: c.Transport.ResponseHeaderTimeout,
		DialContext:           (&net.Dialer{Timeout: c.Transport.DialTimeout, KeepAlive: 30 * time.Second}).DialContext,
	}
	result := &Es{transport: transport, requestTimeout: c.RequestTimeout}
	client, err := es7.NewClient(es7.Config{
		Addresses: c.Addresses, Username: c.Username, Password: c.Password,
		MaxRetries: retries, DisableRetry: retries == 0,
		Transport: &clientTransport{owner: result, base: transport, disableRetry: retries == 0},
	})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, &operationError{"initialize client", err}
	}
	result.Client = client
	response, err := client.Info(client.Info.WithContext(startup))
	if err == nil {
		err = CheckResponse(startup, response)
	} else if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		_ = result.Close()
		return nil, &operationError{"startup check", err}
	}
	return result, nil
}

// MustNewEs 用于启动时必须成功的场景，失败时 panic；可恢复错误使用 NewEs。
func MustNewEs(ctx context.Context, conf *Config) *Es {
	client, err := NewEs(ctx, conf)
	if err != nil {
		panic(err)
	}
	return client
}

// OperationContext 为一次模型操作派生总时限，包含 SDK 重试和响应读取；父级更早的截止时间优先。
// 调用方在完成响应处理后调用 cancel，不用于操作之外的后台任务。
func (e *Es) OperationContext(parent context.Context) (context.Context, context.CancelFunc, error) {
	if parent == nil {
		return nil, nil, errors.New("elasticsearch7 request context is required")
	}
	if e == nil || e.Client == nil {
		return nil, nil, errors.New("elasticsearch7 client is required")
	}
	if e.closed.Load() {
		return nil, nil, ErrClosed
	}
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(parent, e.requestTimeout)
	return ctx, cancel, nil
}

// Close 可重复或并发调用，拒绝后续请求并释放空闲连接，不强制取消在途请求。
// 服务先停止新任务并等待业务退出，再关闭客户端；在途请求仍由自身 ctx 取消。
func (e *Es) Close() error {
	if e != nil && e.closed.CompareAndSwap(false, true) && e.transport != nil {
		e.transport.CloseIdleConnections()
	}
	return nil
}

type clientTransport struct {
	owner        *Es
	base         http.RoundTripper
	disableRetry bool
}

func (t *clientTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.owner.closed.Load() {
		return nil, ErrClosed
	}
	response, err := t.base.RoundTrip(request)
	// 当前 SDK 对直接 io.EOF 的重试不检查 DisableRetry；包装后保留错误链并阻止该特例重试。
	if t.disableRetry && err == io.EOF {
		return response, &operationError{"round trip", err}
	}
	return response, err
}

// operationError 公开文本只包含阶段，底层错误供 errors.Is/As 检查，避免输出凭证或原始请求。
type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return "elasticsearch7 " + e.operation + " failed" }
func (e *operationError) Unwrap() error { return e.cause }
