package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/K7mQ2vR9/base-pkg/es/esv7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

// ErrNotFound 表示文档或索引不存在，适用于 errors.Is，与 OpenSearch 错误独立。
var ErrNotFound = errors.New("elasticsearch7 document or index not found")

// ErrInvalidResponse 表示缺失响应、正文或必要字段，不能将异常响应当作成功或零条数据。
var ErrInvalidResponse = errors.New("elasticsearch7 invalid response")

// ResponseError 保留 HTTP 状态和最多 128 字节的结构化错误类型，不暴露原始正文或 error.reason。
type ResponseError struct {
	// StatusCode 为服务端 HTTP 状态码，404 可通过 errors.Is 匹配 ErrNotFound。
	StatusCode int
	// ErrorType 只保留结构化 error.type，最多 128 字节；缺失时使用 HTTP 状态文本。
	ErrorType string
}

// Error 返回状态与错误类型摘要，不包含原始正文或 error.reason。
func (e *ResponseError) Error() string {
	return fmt.Sprintf("elasticsearch7 HTTP %d (%s)", e.StatusCode, e.ErrorType)
}

// Unwrap 使 HTTP 404 可通过 errors.Is(err, ErrNotFound) 识别，其他状态不映射业务错误。
func (e *ResponseError) Unwrap() error {
	if e.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	return nil
}

// ReadResponse 接管并关闭有效响应体，成功返回完整正文，错误响应仅读取最多 4KiB 提取错误类型。
// 所有提前返回均关闭正文，不无限 drain；ctx 须与请求相同以中断网络读取。
func ReadResponse(ctx context.Context, response *esapi.Response) (body []byte, err error) {
	if response == nil {
		return nil, ErrInvalidResponse
	}
	if response.Body != nil {
		defer func() {
			if closeErr := response.Body.Close(); err == nil && closeErr != nil {
				err = &operationError{"close response", closeErr}
			}
		}()
	}
	if ctx == nil {
		return nil, errors.New("elasticsearch7 response context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if response.Body == nil || response.StatusCode < 200 {
		return nil, ErrInvalidResponse
	}
	var reader io.Reader = response.Body
	if response.StatusCode >= 300 {
		reader = io.LimitReader(reader, 4096)
	}
	body, err = io.ReadAll(reader)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, &operationError{"read response", err}
	}
	if response.StatusCode >= 300 {
		var payload struct {
			Error struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &payload)
		kind := payload.Error.Type
		if kind == "" {
			kind = http.StatusText(response.StatusCode)
		}
		if len(kind) > 128 {
			kind = kind[:128]
			for !utf8.ValidString(kind) && len(kind) > 0 {
				kind = kind[:len(kind)-1]
			}
		}
		return nil, &ResponseError{StatusCode: response.StatusCode, ErrorType: kind}
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, ErrInvalidResponse
	}
	return body, nil
}

// CheckResponse 用于探活和写操作，校验 HTTP 状态与 JSON 正文并关闭响应体。
func CheckResponse(ctx context.Context, response *esapi.Response) error {
	body, err := ReadResponse(ctx, response)
	if err != nil {
		return err
	}
	var result json.RawMessage
	if err := json.Unmarshal(body, &result); err != nil {
		return &operationError{"decode response", err}
	}
	if raw := bytes.TrimSpace(result); len(raw) == 0 || raw[0] != '{' {
		return ErrInvalidResponse
	}
	return ctx.Err()
}

// ParseGetResponse 关闭响应体并直接解码到调用方目标；404 或 found=false 返回 ErrNotFound。
// 目标须为非 nil 指针，成功时向实现 Schema 的目标设置文档 ID。
func ParseGetResponse(ctx context.Context, target any, response *esapi.Response) error {
	body, err := ReadResponse(ctx, response)
	if err != nil {
		return err
	}
	var document struct {
		ID     string          `json:"_id"`
		Found  *bool           `json:"found"`
		Source json.RawMessage `json:"_source"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return &operationError{"decode get response", err}
	}
	if document.Found != nil && !*document.Found {
		return ErrNotFound
	}
	if document.ID == "" || len(document.Source) == 0 || bytes.Equal(bytes.TrimSpace(document.Source), []byte("null")) {
		return ErrInvalidResponse
	}
	if err := json.Unmarshal(document.Source, target); err != nil {
		return &operationError{"decode document", err}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if schema, ok := target.(esv7.Schema); ok {
		schema.SetId(document.ID)
	}
	return nil
}

// ParseSearchResponse 关闭响应体并将 _source 数组解码到目标切片，保留 int64 精度，空命中返回空数组。
// 支持 ES7 对象或数字形式的 total，缺少必要字段或非法总数返回 ErrInvalidResponse。
func ParseSearchResponse(ctx context.Context, target any, response *esapi.Response) (int64, error) {
	body, err := ReadResponse(ctx, response)
	if err != nil {
		return 0, err
	}
	var result struct {
		Hits *struct {
			Total json.RawMessage `json:"total"`
			Hits  []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, &operationError{"decode search response", err}
	}
	if result.Hits == nil || result.Hits.Hits == nil {
		return 0, ErrInvalidResponse
	}
	var total int64
	if raw := bytes.TrimSpace(result.Hits.Total); len(raw) > 0 && raw[0] == '{' {
		var object struct {
			Value *int64 `json:"value"`
		}
		if err := json.Unmarshal(raw, &object); err != nil {
			return 0, &operationError{"decode search total", err}
		}
		if object.Value == nil {
			return 0, ErrInvalidResponse
		}
		total = *object.Value
	} else if err := json.Unmarshal(result.Hits.Total, &total); err != nil {
		return 0, &operationError{"decode search total", err}
	}
	if total < 0 || bytes.Equal(bytes.TrimSpace(result.Hits.Total), []byte("null")) {
		return 0, ErrInvalidResponse
	}
	sources := make([]json.RawMessage, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		if len(hit.Source) == 0 || bytes.Equal(bytes.TrimSpace(hit.Source), []byte("null")) {
			return 0, ErrInvalidResponse
		}
		sources = append(sources, hit.Source)
	}
	data, err := json.Marshal(sources)
	if err != nil {
		return 0, &operationError{"encode search documents", err}
	}
	if err := json.Unmarshal(data, target); err != nil {
		return 0, &operationError{"decode search documents", err}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return total, nil
}
