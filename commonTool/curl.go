package commonTool

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

var (
	// ErrHTTPClientClosed 表示工具客户端已关闭，后续请求不会发起网络调用。
	ErrHTTPClientClosed = errors.New("http client is closed")
	// ErrHTTPResponseTooLarge 表示正文超过配置上限，HTTPResponse.Body 仅保留上限内前缀。
	ErrHTTPResponseTooLarge = errors.New("http response body exceeds limit")
)

// HTTPStatusError 表示非 2xx 状态，使用 errors.As 识别；正文及响应头仍在返回的 HTTPResponse 中。
// 公开文本不包含 URL、凭证、响应正文或服务端自定义状态说明。
type HTTPStatusError struct {
	// StatusCode 为服务端原始 HTTP 状态码，包括未自动跟随的 3xx 重定向。
	StatusCode int
}

// Error 仅输出状态码，调用方按需检查返回的有界响应正文，不直接将其作为公开错误文案。
func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("http request returned status %d", e.StatusCode)
}

// HTTPResponse 为已读取并关闭的响应快照，无需调用方再 Close，响应头及正文由本次调用独立持有。
type HTTPResponse struct {
	// StatusCode 为服务端原始 HTTP 状态码。
	StatusCode int
	// Header 为响应头的独立副本，可能包含敏感字段，由业务控制使用方式。
	Header http.Header
	// Body 为完整正文或失败时已读取的前缀；出现错误时须同时检查 BodyComplete。
	Body []byte
	// BodyComplete 表示正文已读至 EOF 且未超限，不代表 HTTP 状态成功；关闭正文失败仍可能为 true。
	BodyComplete bool
}

// HTTPClient 可由多个请求并发复用；服务退出时调用 Close，不为每个业务请求重新建立连接池。
// GET/POST 都要求 ctx，重定向直接返回，不增加业务重试，也不自动重放 POST 正文。
type HTTPClient struct {
	client           *http.Client
	transport        *http.Transport
	timeout          time.Duration
	maxResponseBytes int64
	closed           atomic.Bool
}

// NewHTTPClient 校验并应用默认值，不发起网络请求；HTTPConfig{} 默认总时限 10s、正文上限 4MiB。
// HTTPS 使用系统证书验证和最低 TLS 1.2，代理沿用 HTTP_PROXY/HTTPS_PROXY/NO_PROXY 环境配置。
func NewHTTPClient(config HTTPConfig) (*HTTPClient, error) {
	config, err := config.normalized()
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: config.Transport.DialTimeout, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   config.Transport.TLSHandshakeTimeout,
		ResponseHeaderTimeout: config.Transport.ResponseHeaderTimeout,
		MaxIdleConns:          config.Transport.MaxIdleConns,
		MaxIdleConnsPerHost:   config.Transport.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.Transport.MaxConnsPerHost,
		IdleConnTimeout:       config.Transport.IdleConnTimeout,
		ForceAttemptHTTP2:     true,
	}
	result := &HTTPClient{transport: transport, timeout: config.Timeout, maxResponseBytes: config.MaxResponseBytes}
	result.client = &http.Client{Transport: &httpClientTransport{owner: result, base: transport}, Timeout: config.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return result, nil
}

// Get 在调用预算内读取 HTTP/HTTPS 资源，所有 2xx 都成功，包括 204 空正文。
// headers 在发送前复制；Host 可通过单一非空 Host 头指定，其余传输控制字段由标准库按正文处理。
func (c *HTTPClient) Get(ctx context.Context, url string, headers http.Header) (*HTTPResponse, error) {
	ctx, cancel, err := c.operationContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return c.do(ctx, http.MethodGet, url, nil, headers)
}

// PostJSON 将任意支持 JSON 的 Go 值编码后发送，默认 Content-Type 为 application/json，可由 headers 覆盖。
// 超时覆盖编码耗时和网络读取，编码失败或编码后预算耗尽不发送；不提供自动正文重放。
func (c *HTTPClient) PostJSON(ctx context.Context, url string, data any, headers http.Header) (*HTTPResponse, error) {
	ctx, cancel, err := c.operationContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	body, encodeErr := json.Marshal(data)
	if encodeErr != nil {
		encodeErr = &httpClientError{"encode JSON request", encodeErr}
	}
	if ctx.Err() != nil {
		return nil, errors.Join(ctx.Err(), encodeErr)
	}
	if encodeErr != nil {
		return nil, encodeErr
	}
	return c.do(ctx, http.MethodPost, url, body, headers)
}

