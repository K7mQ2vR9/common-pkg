package interceptors

import (
	"context"

	"github.com/K7mQ2vR9/base-pkg/consts"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
	"github.com/zeromicro/go-zero/core/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ClientInterceptor 复制调用 metadata 并传递链路，按统一规则恢复 RPC 业务错误。
func ClientInterceptor(rpcName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, err := headInfo.EnsureTrace(ctx)
		if err != nil {
			return err
		}
		md, _ := metadata.FromOutgoingContext(ctx)
		md = md.Copy()
		md.Set("rpc_name", rpcName)
		md.Set(consts.Trace, headInfo.GetTrace(ctx))
		trace.Inject(ctx, otel.GetTextMapPropagator(), &md)
		trace.Inject(ctx, propagation.TraceContext{}, &md)
		ctx = metadata.NewOutgoingContext(ctx, md)
		err = invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			return nil
		}
		return xcode.FromGRPCError(err)
	}
}

func RetryDialOption() grpc.DialOption {
	retryPolicy := `{
		"methodConfig": [{
		  "retryPolicy": {
			  "MaxAttempts": 3,
			  "InitialBackoff": "1s",
			  "MaxBackoff": "1s",
			  "BackoffMultiplier": 1.0,
			  "RetryableStatusCodes": [ "UNAVAILABLE" ]
		  }
		}]}`

	return grpc.WithDefaultServiceConfig(retryPolicy)
}
