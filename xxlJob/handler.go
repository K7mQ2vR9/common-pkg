package xxlJob

import (
	"context"
	"github.com/xxl-job/xxl-job-executor-go"
)

// HandlerFun 保留现有 Go 任务签名；正常返回的字符串作为成功回调内容。
// ctx 随关闭、kill、覆盖策略和调度超时取消；即使随后正常返回，也回调失败，业务 I/O 须传递 ctx。
type HandlerFun func(context.Context, *xxl.RunReq) string

// Handler 在启动前注册，Pattern 必须非空且唯一，Handler 必须返回非 nil 函数。
type Handler interface {
	Handler() HandlerFun
	Pattern() string
}

// LogHandler 可选地为调度中心提供日志查询，ctx 继承 HTTP 请求的取消；返回值是日志内容而非外层响应。
// 未设置时明确告知未配置日志查询，不伪造业务日志；仅在启动前通过 SetLogHandler 设置。
type LogHandler func(context.Context, *xxl.LogReq) (*xxl.LogResContent, error)
