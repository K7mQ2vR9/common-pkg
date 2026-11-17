package main

import (
	"context"
	"fmt"
	"github.com/doubunv/common-pkg/xxlJob"
	"github.com/xxl-job/xxl-job-executor-go"
	"log"
	"os"

	"github.com/xxl-job/xxl-job-executor-go/example/task"
)

func main() {
	address, accessToken := schedulerConnection()
	client := xxlJob.NewClient(context.Background(), xxlJob.Config{
		Address:      address,
		AccessToken:  accessToken,
		ExecutorPort: "9999",
		RegistryKey:  "golang-jobs",
	}).Register(
		&TestHandler{},
	)
	client.MustStart()

	select {}
}

// schedulerConnection 从运行环境读取示例服务的连接地址和凭证。
func schedulerConnection() (address, accessToken string) {
	address = os.Getenv("XXL_JOB_ADDRESS")
	accessToken = os.Getenv("XXL_JOB_ACCESS_TOKEN")
	if address == "" || accessToken == "" {
		log.Fatal("XXL_JOB_ADDRESS and XXL_JOB_ACCESS_TOKEN must be set")
	}
	return address, accessToken
}

type TestHandler struct {
}

func (h TestHandler) Handler() xxlJob.HandlerFun {
	return func(cxt context.Context, param *xxl.RunReq) string {
		fmt.Println("test one task" + param.ExecutorHandler + " param：" + param.ExecutorParams + " log_id:" + xxl.Int64ToStr(param.LogID))
		return "test finish..."
	}
}

func (h TestHandler) Pattern() string {
	return "task.test"
}

func Old() {
	address, accessToken := schedulerConnection()
	exec := xxl.NewExecutor(
		xxl.ServerAddr(address),
		xxl.AccessToken(accessToken),
		// xxl.ExecutorIp("127.0.0.1"),      // 可自动获取
		xxl.ExecutorPort("9999"),       // 默认9999（非必填）
		xxl.RegistryKey("golang-jobs"), // 执行器名称
		// xxl.SetLogger(&logger{}),       // 自定义日志
	)
	exec.Init()
	// 设置日志查看handler
	// exec.LogHandler(customLogHandle)
	// 注册任务handler
	exec.RegTask("task.test", task.Test)
	exec.RegTask("task.test2", task.Test2)
	exec.RegTask("task.panic", task.Panic)
	log.Fatal(exec.Run())
}
