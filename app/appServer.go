package app

import (
	"github.com/K7mQ2vR9/base-pkg/aesGCM"
	rabbitmq "github.com/K7mQ2vR9/base-pkg/amqp/rabbitMq"
	"github.com/K7mQ2vR9/base-pkg/dbM"
	"github.com/K7mQ2vR9/base-pkg/logs"
	"github.com/K7mQ2vR9/base-pkg/logs/xcode"
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func init() {
}

func logInit() {
	writer := logs.NewLogrusWriter(func(logger *logrus.Logger) {
		logger.SetFormatter(&logrus.JSONFormatter{})
	})
	logx.SetWriter(writer)
}

func InitAppServer() {
	logInit()
	logx.DisableStat()
	httpx.SetErrorHandlerCtx(xcode.ErrHandlerCtx)
}

// ConfigAppServer 定义 API 和 RPC 可复用的应用配置，具体资源由服务初始化代码接入。
type ConfigAppServer struct {
	// IsDebug 为应用调试标记，默认 false，由服务启动代码决定如何使用。
	IsDebug bool `json:",optional,default=false"`
	// SelectDb 按 DbKey 配置多个 MySQL 连接池，每项 MySQL 使用独立的 MySQLConfig。
	// 列表未配置时为空，MySQLConfig 未填写的可选参数使用默认值。
	SelectDb []dbM.SelectDbConfig `json:",optional"`
	// RedisConf 配置 Redis 单机或集群，nil 表示未提供 Redis 配置。
	RedisConf *dbM.RedisConfig `json:",optional"`
	// RabbitMQ 配置消息队列，nil 表示未配置；由服务启动代码调用 NewRabbitMQ 并在退出时 Close。
	RabbitMQ *rabbitmq.Config `json:",optional"`
	// AESGCM 为进程共用的加解密配置，通过 WithAESGCM 接入，默认关闭。
	AESGCM aesGCM.Config `json:",optional"`
	// TrustedProxyCIDRs 指定可信代理网段，未配置时使用 HTTP 连接对端 IP。
	// 通过 WithTrustedProxyCIDRs 校验并接入后，才从可信代理提供的转发头解析客户端 IP。
	TrustedProxyCIDRs []string `json:",optional"`
	// MaxConcurrency 为单个 API Server 跨路由共享的并发请求数上限。
	// 默认 0 表示关闭，通过 WithMaxConcurrency 接入，负数会返回配置错误。
	MaxConcurrency int `json:",optional,default=0"`
}
