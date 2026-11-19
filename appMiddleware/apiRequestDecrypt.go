package appMiddleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/K7mQ2vR9/base-pkg/aesGCM"
	"github.com/K7mQ2vR9/base-pkg/consts"
	"github.com/K7mQ2vR9/base-pkg/result"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
)

var RequestDecryptError error = xcode.New(http.StatusBadRequest, "Request decryption failed. ")
var RequestBadError error = xcode.New(http.StatusBadRequest, "Request forbid. ")

// ApiRequestDecryptMiddleware 按当前进程的配置开关解密请求。
type ApiRequestDecryptMiddleware struct{}

// NewApiRequestDecryptMiddleware 校验并应用进程级配置，默认关闭 AES-GCM。
func NewApiRequestDecryptMiddleware(config aesGCM.Config) (*ApiRequestDecryptMiddleware, error) {
	if err := aesGCM.Configure(config); err != nil {
		return nil, err
	}
	return &ApiRequestDecryptMiddleware{}, nil
}

// Handle 按配置开关解密请求，失败时返回统一错误响应。
func (m *ApiRequestDecryptMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := m.RequestDecrypt(r); err != nil {
			result.HttpErrorResult(r.Context(), w, err)
			return
		}
		next(w, r)
	}
}

// RequestDecrypt 处理 aes_data 封装，解密成功后重建请求体及其长度。
// 非空请求体带有 business_code 时要求提供 aes_data，其他普通请求可透传。
func (m *ApiRequestDecryptMiddleware) RequestDecrypt(r *http.Request) error {
	cipher := aesGCM.ConfiguredCipher()
	if cipher == nil || r.Body == nil {
		return nil
	}
	data, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return RequestBadError
	}
	replaceRequestBody(r, data)
	if len(data) == 0 {
		return nil
	}

	var envelope struct {
		AESData json.RawMessage `json:"aes_data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.AESData) == 0 {
		if r.Header.Get(consts.BusinessCode) != "" {
			return RequestBadError
		}
		return nil
	}

	var ciphertext string
	if err := json.Unmarshal(envelope.AESData, &ciphertext); err != nil || ciphertext == "" {
		return RequestDecryptError
	}
	plaintext, err := cipher.Decrypt(ciphertext)
	if err != nil {
		return RequestDecryptError
	}
	replaceRequestBody(r, plaintext)
	return nil
}

// replaceRequestBody 为后续解析和请求重读提供相同的数据，清除原传输编码和长度。
func replaceRequestBody(r *http.Request, body []byte) {
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	r.Body, _ = r.GetBody()
	r.ContentLength = int64(len(body))
	r.TransferEncoding = nil
	if r.Header != nil {
		r.Header.Set("Content-Length", strconv.Itoa(len(body)))
		r.Header.Del("Transfer-Encoding")
	}
}
