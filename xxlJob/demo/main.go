package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/xxlJob"
	xxl "github.com/xxl-job/xxl-job-executor-go"
)

func main() {
	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := xxlJob.NewClient(xxlJob.Config{
		AdminAddress:    os.Getenv("XXL_JOB_ADMIN_ADDRESS"),
		AccessToken:     os.Getenv("XXL_JOB_ACCESS_TOKEN"),
		ExecutorAddress: os.Getenv("XXL_JOB_EXECUTOR_ADDRESS"),
		ListenAddress:   os.Getenv("XXL_JOB_LISTEN_ADDRESS"),
		RegistryKey:     "platform-jobs",
	})
	if err != nil {
		log.Printf("initialize executor: %v", err)
		return
	}
	if err := client.Register(TestHandler{}); err != nil {
		log.Printf("register task: %v", err)
		return
	}
	if err := client.Start(appCtx); err != nil {
		log.Printf("start executor: %v", err)
		return
	}
	// Start 后返回，监听错误或退出信号都会触发有界关闭；不再由 SDK 接管进程信号。
	if err := client.Wait(context.Background()); err != nil {
		log.Printf("executor stopped: %v", err)
	}
}

type TestHandler struct{}

func (TestHandler) Pattern() string { return "task.test" }
func (TestHandler) Handler() xxlJob.HandlerFun {
	return func(ctx context.Context, param *xxl.RunReq) string {
		select {
		case <-ctx.Done():
			return "canceled"
		default:
		}
		// 实际数据库、RPC 和 MQ 操作均使用 ctx；不要输出未经处理的 ExecutorParams 或令牌。
		log.Printf("job=%d log=%d trace=%s", param.JobID, param.LogID, headInfo.GetTrace(ctx))
		return "finished"
	}
}
