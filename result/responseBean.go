package result

import (
	"net/http"
)

type ResponseBean struct {
	Code  int         `json:"code"`
	Msg   string      `json:"message"`
	Data  interface{} `json:"data"`
	Trace string      `json:"trace"` // 统一请求 ID，与响应头 X-Request-ID 及下游 Trace ID 相同。
}

func Success(data interface{}, trace string) *ResponseBean {
	if data == nil {
		data = struct{}{}
	}
	return &ResponseBean{http.StatusOK, "success", data, trace}
}

func Error(errCode int, errMsg string, trace string) *ResponseBean {
	return &ResponseBean{errCode, errMsg, struct{}{}, trace}
}
