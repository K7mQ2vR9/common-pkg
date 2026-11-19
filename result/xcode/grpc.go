package xcode

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strconv"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	rpcErrorDomain = "github.com/K7mQ2vR9/base-pkg"
	rpcErrorReason = "BUSINESS_ERROR"
)

// ToGRPCStatus 将业务错误转换为标准 gRPC 状态，业务码存入 ErrorInfo。
// 普通错误和内部错误使用固定公开文案，nil 返回 OK。
func ToGRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}
	if nilRPCError(err) {
		return status.New(codes.Internal, http.StatusText(http.StatusInternalServerError))
	}
	var coded XCode
	if errors.As(err, &coded) {
		if nilRPCError(coded) {
			return status.New(codes.Internal, http.StatusText(http.StatusInternalServerError))
		}
		return businessGRPCStatus(coded)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.New(codes.DeadlineExceeded, context.DeadlineExceeded.Error())
	}
	if errors.Is(err, context.Canceled) {
		return status.New(codes.Canceled, context.Canceled.Error())
	}
	// 读取原始状态提供者，避免外层包装错误的诊断文本进入公开 message。
	var provider interface {
		error
		GRPCStatus() *status.Status
	}
	if errors.As(err, &provider) && !nilRPCError(provider) {
		if st := provider.GRPCStatus(); st != nil {
			return publicGRPCStatus(st)
		}
	}
	return status.New(codes.Internal, http.StatusText(http.StatusInternalServerError))
}

func businessGRPCStatus(err XCode) *status.Status {
	code := err.Code()
	if code <= 0 {
		return status.New(codes.Internal, http.StatusText(http.StatusInternalServerError))
	}
	message := publicBusinessMessage(code, err.Message())
	detail := &errdetails.ErrorInfo{
		Domain:   rpcErrorDomain,
		Reason:   rpcErrorReason,
		Metadata: map[string]string{"code": strconv.Itoa(code), "message": message},
	}
	switch err.(type) {
	case SpecialCodeError, *SpecialCodeError:
		detail.Metadata["kind"] = "special"
	}
	st, detailErr := status.New(grpcCode(code), message).WithDetails(detail)
	if detailErr != nil {
		return status.New(codes.Internal, http.StatusText(http.StatusInternalServerError))
	}
	return st
}

// FromGRPCError 为 API 恢复业务码和公开文案，并保留底层错误链。
// 原生 gRPC 取消和超时同时支持 errors.Is 检查标准 context 错误。
func FromGRPCError(err error) error {
	if err == nil {
		return nil
	}
	st := ToGRPCStatus(err)
	code := httpCode(st.Code())
	message := st.Message()
	special := false
	for _, value := range st.Details() {
		detail, ok := value.(*errdetails.ErrorInfo)
		if !ok || detail.Domain != rpcErrorDomain || detail.Reason != rpcErrorReason {
			continue
		}
		businessCode, parseErr := strconv.Atoi(detail.Metadata["code"])
		if parseErr == nil && businessCode > 0 {
			code = businessCode
			if publicMessage, exists := detail.Metadata["message"]; exists {
				message = publicMessage
			}
			special = detail.Metadata["kind"] == "special"
			break
		}
	}
	cause := err
	switch st.Code() {
	case codes.Canceled:
		cause = errors.Join(err, context.Canceled)
	case codes.DeadlineExceeded:
		cause = errors.Join(err, context.DeadlineExceeded)
	}
	if special {
		return NewSpecialError(cause, code)
	}
	return &rpcError{XCode: New(code, publicBusinessMessage(code, message)), cause: cause}
}

type rpcError struct {
	XCode
	cause error
}

func (err *rpcError) GRPCStatus() *status.Status {
	return ToGRPCStatus(err)
}

func (err *rpcError) Unwrap() error {
	return err.cause
}

func nilRPCError(err error) bool {
	if err == nil {
		return true
	}
	value := reflect.ValueOf(err)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

// publicBusinessMessage 统一内部业务错误的公开文案。
func publicBusinessMessage(code int, message string) string {
	if code < 500 || code > 599 {
		return message
	}
	if text := http.StatusText(code); text != "" {
		return text
	}
	return http.StatusText(http.StatusInternalServerError)
}

func publicGRPCStatus(st *status.Status) *status.Status {
	code := st.Code()
	if code == codes.OK || code > codes.Unauthenticated {
		return status.New(codes.Internal, http.StatusText(http.StatusInternalServerError))
	}
	proto := st.Proto()
	switch code {
	case codes.Internal, codes.Unknown, codes.DataLoss:
		proto.Message = http.StatusText(http.StatusInternalServerError)
	case codes.Unavailable:
		proto.Message = http.StatusText(http.StatusServiceUnavailable)
	case codes.Canceled:
		proto.Message = context.Canceled.Error()
	case codes.DeadlineExceeded:
		proto.Message = context.DeadlineExceeded.Error()
	}
	return status.FromProto(proto)
}

// grpcCode 将 HTTP 风格的业务码映射为有效的 gRPC 状态码。
func grpcCode(code int) codes.Code {
	switch code {
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized, http.StatusPaymentRequired:
		return codes.Unauthenticated
	case http.StatusForbidden, http.StatusMethodNotAllowed:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return codes.DeadlineExceeded
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case 499:
		return codes.Canceled
	case http.StatusNotImplemented:
		return codes.Unimplemented
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	default:
		if code >= 500 && code <= 599 {
			return codes.Internal
		}
		return codes.FailedPrecondition
	}
}

// httpCode 为未携带平台业务详情的原生 gRPC 错误选择 API 业务码。
func httpCode(code codes.Code) int {
	switch code {
	case codes.Canceled:
		return 499
	case codes.InvalidArgument, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
