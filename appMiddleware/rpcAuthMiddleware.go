package appMiddleware

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/K7mQ2vR9/base-pkg/consts"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
	"github.com/zeromicro/go-zero/core/logc"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type RpcAuthMiddleware struct {
	Debug bool
}

type PathWhiteHandle func(ctx context.Context) map[string]int

func NewRpcAuthMiddleware() *RpcAuthMiddleware {
	return &RpcAuthMiddleware{}
}

// contextMetadataInLog 只写入已明确用于排查请求的字段，原始 metadata 继续供认证使用。
func contextMetadataInLog(ctx context.Context, md metadata.MD) context.Context {
	logKeys := []string{
		consts.Trace, consts.RequestId, "x-request-id", "traceparent", "rpc_name",
		consts.ReqPath, consts.BusinessCode, consts.Business, consts.Source, consts.Version,
		consts.ClientIp, consts.AdminClientIp, consts.TokenUid, consts.TokenUidRole,
		consts.ContentLanguage, consts.Timezone, consts.UserAgent, consts.DeviceId, consts.OriginUrl,
	}
	fields := make([]logx.LogField, 0, len(logKeys))
	for _, key := range logKeys {
		if values := md.Get(key); len(values) > 0 {
			fields = append(fields, logx.Field(key, values))
		}
	}

	return logx.ContextWithFields(ctx, fields...)
}

func (m *RpcAuthMiddleware) Handle(getPathWhite PathWhiteHandle) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				logc.Error(ctx, err, string(debug.Stack()))
				err = xcode.NewRpc(http.StatusInternalServerError, "Rpc Server error.")
				return
			}
		}()

		mdData, ok := metadata.FromIncomingContext(ctx)
		if ok {
			ctx = metadata.NewOutgoingContext(ctx, mdData)
			ctx = contextMetadataInLog(ctx, mdData)
			if headInfo.GetBusinessCode(ctx) == "" {
				if getPathWhite == nil {
					return nil, errors.New("Rpc header data err")
				}
				whitePath := getPathWhite(ctx)
				if _, okPath := whitePath[info.FullMethod]; !okPath {
					return nil, errors.New("Rpc header data err")
				}
			}
		}

		resp, err = handler(ctx, req)
		return resp, err
	}
}
