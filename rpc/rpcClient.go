package rpc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/K7mQ2vR9/base-pkg/rpc/interceptors"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/keepalive"
)

// ErrClientClosed 表示 RPC 客户端已关闭，后续一元调用直接拒绝。
var ErrClientClosed = errors.New("RPC client is closed")

// Client 持有共享 gRPC 连接，实现 zrpc.Client，供生成的业务客户端使用 Conn()。
// 初始化 ctx 仅控制启动过程；服务退出时调用 Close，关闭会取消仍在途的 RPC。
type Client struct {
	conn        *grpc.ClientConn
	callTimeout time.Duration
	closed      atomic.Bool
	closeOnce   sync.Once
	closeErr    error
}

var _ zrpc.Client = (*Client)(nil)

// NewClient 校验配置并在启动 ctx/ConnectTimeout 中较早的截止时间前等待连接 Ready，默认最多 5s。
// NonBlock=true 时不等待 Ready；启动失败释放连接，成功后的 ctx 取消不关闭实例。
// 复用 go-zero 目标解析与负载均衡，显式设置一元调用预算，保留现有链路及业务错误映射。
func NewClient(ctx context.Context, conf Config) (*Client, error) {
	if ctx == nil {
		return nil, errors.New("RPC startup context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	config, err := conf.normalized()
	if err != nil {
		return nil, err
	}
	startup, cancel := context.WithTimeout(ctx, config.ConnectTimeout)
	defer cancel()
	result := &Client{callTimeout: config.CallTimeout}
	options := []zrpc.ClientOption{
		zrpc.WithUnaryClientInterceptor(result.callBudgetInterceptor()),
		zrpc.WithUnaryClientInterceptor(interceptors.ClientInterceptor(config.RpcName)),
		// 禁止服务配置驱动的策略重试，不再注入固定三次 UNAVAILABLE 策略；gRPC 透明恢复仍由底层控制。
		zrpc.WithDialOption(grpc.WithDisableRetry()),
	}
	if config.Keepalive.Enabled {
		options = append(options, zrpc.WithDialOption(grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time: config.Keepalive.Time, Timeout: config.Keepalive.Timeout,
			PermitWithoutStream: config.Keepalive.PermitWithoutStream,
		})))
	}
	// go-zero v1.7.4 内部阻塞拨号固定 3s，先非阻塞创建，再由当前启动上下文负责等待和失败关闭。
	underlying, err := zrpc.NewClient(zrpc.RpcClientConf{Target: config.Target, NonBlock: true}, options...)
	if err != nil {
		return nil, &clientError{"create connection", err}
	}
	result.conn = underlying.Conn()
	if !config.NonBlock {
		err = waitReady(startup, result.conn)
	}
	if err == nil {
		err = startup.Err()
	}
	if err != nil {
		return nil, errors.Join(&clientError{"wait for connection", err}, result.Close())
	}
	return result, nil
}

// MustNewClient 复用 NewClient，失败时 panic；需要处理启动错误时使用 NewClient，不调用进程退出函数。
func MustNewClient(ctx context.Context, conf Config) *Client {
	client, err := NewClient(ctx, conf)
	if err != nil {
		panic(err)
	}
	return client
}

// Conn 返回共享底层连接，nil/零值实例返回 nil；生成的 RPC 客户端使用该连接，不自行重新拨号。
// 优先由 Client.Close 管理生命周期，关闭后仍返回原连接供检查其 Shutdown 状态。
func (c *Client) Conn() *grpc.ClientConn {
	if c == nil {
		return nil
	}
	return c.conn
}

// Close 可并发或重复调用，释放共享连接并取消在途 RPC，nil/零值实例安全。
// 服务须先停止新业务并按需要等待请求收尾；Client 不执行后台业务排空。
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		if c.conn != nil {
			c.closeErr = c.conn.Close()
			if errors.Is(c.closeErr, grpc.ErrClientConnClosing) {
				c.closeErr = nil
			}
		}
	})
	return c.closeErr
}

func (c *Client) callBudgetInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if c.closed.Load() {
			return ErrClientClosed
		}
		// 每次调用创建独立预算，保留父级上下文的值、取消和较早的截止时间，不受 go-zero 中间件零值影响。
		call, cancel := context.WithTimeout(ctx, c.callTimeout)
		defer cancel()
		return invoker(call, method, req, reply, cc, opts...)
	}
}

func waitReady(ctx context.Context, conn *grpc.ClientConn) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return nil
		case connectivity.Shutdown:
			return ErrClientClosed
		case connectivity.Idle:
			conn.Connect()
		}
		// 状态等待由 ctx 取消，不额外创建后台轮询或等待 goroutine。
		conn.WaitForStateChange(ctx, state)
	}
}

// clientError 隐藏可能包含凭证的目标地址和底层诊断文本，保留错误链用于 errors.Is/As。
type clientError struct {
	operation string
	cause     error
}

func (e *clientError) Error() string { return "RPC client " + e.operation + " failed" }
func (e *clientError) Unwrap() error { return e.cause }
