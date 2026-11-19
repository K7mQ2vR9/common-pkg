package appMiddleware

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"runtime/debug"

	"github.com/K7mQ2vR9/base-pkg/consts"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
	"github.com/zeromicro/go-zero/core/logc"
	ztrace "github.com/zeromicro/go-zero/core/trace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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

// Handle 要求全部调用携带 metadata 和有效 Trace ID，再检查白名单与认证。
// 业务错误统一编码为标准 gRPC 状态和错误详情。
func (m *RpcAuthMiddleware) Handle(getPathWhite PathWhiteHandle) grpc.UnaryServerInterceptor {
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
		ctx, err = rpcTraceContext(ctx, md)
		if err != nil {
			return nil, err
		}
		md.Set(consts.Trace, headInfo.GetTrace(ctx))
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

// rpcTraceContext 校验传入的 W3C 或兼容 trace 字段，保留调用方提供的 Trace ID。
// 仅 trace 字段存在时为本次接收生成 Span ID，所有链路字段均缺失时返回错误。
func rpcTraceContext(ctx context.Context, md metadata.MD) (context.Context, error) {
	parents := md.Get("traceparent")
	traces := md.Get(consts.Trace)
	if len(parents) > 1 || len(traces) > 1 {
		return ctx, status.Error(codes.InvalidArgument, "RPC trace metadata is invalid")
	}
	_, parent := ztrace.Extract(context.Background(), propagation.TraceContext{}, &md)
	if len(parents) == 1 && !parent.IsValid() {
		return ctx, status.Error(codes.InvalidArgument, "RPC trace metadata is invalid")
	}
	var traceID trace.TraceID
	if len(traces) == 1 {
		var err error
		traceID, err = trace.TraceIDFromHex(traces[0])
		if err != nil {
			return ctx, status.Error(codes.InvalidArgument, "RPC trace metadata is invalid")
		}
	}
	if parent.IsValid() {
		if traceID.IsValid() && traceID != parent.TraceID() {
			return ctx, status.Error(codes.InvalidArgument, "RPC trace metadata is invalid")
		}
		if headInfo.GetTrace(ctx) != parent.TraceID().String() {
			ctx = trace.ContextWithRemoteSpanContext(ctx, parent)
		}
		return ctx, nil
	}
	if !traceID.IsValid() {
		return ctx, status.Error(codes.InvalidArgument, "RPC trace ID is required")
	}
	if headInfo.GetTrace(ctx) == traceID.String() {
		return ctx, nil
	}
	var spanID trace.SpanID
	for !spanID.IsValid() {
		if _, err := io.ReadFull(rand.Reader, spanID[:]); err != nil {
			logc.Errorf(ctx, "initialize RPC span: %v", err)
			return ctx, status.Error(codes.Internal, "Internal Server Error")
		}
	}
	return trace.ContextWithRemoteSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})), nil
}
