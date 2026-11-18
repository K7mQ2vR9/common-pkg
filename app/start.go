package app

import (
	"fmt"

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

func WithAesKeyOption(aesKey string) SMOption {
	return func(s *ServerMiddleware) {
		s.aesKey = aesKey
	}
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
	aesKey  string
}

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
	s.Server.Use(appMiddleware.EnsureRequestTrace)
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
	var apiOption = []appMiddleware.ApiRequestDecryptOption{
		appMiddleware.DecryptKeyOption(s.aesKey),
	}
	if s.isDebug {
		apiOption = append(apiOption, appMiddleware.DecryptWithDebugFalseOption())
	}
	s.Server.Use(appMiddleware.NewApiRequestDecryptMiddleware(
		apiOption...,
	).Handle)
}
