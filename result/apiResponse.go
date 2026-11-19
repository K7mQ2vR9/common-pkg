package result

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/K7mQ2vR9/base-pkg/aesGCM"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/language"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"

	"github.com/zeromicro/go-zero/core/logc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func interfaceToBytes(data interface{}) ([]byte, error) {
	switch v := data.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data: %w", err)
		}
		return jsonBytes, nil
	}
}

// HttpSuccessResult 按进程级配置开关加密响应，序列化或加密失败时返回统一错误。
func HttpSuccessResult(ctx context.Context, w http.ResponseWriter, resp interface{}) {
	resp = language.SwitchLanguage(resp, headInfo.GetContentLanguage(ctx))

	if cipher := aesGCM.ConfiguredCipher(); cipher != nil {
		respByte, err := interfaceToBytes(resp)
		if err != nil {
			HttpErrorResult(ctx, w, err)
			return
		}
		encrypted, err := cipher.Encrypt(respByte)
		if err != nil {
			HttpErrorResult(ctx, w, err)
			return
		}
		resp = map[string]string{"aes_data": encrypted}
	}

	success := Success(resp, headInfo.GetTrace(ctx))
	httpx.WriteJsonCtx(ctx, w, http.StatusOK, success)
}

func HttpSuccessResultNotAes(ctx context.Context, w http.ResponseWriter, resp interface{}) {
	resp = language.SwitchLanguage(resp, headInfo.GetContentLanguage(ctx))
	// todo 不要打印
	// logSucc, _ := json.Marshal(Success(resp, trace.TraceIDFromContext(ctx)))
	// logc.Info(ctx, "ApiResponse:", fmt.Sprintf("%s", string(logSucc)))
	success := Success(resp, headInfo.GetTrace(ctx))
	httpx.WriteJsonCtx(ctx, w, http.StatusOK, success)
}

func HttpErrorResult(ctx context.Context, w http.ResponseWriter, err error) {
	statusCode, body := HttpErrorResponse(ctx, err)
	httpx.WriteJsonCtx(ctx, w, statusCode, body)
}

// HttpErrorResponse 为业务中间件和 go-zero 的全局错误处理器生成同一个响应结构。
func HttpErrorResponse(ctx context.Context, err error) (int, any) {
	code, message, logDetail := publicErrorDetails(err)
	if logDetail && err != nil {
		logc.Errorf(ctx, "HTTP request failed: %v", err)
	}
	message = language.TranslateMessage(message, headInfo.GetContentLanguage(ctx))
	return http.StatusOK, Error(code, message, headInfo.GetTrace(ctx))
}

func publicErrorDetails(err error) (int, string, bool) {
	fallback := http.StatusText(http.StatusInternalServerError)
	var coded xcode.XCode
	if !errors.As(err, &coded) || coded == nil {
		return http.StatusInternalServerError, fallback, true
	}
	if value := reflect.ValueOf(coded); value.Kind() == reflect.Pointer && value.IsNil() {
		return http.StatusInternalServerError, fallback, true
	}

	code := coded.Code()
	if code >= 500 && code <= 599 {
		if message := http.StatusText(code); message != "" {
			return code, message, true
		}
		return code, fallback, true
	}
	switch coded.(type) {
	case xcode.SpecialCodeError, *xcode.SpecialCodeError:
		return code, coded.Message(), true
	default:
		return code, coded.Message(), false
	}
}
