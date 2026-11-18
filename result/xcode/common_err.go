package xcode

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	UserNotFound             = New(http.StatusUnauthorized, "User not login. ")
	TokenInvalid             = New(http.StatusPaymentRequired, "Token invalid. ")
	TokenPermissionDenied    = New(http.StatusMethodNotAllowed, "No permission")
	StatusServiceUnavailable = New(http.StatusServiceUnavailable, "Status service unavailable")
)

func TokenPermissionDeniedWithMsg(msg string) error {
	return New(http.StatusMethodNotAllowed, fmt.Sprintf("无权限: %s", msg))
}

func TokenNotLoginWithMsg(msg string) error {
	return New(http.StatusUnauthorized, msg)
}

func TokenInvalidWithMsg(msg string) error {
	return New(http.StatusPaymentRequired, msg)
}

func TokenSpecial(err error) error {
	if ok, codeErr := IsSpecialCodeError(err); ok && codeErr != nil {
		return New(codeErr.Code(), "Authentication failed.")
	}
	return TokenInvalid
}

func NewNotifyError(err error) error {
	return NewSpecialError(err, http.StatusGone)
}

func NewSpecialError(err error, code int) error {
	return &SpecialCodeError{
		Err:  err,
		code: code,
	}
}

// SpecialCodeError 需要特殊通知的error
type SpecialCodeError struct {
	Err  error // 底层包装的 error
	code int   // 状态码
}

func IsSpecialNotifyError(err error) bool {
	return errors.Is(err, specialNotifyErr)
}

func IsSpecialCodeError(err error) (ok bool, finalErr *SpecialCodeError) {
	if errors.As(err, &finalErr) && finalErr != nil {
		return true, finalErr
	}

	return false, nil
}

var specialNotifyErr = &SpecialCodeError{code: http.StatusGone}

func (e SpecialCodeError) Error() string {
	if e.Err == nil {
		return e.Message()
	}
	return e.Err.Error()
}

// Unwrap 支持错误链解包
func (e *SpecialCodeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e SpecialCodeError) Code() int {
	return e.code
}

func (e SpecialCodeError) Message() string {
	return "Request failed."
}

func (e SpecialCodeError) Details() []interface{} {
	return nil
}

// Is 自定义 errors.Is 匹配规则
// 场景：允许按错误码（Code）进行等价比对，忽略动态变化的 Message 或底层 Err
func (e *SpecialCodeError) Is(target error) bool {
	other, ok := target.(*SpecialCodeError)
	return ok && e != nil && other != nil && e.code == other.code
}

// As 自定义 errors.As 行为（可选）
// 如果需要对目标指针进行特殊赋值或校验时实现；否则 Go 标准库默认会通过反射匹配指针类型
func (e *SpecialCodeError) As(target any) bool {
	t, ok := target.(**SpecialCodeError)
	if !ok {
		return false
	}
	*t = e
	return true
}