// Close 可并发或重复调用，拒绝后续请求并释放空闲连接，不强制取消已经发起的请求。
// 服务应先停止新业务、等待请求退出再关闭；在途请求继续受自身 ctx 和总时限约束。
func (c *HTTPClient) Close() error {
	if c != nil && c.closed.CompareAndSwap(false, true) && c.transport != nil {
		c.transport.CloseIdleConnections()
	}
	return nil
}

func (c *HTTPClient) operationContext(parent context.Context) (context.Context, context.CancelFunc, error) {
	if parent == nil {
		return nil, nil, errors.New("http client context is required")
	}
	if c == nil || c.client == nil {
		return nil, nil, errors.New("http client must be initialized with NewHTTPClient")
	}
	if c.closed.Load() {
		return nil, nil, ErrHTTPClientClosed
	}
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	return ctx, cancel, nil
}

func (c *HTTPClient) do(ctx context.Context, method, url string, body []byte, headers http.Header) (result *HTTPResponse, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, &httpClientError{"prepare request", err}
	}
	if (request.URL.Scheme != "http" && request.URL.Scheme != "https") || request.URL.Hostname() == "" {
		return nil, &httpClientError{"prepare request", errors.New("HTTP/HTTPS URL with host is required")}
	}
	request.Header = make(http.Header, len(headers))
	// 规范大小写并复制多值切片，后续设置 Content-Type/Host 不得修改调用方的请求头。
	for key, values := range headers {
		canonical := http.CanonicalHeaderKey(key)
		request.Header[canonical] = append(request.Header[canonical], values...)
	}
	if values, ok := request.Header["Host"]; ok {
		if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			return nil, &httpClientError{"prepare headers", errors.New("Host must have a single non-empty value")}
		}
		request.Host = values[0]
		delete(request.Header, "Host")
	}
	if method == http.MethodPost {
		if request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", "application/json")
		}
		// 不提供已发送 POST 正文的重建函数，避免连接恢复时自动重放；业务重试自行构造下一次调用。
		request.GetBody = nil
	}
	response, sendErr := c.client.Do(request)
	if response != nil {
		result = &HTTPResponse{StatusCode: response.StatusCode, Header: response.Header.Clone()}
		if response.Body != nil {
			defer func() {
				if closeErr := response.Body.Close(); closeErr != nil {
					err = errors.Join(err, &httpClientError{"close response", closeErr})
				}
			}()
		}
	}
	if sendErr != nil {
		err = &httpClientError{"send request", sendErr}
		if ctx.Err() != nil {
			err = errors.Join(err, ctx.Err())
		}
		return result, err
	}
	if response == nil || response.Body == nil {
		return result, &httpClientError{"read response", errors.New("response body is missing")}
	}
	var statusErr error
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		statusErr = &HTTPStatusError{StatusCode: response.StatusCode}
	}
	payload, readErr := io.ReadAll(io.LimitReader(response.Body, c.maxResponseBytes+1))
	// 多读一个字节区分“正好达到上限”和“正文超限”，只将上限内数据交给业务。
	overflow := int64(len(payload)) > c.maxResponseBytes
	if overflow {
		payload = payload[:c.maxResponseBytes:c.maxResponseBytes]
	}
	result.Body = payload
	result.BodyComplete = readErr == nil && !overflow
	if readErr != nil {
		readErr = &httpClientError{"read response", readErr}
	}
	var sizeErr error
	if overflow {
		sizeErr = fmt.Errorf("%w (limit %d bytes)", ErrHTTPResponseTooLarge, c.maxResponseBytes)
	}
	// 状态、读取、超限与取消可同时发生，保留所有错误链，不能因读取失败丢失服务端状态。
	return result, errors.Join(statusErr, readErr, sizeErr, ctx.Err())
}

type httpClientTransport struct {
	owner *HTTPClient
	base  http.RoundTripper
}

func (t *httpClientTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	// 在真正进入网络层前复查关闭状态，覆盖准备上下文或编码 JSON 期间发生的 Close。
	if t.owner.closed.Load() {
		return nil, ErrHTTPClientClosed
	}
	return t.base.RoundTrip(request)
}

// httpClientError 隐藏可能包含凭证的 URL、请求头和 JSON 值，保留底层错误链供 errors.Is/As 检查。
type httpClientError struct {
	operation string
	cause     error
}

func (e *httpClientError) Error() string { return "http client " + e.operation + " failed" }
func (e *httpClientError) Unwrap() error { return e.cause }
