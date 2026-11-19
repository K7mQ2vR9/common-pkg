package app

import (
	"fmt"

	"github.com/K7mQ2vR9/base-pkg/aesGCM"
	"github.com/K7mQ2vR9/base-pkg/appMiddleware"
	"github.com/zeromicro/go-zero/rest"
)

type SMOption func(s *ServerMiddleware)

func WithWhiteHeaderPathSMOption(whiteHeader map[string]int) SMOption {
	return func(s *ServerMiddleware) {
		s.whiteHeader = whiteHeader
	}
}

func WithDebugOption() SMOption {
	return func(s *ServerMiddleware) {
		s.isDebug = true
	}
}

func WithTestOption() SMOption {
	return func(s *ServerMiddleware) {
		s.isTest = true
	}
}

// WithAESGCM 校验进程级加解密配置，在 ApiUseMiddleware 装配时应用，默认关闭。
func WithAESGCM(config aesGCM.Config) (SMOption, error) {
	if _, err := aesGCM.New(config); err != nil {
		return nil, err
	}
	return func(s *ServerMiddleware) {
		s.aesConfig = config
	}, nil
}

// WithMaxConcurrency sets the shared per-server request limit. Zero disables it.
func WithMaxConcurrency(max int) (SMOption, error) {
	if max < 0 {
		return nil, fmt.Errorf("MaxConcurrency must be non-negative, got %d", max)
	}
	return func(s *ServerMiddleware) {
		s.maxConcurrency = max
	}, nil
}

func WithCheckTokenHandleSMOption(fun appMiddleware.CheckRequestTokenFunc) SMOption {
	return func(s *ServerMiddleware) {
		s.checkTokenHandle = fun
		s.authOption = nil
	}
}

// WithCheckTokenHandleWithErrSMOption 启用可返回错误的认证回调。
func WithCheckTokenHandleWithErrSMOption(fun appMiddleware.CheckRequestTokenFuncWithErr) SMOption {
	return withAuthOption(appMiddleware.WithCheckOptionWithErr(fun))
}

// WithCheckTokenHandleWithUserInfoSMOption 启用可返回登录用户 JSON 信息的认证回调。
func WithCheckTokenHandleWithUserInfoSMOption(fun appMiddleware.CheckRequestTokenFuncWithUserInfo) SMOption {
	return withAuthOption(appMiddleware.WithCheckOptionWithUserInfo(fun))
}

func withAuthOption(option appMiddleware.AuthMiddlewareOption) SMOption {
	return func(s *ServerMiddleware) {
		s.authOption = option
		s.checkTokenHandle = nil
	}
}

// WithTrustedProxyCIDRs 设置 HTTP 中间件信任的代理网段。
func WithTrustedProxyCIDRs(cidrs []string) (SMOption, error) {
	option, err := appMiddleware.WithTrustedProxyCIDRs(cidrs)
	if err != nil {
		return nil, err
	}
	return func(s *ServerMiddleware) {
		s.trustedProxyOption = option
	}, nil
}

type ServerMiddleware struct {
	whiteHeader        map[string]int
	checkTokenHandle   appMiddleware.CheckRequestTokenFunc
	authOption         appMiddleware.AuthMiddlewareOption
	trustedProxyOption appMiddleware.ApiHeadOption
	maxConcurrency     int
	Server             *rest.Server

	isDebug bool
	isTest  bool
	// aesConfig 在 API 装配时应用为进程共用的加解密配置。
	aesConfig aesGCM.Config
}

// NewServerMiddleware 装配应用中间件；Server 用 NewServer 创建，以便在原生 Span 前初始化统一 ID。
func NewServerMiddleware(s *rest.Server, opt ...SMOption) *ServerMiddleware {
	res := &ServerMiddleware{
		Server: s,
	}

	for _, item := range opt {
		item(res)
	}

	return res
}

func (s *ServerMiddleware) ApiUseMiddleware() {
	authMiddleware := s.authMiddleware()
	s.Server.Use(appMiddleware.NewCorsMiddleware().Handle)
	if s.maxConcurrency > 0 {
		s.Server.Use(appMiddleware.NewConcurrencyLimiter(s.maxConcurrency).Handle)
	}
	s.useApiRequestDecrypt()
	s.useApiHeaderMiddleware()
	s.Server.Use(authMiddleware)
}

func (s *ServerMiddleware) useApiHeaderMiddleware() {
	var apiHeaderOption = []appMiddleware.ApiHeadOption{
		appMiddleware.CloseVerifyOption(s.whiteHeader),
	}
	if s.trustedProxyOption != nil {
		apiHeaderOption = append(apiHeaderOption, s.trustedProxyOption)
	}
	if s.isDebug {
		apiHeaderOption = append(apiHeaderOption, appMiddleware.WithDebugOption())
	}
	s.Server.Use(appMiddleware.NewApiHeaderMiddleware(
		apiHeaderOption...,
	).Handle)
}

func (s *ServerMiddleware) authMiddleware() rest.Middleware {
	if s.authOption != nil {
		return appMiddleware.NewUserAuthMiddlewareWithErr(s.whiteHeader, s.authOption).Handle
	}
	if s.checkTokenHandle == nil {
		panic("token checker is required: use an app token checker option")
	}

	return appMiddleware.NewUserAgentMiddleware(
		s.whiteHeader,
		appMiddleware.WithCheckOption(s.checkTokenHandle),
	).Handle
}

func (s *ServerMiddleware) useApiRequestDecrypt() {
	middleware, err := appMiddleware.NewApiRequestDecryptMiddleware(s.aesConfig)
	if err != nil {
		panic(fmt.Errorf("invalid AES-GCM configuration: %w", err))
	}
	s.Server.Use(middleware.Handle)
}
