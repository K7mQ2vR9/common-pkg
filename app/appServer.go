package app

import (
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

type RedisConf struct {
	Addr               string `json:",optional"`
	Username           string `json:",optional"`
	Password           string `json:",optional"`
	DB                 int    `json:",optional"`
	InsecureSkipVerify bool   `json:",optional"`
	Type               string `json:",default=node,options=node|cluster"`
}

type ConfigAppServer struct {
	IsDebug           bool                 `json:",optional,default=false"`
	SelectDb          []dbM.SelectDbConfig `json:",optional"`
	RedisConf         *RedisConf           `json:",optional"`
	TrustedProxyCIDRs []string             `json:",optional"`
	MaxConcurrency    int                  `json:",optional,default=0"`
}
