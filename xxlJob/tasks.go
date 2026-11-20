package xxlJob

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	xxl "github.com/xxl-job/xxl-job-executor-go"
	"github.com/zeromicro/go-zero/core/logc"
	"go.opentelemetry.io/otel/trace"
)

type execution struct {
	request xxl.RunReq
	fn      HandlerFun
	ctx     context.Context
	cancel  context.CancelFunc
	once    sync.Once
}
type jobQueue struct {
	current *execution
	queue   []*execution
}

func (c *Client) enqueue(r *http.Request, param xxl.RunReq) error {
	if param.JobID <= 0 || param.LogID <= 0 || param.ExecutorTimeout < 0 || param.ExecutorTimeout > math.MaxInt64/int64(time.Second) || param.BroadcastIndex < 0 || param.BroadcastTotal < 0 || (param.BroadcastTotal > 0 && param.BroadcastIndex >= param.BroadcastTotal) {
		return errors.New("invalid run parameters")
	}
	if param.GlueType != "" && param.GlueType != "BEAN" {
		return errors.New("only BEAN tasks are supported")
	}
	strategy := param.ExecutorBlockStrategy
	if strategy == "" {
		strategy = "SERIAL_EXECUTION"
	}
	if strategy != "SERIAL_EXECUTION" && strategy != "DISCARD_LATER" && strategy != "COVER_EARLY" {
		return errors.New("invalid block strategy")
	}
	// 每次独立调度初始化自己的链路，保留合法入站 ID，不继承启动请求的 ID 或 HTTP 请求的取消。
	base, err := headInfo.ExtractRequestTrace(context.Background(), r.Header)
	if errors.Is(err, headInfo.ErrTraceRequired) {
		base, err = headInfo.EnsureTrace(context.Background())
	}
	if err != nil {
		return errors.New("invalid task trace")
	}
	c.mu.Lock()
	if c.state != running || c.life.Err() != nil {
		c.mu.Unlock()
		return errors.New("executor is not running")
	}
	fn, ok := c.handlers[param.ExecutorHandler]
	if !ok {
		c.mu.Unlock()
		return errors.New("task handler not registered")
	}
	if existing := c.active[param.LogID]; existing != nil {
		c.mu.Unlock()
		if existing.request != param {
			return errors.New("log ID conflicts with an active task")
		}
		return nil
	}
	if len(c.active) >= c.conf.MaxTasks {
		c.mu.Unlock()
		return errors.New("executor task limit reached")
	}
	job := c.jobs[param.JobID]
	if job != nil && strategy == "DISCARD_LATER" {
		c.mu.Unlock()
		return errors.New("job is already running or queued")
	}
	ctx, cancel := context.WithCancel(base)
	stop := context.AfterFunc(c.life, cancel)
	task := &execution{request: param, fn: fn, ctx: ctx, cancel: func() { stop(); cancel() }}
	c.active[param.LogID] = task
	c.tasks.Add(1)
	var replaced []*execution
	if job == nil {
		job = &jobQueue{current: task}
		c.jobs[param.JobID] = job
		go c.runJob(param.JobID, job)
	} else {
		if strategy == "COVER_EARLY" {
			job.current.cancel()
			replaced = job.queue
			job.queue = nil
		}
		job.queue = append(job.queue, task)
	}
	c.mu.Unlock()
	for _, old := range replaced {
		old.cancel()
		go c.complete(old, 500, "task replaced")
	}
	return nil
}

func (c *Client) runJob(id int64, job *jobQueue) {
	for {
		c.mu.Lock()
		task := job.current
		c.mu.Unlock()
		code, message := invoke(task)
		task.cancel()
		// 回调不占用业务串行队列，但仍计入 MaxTasks 与关闭等待，网络故障不阻塞下一个业务任务。
		go c.complete(task, code, message)
		c.mu.Lock()
		if c.state != running || len(job.queue) == 0 {
			pending := job.queue
			job.queue = nil
			if c.jobs[id] == job {
				delete(c.jobs, id)
			}
			c.mu.Unlock()
			// 关闭与当前任务退出可能先于 shutdown 收集队列，必须在移除 job 前接管剩余回调。
			for _, queued := range pending {
				queued.cancel()
				go c.complete(queued, 500, "executor stopped")
			}
			return
		}
		job.current, job.queue = job.queue[0], job.queue[1:]
		c.mu.Unlock()
	}
}

func invoke(task *execution) (code int, message string) {
	ctx := task.ctx
	var cancel context.CancelFunc
	if task.request.ExecutorTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(task.request.ExecutorTimeout)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	defer func() {
		if recover() != nil {
			code, message = 500, "task handler panicked"
			logc.Errorf(ctx, "XXL-JOB task handler panicked: job_id=%d log_id=%d", task.request.JobID, task.request.LogID)
		}
	}()
	if err := ctx.Err(); err != nil {
		return 500, "task canceled"
	}
	param := task.request // 业务修改参数不能污染回调 ID 或其他调度的参数。
	message = task.fn(ctx, &param)
	if err := ctx.Err(); err != nil {
		return 500, "task canceled or timed out"
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return 500, "task timed out"
	}
	return 200, summary(message)
}

func (c *Client) complete(task *execution, code int, message string) {
	task.once.Do(func() {
		defer c.tasks.Done()
		defer task.cancel()
		// 业务取消后仍需报告失败；回调沿用原任务 Trace，并由独立关闭预算及 RequestTimeout 约束。
		ctx, cancel := context.WithCancel(trace.ContextWithSpanContext(c.callbackLife, trace.SpanContextFromContext(task.ctx)))
		defer cancel()
		payload := []struct {
			LogID      int64  `json:"logId"`
			LogDateTim int64  `json:"logDateTim"`
			HandleCode int    `json:"handleCode"`
			HandleMsg  string `json:"handleMsg"`
		}{{task.request.LogID, task.request.LogDateTime, code, summary(message)}}
		if err := c.post(ctx, "callback", payload); err != nil {
			logc.Errorf(task.ctx, "XXL-JOB task callback failed: job_id=%d log_id=%d error=%v", task.request.JobID, task.request.LogID, err)
		}
		c.mu.Lock()
		if c.active[task.request.LogID] == task {
			delete(c.active, task.request.LogID)
		}
		c.mu.Unlock()
	})
}

func (c *Client) kill(id int64) {
	c.mu.Lock()
	job := c.jobs[id]
	if job == nil {
		c.mu.Unlock()
		return
	}
	job.current.cancel()
	pending := job.queue
	job.queue = nil
	c.mu.Unlock()
	for _, task := range pending {
		task.cancel()
		go c.complete(task, 500, "task killed before execution")
	}
}

func summary(message string) string {
	// 先限制扫描范围，再规范无效 UTF-8，防止 JSON 替换字符使回调摘要膨胀超限。
	if len(message) > 1024 {
		message = message[:1024]
	}
	if !utf8.ValidString(message) {
		message = strings.ToValidUTF8(message, "�")
	}
	if len(message) > 1024 {
		message = message[:1024]
	}
	for !utf8.ValidString(message) && len(message) > 0 {
		message = message[:len(message)-1]
	}
	return message
}
