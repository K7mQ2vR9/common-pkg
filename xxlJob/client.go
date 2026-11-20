package xxlJob

import (
	"context"
	"errors"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

var (
	// ErrStarted 表示已经开始启动，不能再次启动或修改任务注册。
	ErrStarted = errors.New("xxljob executor already started")
	// ErrStopped 表示执行器已停止或正在停止；重新启动须创建新实例。
	ErrStopped = errors.New("xxljob executor stopped")
)

type clientState uint8

const (
	ready clientState = iota
	starting
	running
	stopping
	stopped
)

// Client 由本包管理 HTTP、注册心跳和任务生命周期，不调用依赖 SDK 的 Init/Run/Stop。
// 一次实例只启动一次；Start 的 ctx 为整个服务生命周期，Stop 可并发和重复调用。
type Client struct {
	conf           Config
	mu             sync.Mutex
	state          clientState
	handlers       map[string]HandlerFun
	logHandler     LogHandler
	jobs           map[int64]*jobQueue
	active         map[int64]*execution
	tasks          sync.WaitGroup
	life           context.Context
	cancel         context.CancelFunc
	callbackLife   context.Context
	callbackCancel context.CancelFunc
	listener       net.Listener
	server         *http.Server
	startDone      chan struct{}
	heartbeatDone  chan struct{}
	serveDone      chan struct{}
	stopDone       chan struct{}
	stopErr        error
	serveErr       error
	registered     bool
	httpClient     *http.Client
	transport      *http.Transport
}

// NewClient 校验配置并创建执行器，不监听或发起网络请求；使用 Register 后调用 Start。
func NewClient(conf Config) (*Client, error) {
	conf, err := conf.normalized()
	if err != nil {
		return nil, err
	}
	// 管理请求连接池固定保留空闲连接 60s，不随心跳间隔相乘，避免大时长溢出。
	transport := &http.Transport{MaxIdleConns: 16, MaxIdleConnsPerHost: 8, IdleConnTimeout: 60 * time.Second, TLSHandshakeTimeout: conf.RequestTimeout, ResponseHeaderTimeout: conf.RequestTimeout, DialContext: (&net.Dialer{Timeout: conf.RequestTimeout}).DialContext}
	return &Client{conf: conf, handlers: make(map[string]HandlerFun), jobs: make(map[int64]*jobQueue), active: make(map[int64]*execution), stopDone: make(chan struct{}), transport: transport, httpClient: &http.Client{Transport: transport, Timeout: conf.RequestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// Register 在启动前原子注册一组任务，拒绝 nil、空名称、重复名称和 nil 函数，不再使用链式旧签名。
func (c *Client) Register(handlers ...Handler) error {
	if len(handlers) == 0 {
		return errors.New("xxljob handlers are required")
	}
	pending := make(map[string]HandlerFun, len(handlers))
	for _, handler := range handlers {
		if handler == nil {
			return errors.New("xxljob nil handler")
		}
		value := reflect.ValueOf(handler)
		switch value.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
			if value.IsNil() {
				return errors.New("xxljob nil handler")
			}
		}
		name, fn := handler.Pattern(), handler.Handler()
		if strings.TrimSpace(name) == "" || fn == nil {
			return errors.New("xxljob non-empty handler name and function are required")
		}
		if _, ok := pending[name]; ok {
			return errors.New("xxljob duplicate handler")
		}
		pending[name] = fn
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state >= stopping {
		return ErrStopped
	}
	if c.state != ready {
		return ErrStarted
	}
	for name := range pending {
		if _, ok := c.handlers[name]; ok {
			return errors.New("xxljob duplicate handler")
		}
	}
	for name, fn := range pending {
		c.handlers[name] = fn
	}
	return nil
}

// SetLogHandler 在启动前设置日志查询处理器，不持有或修改业务日志文件。
func (c *Client) SetLogHandler(handler LogHandler) error {
	if handler == nil {
		return errors.New("xxljob log handler is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state >= stopping {
		return ErrStopped
	}
	if c.state != ready {
		return ErrStarted
	}
	c.logHandler = handler
	return nil
}

// Start 先完成监听再向调度中心注册，首次注册成功才返回 nil；ctx 控制整个实例生命周期。
// 启动失败会关闭监听并完成有界清理；启动后的异步监听错误通过 Wait 返回，不退出进程。
func (c *Client) Start(ctx context.Context) (err error) {
	if ctx == nil {
		return errors.New("xxljob start context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.state >= stopping {
		c.mu.Unlock()
		return ErrStopped
	}
	if c.state != ready {
		c.mu.Unlock()
		return ErrStarted
	}
	c.state = starting
	c.life, c.cancel = context.WithCancel(ctx)
	c.callbackLife, c.callbackCancel = context.WithCancel(context.WithoutCancel(ctx))
	c.startDone = make(chan struct{})
	startDone := c.startDone
	c.mu.Unlock()
	defer func() {
		close(startDone)
		if err != nil {
			c.beginStop(err)
			<-c.stopDone
		}
	}()
	context.AfterFunc(c.life, func() { c.beginStop(nil) })
	listener, err := (&net.ListenConfig{}).Listen(c.life, "tcp", c.conf.ListenAddress)
	if err != nil {
		return &operationError{"listen", err}
	}
	c.mu.Lock()
	if c.state != starting || c.life.Err() != nil {
		c.mu.Unlock()
		_ = listener.Close()
		return ErrStopped
	}
	c.listener = listener
	c.server = &http.Server{Handler: c.router(), ReadHeaderTimeout: c.conf.RequestTimeout, ReadTimeout: c.conf.RequestTimeout, WriteTimeout: c.conf.RequestTimeout, IdleTimeout: 60 * time.Second, BaseContext: func(net.Listener) context.Context { return c.life }}
	c.serveDone = make(chan struct{})
	server := c.server
	go func() {
		defer close(c.serveDone)
		err := server.Serve(listener)
		c.mu.Lock()
		unexpected := c.state < stopping
		if unexpected && err != nil {
			c.serveErr = &operationError{"serve", err}
		}
		c.mu.Unlock()
		if unexpected {
			c.beginStop(c.serveErr)
		}
	}()
	c.registered = true // 注册请求可能已被接收，即使响应失败也在关闭时尝试摘除。
	c.mu.Unlock()
	if err = c.post(c.life, "registry", c.registration()); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != starting || c.life.Err() != nil {
		return ErrStopped
	}
	c.state = running
	c.heartbeatDone = make(chan struct{})
	go c.heartbeat()
	return nil
}

// Stop 立即拒绝新调度、关闭监听并取消任务，再等待整个收尾；调用方取消只中止等待，内部仍按 ShutdownTimeout 清理。
// 未启动时调用安全，首次 Stop 后不能重启；内部预算耗尽会强制关闭 HTTP 并取消回调，但不能终止忽略 ctx 的 Go 任务。
func (c *Client) Stop(ctx context.Context) error {
	if ctx == nil {
		return errors.New("xxljob stop context is required")
	}
	c.beginStop(nil)
	select {
	case <-c.stopDone:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.stopErr
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stopDone:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.stopErr
	}
}

// Wait 等待自动关闭或 Stop 完成，返回异步监听和关闭错误；ctx 取消只中止等待，不替代 Stop。
func (c *Client) Wait(ctx context.Context) error {
	if ctx == nil {
		return errors.New("xxljob wait context is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stopDone:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.stopErr
	}
}

func (c *Client) beginStop(cause error) {
	c.mu.Lock()
	if c.state >= stopping {
		c.mu.Unlock()
		return
	}
	if c.state == ready {
		c.state = stopped
		c.stopErr = cause
		c.transport.CloseIdleConnections()
		close(c.stopDone)
		c.mu.Unlock()
		return
	}
	c.state = stopping
	c.stopErr = errors.Join(c.stopErr, cause)
	cancel, listener := c.cancel, c.listener
	c.mu.Unlock()
	cancel()
	if listener != nil {
		_ = listener.Close()
	}
	go c.shutdown()
}

func (c *Client) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), c.conf.ShutdownTimeout)
	defer cancel()
	stopCallback := context.AfterFunc(ctx, c.callbackCancel)
	defer stopCallback()
	defer c.callbackCancel()
	var result error
	wait := func(done <-chan struct{}) bool {
		if done == nil {
			return true
		}
		select {
		case <-done:
			return true
		case <-ctx.Done():
			if !errors.Is(result, ctx.Err()) {
				result = errors.Join(result, ctx.Err())
			}
			return false
		}
	}
	c.mu.Lock()
	startDone := c.startDone
	c.mu.Unlock()
	wait(startDone)
	c.mu.Lock()
	server, heartbeatDone, serveDone, registered := c.server, c.heartbeatDone, c.serveDone, c.registered
	// 取消排队任务也要回调，但不能等待一个忽略取消的运行任务先退出。
	var pending []*execution
	for _, job := range c.jobs {
		pending = append(pending, job.queue...)
		job.queue = nil
	}
	c.mu.Unlock()
	for _, task := range pending {
		task.cancel()
		go c.complete(task, 500, "executor stopped")
	}
	httpDone := make(chan struct{})
	go func() {
		defer close(httpDone)
		if server != nil {
			if err := server.Shutdown(ctx); err != nil {
				_ = server.Close()
			}
		}
	}()
	// 必须等注册请求和心跳退出后再摘除，防止本地后台循环在摘除后重新注册。
	if wait(heartbeatDone) && registered && ctx.Err() == nil {
		result = errors.Join(result, c.post(ctx, "registryRemove", c.registration()))
	}
	tasksDone := make(chan struct{})
	go func() { c.tasks.Wait(); close(tasksDone) }()
	wait(httpDone)
	wait(serveDone)
	wait(tasksDone)
	if ctx.Err() != nil && server != nil {
		_ = server.Close()
	}
	if ctx.Err() != nil && !errors.Is(result, ctx.Err()) {
		result = errors.Join(result, ctx.Err())
	}
	c.callbackCancel()
	c.transport.CloseIdleConnections()
	c.mu.Lock()
	c.stopErr = errors.Join(c.stopErr, result)
	c.state = stopped
	close(c.stopDone)
	c.mu.Unlock()
}

func (c *Client) heartbeat() {
	defer close(c.heartbeatDone)
	timer := time.NewTicker(c.conf.RegistryInterval)
	defer timer.Stop()
	for {
		select {
		case <-c.life.Done():
			return
		case <-timer.C:
			if c.life.Err() != nil {
				return
			}
			if err := c.post(c.life, "registry", c.registration()); err != nil && c.life.Err() == nil {
				logc.Errorf(c.life, "XXL-JOB registry heartbeat failed: %v", err)
			}
		}
	}
}

// operationError 只输出处理阶段，保留底层错误链供 errors.Is/As，避免泄露令牌、地址及协议正文。
type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return "xxljob " + e.operation + " failed" }
func (e *operationError) Unwrap() error { return e.cause }
