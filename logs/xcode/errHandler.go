package xcode

import (
	"context"

	"github.com/K7mQ2vR9/base-pkg/result"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
	"google.golang.org/grpc/status"
)

func ErrHandler(err error) (int, any) {
	return ErrHandlerCtx(context.Background(), err)
}

// ErrHandlerCtx 使用请求上下文中的 trace，为 httpx.ErrorCtx 构建错误响应。
func ErrHandlerCtx(ctx context.Context, err error) (int, any) {
	return result.HttpErrorResponse(ctx, err)
}

// FromError 使用统一规则构建 gRPC 状态，支持 nil 和包装后的错误。
func FromError(err error) *status.Status {
	return xcode.ToGRPCStatus(err)
}

// Deprecated : 使用 pkg/xcode/New 替换
func New(code int, msg string) xcode.Code {
	return xcode.New(code, msg)
}
