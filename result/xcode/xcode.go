package xcode

import (
	"strconv"

	"google.golang.org/grpc/status"
)

type Details struct {
	Code int `json:"code"`
}

type XCode interface {
	Error() string
	Code() int
	// Message 返回可对调用方展示的文案。
	Message() string
	Details() []interface{}
}

type Code struct {
	code int
	msg  string
}

func (c Code) Error() string {
	if len(c.msg) > 0 {
		return c.msg
	}

	return strconv.Itoa(c.code)
}

func (c Code) Code() int {
	return c.code
}

func (c Code) Message() string {
	return c.Error()
}

func (c Code) Details() []interface{} {
	return nil
}

// GRPCStatus 使用标准传输状态码，通过错误详情传递业务码和公开文案。
func (c Code) GRPCStatus() *status.Status {
	return ToGRPCStatus(c)
}

func New(code int, msg string) Code {
	return Code{code: code, msg: msg}
}

// NewRpc 创建携带业务码的 gRPC 错误。
func NewRpc(code int, msg string) error {
	return New(code, msg).GRPCStatus().Err()
}
