package headInfo

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/doubunv/common-pkg/consts"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	"github.com/zeromicro/go-zero/core/logx"
)

const PLATFORM_BUSINESS_CODE = "99999999"

type Head struct {
	AuthorizationJwt string `json:"authorization_jwt"` // 用户token
	Version          string `json:"version"`           // APP版本
	Source           string `json:"source"`            // 来源渠道
	ClientIp         string `json:"client_ip"`         // 客户端IP
	AdminClientIp    string `json:"admin_client_ip"`   // 管理后台域名商IP
	Trace            string `json:"trace"`             // 链路路由
	TokenUid         string `json:"token_uid"`         // 用户ID
	ReqPath          string `json:"req_path"`          // 请求path
	Business         string `json:"business"`
	BusinessCode     string `json:"business_code"`
	ContentLanguage  string `json:"content_language"`
	TokenUidRole     string `json:"token_uid_role"`
	ReqOrigin        string `json:"req_origin"` // 请求地址
	Timezone         string `json:"timezone"`
	UserAgent        string `json:"user_agent"`
	DeviceId         string `json:"device_id"`
}

func GetHead(r *http.Request) *Head {
	header := r.Header
	return &Head{
		AuthorizationJwt: strings.Trim(header.Get(consts.HeaderToken), " "),
		Version:          strings.Trim(header.Get(consts.Version), " "),
		Source:           strings.Trim(header.Get(consts.Source), " "),
		ClientIp:         getClientIP(r),
		AdminClientIp:    getAdminClientIPS(r),
		Trace:            getTraceId(r),
		ReqPath:          r.URL.Path,
		Business:         strings.Trim(header.Get(consts.Business), " "),
		BusinessCode:     strings.Trim(header.Get(consts.BusinessCode), " "),
		ContentLanguage:  strings.Trim(header.Get(consts.ContentLanguage), " "),
		TokenUid:         "",
		TokenUidRole:     "",
		ReqOrigin:        strings.Trim(header.Get("Origin"), " "),
		Timezone:         strings.Trim(header.Get(consts.Timezone), " "),
		UserAgent:        strings.Trim(header.Get("user-agent"), ""),
		DeviceId:         strings.Trim(header.Get(consts.DeviceId), ""),
	}
}

func (h *Head) Verify() error {
	return nil
}

func (h *Head) String() string {
	if h == nil {
		return "null"
	}

	// 字符串表示用于日志，清空副本中的凭证以保留原始认证数据。
	logHead := *h
	logHead.AuthorizationJwt = ""
	data, _ := json.Marshal(logHead)
	return string(data)
}

func ContextHeadInLog(ctx context.Context, h *Head) context.Context {
	ctxNew := logx.ContextWithFields(ctx,
		logx.Field(consts.Version, h.Version),
		logx.Field(consts.Source, h.Source),
		logx.Field(consts.ClientIp, h.ClientIp),
		logx.Field(consts.AdminClientIp, h.AdminClientIp),
		logx.Field(consts.Trace, h.Trace),
		logx.Field(consts.TokenUid, h.TokenUid),
		logx.Field(consts.ReqPath, h.ReqPath),
		logx.Field(consts.Business, h.Business),
		logx.Field(consts.BusinessCode, h.BusinessCode),
		logx.Field(consts.ContentLanguage, h.ContentLanguage),
		logx.Field(consts.TokenUidRole, h.TokenUidRole),
		logx.Field(consts.OriginUrl, h.ReqOrigin),
		logx.Field(consts.Timezone, h.Timezone),
		logx.Field(consts.UserAgent, h.UserAgent),
		logx.Field(consts.DeviceId, h.DeviceId),
	)
	return ctxNew
}

func getTraceId(r *http.Request) string {
	traceId := r.Header.Get("X-Request-ID")
	if traceId == "" {
		traceId = strings.Trim(r.Header.Get(consts.RequestId), " ")
	}
	if traceId == "" {
		traceId = trace.SpanContextFromContext(r.Context()).TraceID().String()
	}
	return traceId
}

// /////////////////////////////////////
var adminClientIPHeaders = []string{
	"CF-Connecting-IP", //CLOUDFLARE
}

func getAdminClientIP(r *http.Request) string {
	for _, header := range adminClientIPHeaders {
		if ip := getIPFromHeader(r, header); ip != "" {
			return ip
		}
	}
	return normalizeIP(r.RemoteAddr)
}

func getAdminClientIPS(r *http.Request) string {
	return r.Header.Get("X-Forwarded-For")
}

///////////////////////////////////////

var clientIPHeaders = []string{
	"CF-Connecting-IP", //CLOUDFLARE
	"TZ-Connecting-IP", //自定义
	"X-Real-IP",
	"X-Forwarded-For",
}

func getClientIP(r *http.Request) string {
	for _, header := range clientIPHeaders {
		if ip := getIPFromHeader(r, header); ip != "" {
			return ip
		}
	}
	return normalizeIP(r.RemoteAddr)
}

func getIPFromHeader(r *http.Request, header string) string {
	for _, value := range r.Header.Values(header) {
		// X-Forwarded-For 可能是：
		// client, proxy1, proxy2
		first, _, _ := strings.Cut(value, ",")

		if ip := normalizeIP(first); ip != "" {
			return ip
		}
	}

	return ""
}

func normalizeIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	// 处理 IPv4:port 和 [IPv6]:port。
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}

	// 兼容没有端口但包含方括号的 IPv6。
	value = strings.Trim(value, "[]")

	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}

	// 返回标准格式，同时处理 IPv4-mapped IPv6。
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}

	return ip.String()
}

// GetFullHead 返回原始请求头，结果可能包含认证凭证。
func GetFullHead(r *http.Request) map[string][]string {
	headers := make(map[string][]string)

	for k, v := range r.Header {
		headers[k] = v
	}

	return headers
}

func HeadInMetadata(ctx context.Context, h Head) context.Context {
	md := metadata.Pairs(
		consts.HeaderToken, h.AuthorizationJwt,
		consts.TokenUid, h.TokenUid,
		consts.ClientIp, h.ClientIp,
		consts.AdminClientIp, h.AdminClientIp,
		consts.ReqPath, h.ReqPath,
		consts.Version, h.Version,
		consts.Source, h.Source,
		consts.Business, h.Business,
		consts.BusinessCode, h.BusinessCode,
		consts.ContentLanguage, h.ContentLanguage,
		consts.TokenUidRole, h.TokenUidRole,
		consts.OriginUrl, h.ReqOrigin,
		consts.Timezone, h.Timezone,
		consts.UserAgent, h.UserAgent,
		consts.Trace, h.Trace,
		consts.DeviceId, h.DeviceId,
	)

	ctxNew := metadata.NewOutgoingContext(ctx, md)
	return ctxNew
}
