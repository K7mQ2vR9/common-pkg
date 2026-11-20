package xxlJob

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	xxl "github.com/xxl-job/xxl-job-executor-go"
	"go.opentelemetry.io/otel/propagation"
)

type envelope struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Content any    `json:"content,omitempty"`
}

func (c *Client) registration() any {
	return struct {
		Group string `json:"registryGroup"`
		Key   string `json:"registryKey"`
		Value string `json:"registryValue"`
	}{"EXECUTOR", c.conf.RegistryKey, c.conf.ExecutorAddress}
}

// post 仅访问配置的调度中心，取消和 RequestTimeout 同时约束网络及正文读取，不自动重试回调。
func (c *Client) post(ctx context.Context, action string, payload any) error {
	ctx, cancel := context.WithTimeout(ctx, c.conf.RequestTimeout)
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return &operationError{action, err}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.conf.AdminAddress+"/api/"+action, bytes.NewReader(body))
	if err != nil {
		return &operationError{action, err}
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	request.Header.Set("XXL-JOB-ACCESS-TOKEN", c.conf.AccessToken)
	(propagation.TraceContext{}).Inject(ctx, propagation.HeaderCarrier(request.Header))
	response, err := c.httpClient.Do(request)
	if err != nil {
		return &operationError{action, err}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if ctx.Err() != nil {
		return &operationError{action, ctx.Err()}
	}
	if err != nil {
		return &operationError{action, err}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || len(data) > 1<<20 {
		return &operationError{action, errors.New("unexpected admin HTTP status or response size")}
	}
	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return &operationError{action, err}
	}
	if result.Code != 200 {
		return &operationError{action, errors.New("admin rejected request")}
	}
	return nil
}

func (c *Client) router() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			c.reply(w, 405, 500, "POST required", nil)
			return
		}
		tokens := r.Header.Values("XXL-JOB-ACCESS-TOKEN")
		if len(tokens) != 1 || subtle.ConstantTimeCompare([]byte(tokens[0]), []byte(c.conf.AccessToken)) != 1 {
			c.reply(w, 403, 500, "invalid access token", nil)
			return
		}
		c.mu.Lock()
		available := c.state == starting || c.state == running
		c.mu.Unlock()
		if !available {
			c.reply(w, 503, 500, "executor stopping", nil)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, c.conf.MaxBodyBytes)
		defer r.Body.Close()
		switch r.URL.Path {
		case "/beat":
			c.reply(w, 200, 200, "", nil)
		case "/run":
			var param xxl.RunReq
			if decode(r, &param) != nil {
				c.reply(w, 200, 500, "invalid run request", nil)
				return
			}
			if err := c.enqueue(r, param); err != nil {
				c.reply(w, 200, 500, err.Error(), nil)
				return
			}
			c.reply(w, 200, 200, "", nil)
		case "/kill", "/idleBeat":
			var param struct {
				JobID int64 `json:"jobId"`
			}
			if decode(r, &param) != nil || param.JobID <= 0 {
				c.reply(w, 200, 500, "invalid job ID", nil)
				return
			}
			if r.URL.Path == "/kill" {
				c.kill(param.JobID)
				c.reply(w, 200, 200, "", nil)
				return
			}
			c.mu.Lock()
			busy := c.jobs[param.JobID] != nil
			c.mu.Unlock()
			if busy {
				c.reply(w, 200, 500, "job is running or queued", nil)
			} else {
				c.reply(w, 200, 200, "", nil)
			}
		case "/log":
			c.readLog(w, r)
		default:
			c.reply(w, 404, 500, "unknown endpoint", nil)
		}
	})
}

func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("multiple or invalid JSON values")
	}
	return nil
}

func (c *Client) reply(w http.ResponseWriter, status, code int, message string, content any) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Code: code, Msg: message, Content: content})
}

func (c *Client) readLog(w http.ResponseWriter, r *http.Request) {
	var param xxl.LogReq
	if decode(r, &param) != nil || param.LogID <= 0 || param.FromLineNum < 1 {
		c.reply(w, 200, 500, "invalid log request", nil)
		return
	}
	c.mu.Lock()
	handler := c.logHandler
	c.mu.Unlock()
	if handler == nil {
		c.reply(w, 200, 200, "", &xxl.LogResContent{FromLineNum: param.FromLineNum, ToLineNum: param.FromLineNum, LogContent: "未配置调度中心日志查询，请使用应用日志系统。", IsEnd: true})
		return
	}
	// 日志提供器属于业务代码，panic 不得破坏 HTTP 服务或泄露原始参数。
	defer func() {
		if recover() != nil {
			c.reply(w, 200, 500, "log handler panicked", nil)
		}
	}()
	content, err := handler(r.Context(), &param)
	if err != nil || content == nil {
		c.reply(w, 200, 500, "log query failed", nil)
		return
	}
	c.reply(w, 200, 200, "", content)
}
