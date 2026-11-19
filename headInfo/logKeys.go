package headInfo

import (
	"context"

	"github.com/K7mQ2vR9/base-pkg/consts"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/metadata"
)

// LogKeys 为 HTTP 和 RPC 日志共用的业务字段名单，请求期间只读复用。
// Trace ID 由 go-zero 从当前 Span 自动输出，不再重复记录请求 ID 别名。
var LogKeys = []string{
	"rpc_name",
	consts.ReqPath, consts.BusinessCode, consts.Business, consts.Source, consts.Version,
	consts.ClientIp, consts.AdminClientIp, consts.TokenUid, consts.TokenUidRole,
	consts.ContentLanguage, consts.Timezone, consts.UserAgent, consts.DeviceId, consts.OriginUrl,
}

// ContextMetadataInLog 按 LogKeys 选择 metadata 日志字段，保留每个字段的字符串列表格式。
func ContextMetadataInLog(ctx context.Context, md metadata.MD) context.Context {
	fields := make([]logx.LogField, 0, len(LogKeys))
	for _, key := range LogKeys {
		if values := md.Get(key); len(values) > 0 {
			fields = append(fields, logx.Field(key, values))
		}
	}
	return logx.ContextWithFields(ctx, fields...)
}
