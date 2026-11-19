package appMiddleware

import (
	"context"
	"errors"
	"runtime/debug"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
	"github.com/zeromicro/go-zero/core/logc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// CheckRPCRequestFunc 校验 RPC 调用方，ctx 中可读取 metadata 或传输层身份。
// 返回 nil 表示通过，拒绝认证及权限不足可返回对应的标准 gRPC 状态。
type CheckRPCRequestFunc func(ctx context.Context, fullMethod string) error

// RpcAuthMiddleware 在链路上下文就绪后校验受保护方法的调用方。
type RpcAuthMiddleware struct {
	check CheckRPCRequestFunc
}

// PathWhiteHandle 返回公开方法列表，支持精确路径和以 * 结尾的路径前缀。
type PathWhiteHandle func(ctx context.Context) map[string]int

// NewRpcAuthMiddleware 校验认证回调，缺少回调时返回初始化错误。
func NewRpcAuthMiddleware(check CheckRPCRequestFunc) (*RpcAuthMiddleware, error) {
	if check == nil {
		return nil, errors.New("RPC authentication checker is required")
	}
	return &RpcAuthMiddleware{check: check}, nil
}

// ServerOption 在原生 gRPC 链路拦截器前校验 W3C 上下文，缺失或非法链路直接拒绝。
// go-zero 使用 server.AddOptions(auth.ServerOption(white))；同一 Server 仅设置一次主 UnaryInterceptor。
func (m *RpcAuthMiddleware) ServerOption(getPathWhite PathWhiteHandle) grpc.ServerOption {
	return grpc.UnaryInterceptor(m.handle(getPathWhite))
}

// handle 要求全部调用携带 metadata 和有效 W3C 链路，再检查白名单与认证。
// 业务错误统一编码为标准 gRPC 状态和错误详情。
func (m *RpcAuthMiddleware) handle(getPathWhite PathWhiteHandle) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				logc.Errorf(ctx, "RPC panic: %v\n%s", p, debug.Stack())
				resp = nil
				err = status.Error(codes.Internal, "Internal Server Error")
			}
		}()

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok || len(md) == 0 {
			return nil, status.Error(codes.InvalidArgument, "RPC metadata is required")
		}
		md = md.Copy()
		ctx, err = headInfo.ExtractTrace(ctx, md)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		ctx = metadata.NewIncomingContext(ctx, md.Copy())
		ctx = metadata.NewOutgoingContext(ctx, md.Copy())
		ctx = headInfo.ContextMetadataInLog(ctx, md)
		if err := ctx.Err(); err != nil {
			return nil, xcode.ToGRPCStatus(err).Err()
		}

		var white map[string]int
		if getPathWhite != nil {
			white = getPathWhite(ctx)
		}
		if !isWhitelistedPath(info.FullMethod, white) {
			if m.check == nil {
				logc.Error(ctx, "RPC authentication checker is missing")
				return nil, status.Error(codes.Internal, "Internal Server Error")
			}
			if authErr := m.check(ctx, info.FullMethod); authErr != nil {
				logc.Errorf(ctx, "RPC authentication failed: %v", authErr)
				authStatus := xcode.ToGRPCStatus(authErr)
				switch authStatus.Code() {
				case codes.PermissionDenied:
					return nil, status.Error(codes.PermissionDenied, "No permission")
				case codes.Canceled, codes.DeadlineExceeded, codes.Unavailable:
					return nil, authStatus.Err()
				default:
					return nil, status.Error(codes.Unauthenticated, "Authentication failed.")
				}
			}
		}

		resp, err = handler(ctx, req)
		if err != nil {
			st := xcode.ToGRPCStatus(err)
			if st.Code() == codes.Internal || st.Code() == codes.Unknown || st.Code() == codes.DataLoss {
				logc.Errorf(ctx, "RPC request failed: %v", err)
			}
			return nil, st.Err()
		}
		return resp, nil
	}
}
