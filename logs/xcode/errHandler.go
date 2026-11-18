package xcode

import (
	"context"
	"errors"
	"net/http"

	"github.com/K7mQ2vR9/base-pkg/result"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ErrHandler(err error) (int, any) {
	return ErrHandlerCtx(context.Background(), err)
}

// ErrHandlerCtx 使用请求上下文中的 trace，为 httpx.ErrorCtx 构建错误响应。
func ErrHandlerCtx(ctx context.Context, err error) (int, any) {
	return result.HttpErrorResponse(ctx, err)
}

func FromError(err error) *status.Status {
	var code xcode.XCode
	if errors.As(err, &code) {
		return status.New(codes.Code(code.Code()), code.Error())
	}

	return status.New(codes.Code(http.StatusInternalServerError), err.Error())
}

// Deprecated : 使用 pkg/xcode/New 替换
func New(code int, msg string) xcode.Code {
	return xcode.New(code, msg)
}
