package xxlJob

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config 配置当前进程的 XXL-JOB 执行器，监听与调度中心可访问的注册地址分别指定。
type Config struct {
	// AdminAddress 必填，为调度中心根地址，例如 https://scheduler.example/xxl-job-admin。
	AdminAddress string `json:",optional"`
	// AccessToken 必填，注册/回调和入站调度使用相同令牌，不允许无认证执行任务。
	AccessToken string `json:",optional"`
	// RegistryKey 必填，为调度中心配置的执行器 AppName。
	RegistryKey string `json:",optional"`
	// ListenAddress 默认 :9999；允许端口 0 动态分配，但 ExecutorAddress 仍须显式配置可达地址。
	ListenAddress string `json:",optional"`
	// ExecutorAddress 必填，为调度中心可访问的 HTTP/HTTPS 根地址，可经过反向代理，不得使用未指定 IP。
	ExecutorAddress string `json:",optional"`
	// RequestTimeout 为调度中心请求及入站 HTTP 读写时限，零值默认 5s，负值非法。
	RequestTimeout time.Duration `json:",optional"`
	// RegistryInterval 为注册心跳间隔，零值默认 20s，负值非法；应小于调度中心的失效判定时间。
	RegistryInterval time.Duration `json:",optional"`
	// ShutdownTimeout 为整个关闭流程的总预算，包含心跳退出、HTTP 收尾、摘除和任务/回调等待，默认 10s。
	ShutdownTimeout time.Duration `json:",optional"`
	// MaxBodyBytes 为单次调度请求正文大小上限，零值默认 1MiB，负值非法。
	MaxBodyBytes int64 `json:",optional"`
	// MaxTasks 为已接收且尚未完成的调度数上限，包含运行、排队和回调，零值默认 1024，负值非法。
	MaxTasks int `json:",optional"`
}

func (c Config) normalized() (Config, error) {
	for _, target := range []*string{&c.AdminAddress, &c.ExecutorAddress} {
		u, err := url.Parse(*target)
		if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.TrimSpace(*target) != *target {
			return c, errors.New("xxljob AdminAddress and ExecutorAddress must be HTTP/HTTPS URLs without credentials, query or fragment")
		}
		if target == &c.ExecutorAddress {
			if ip := net.ParseIP(u.Hostname()); ip != nil && ip.IsUnspecified() {
				return c, errors.New("xxljob ExecutorAddress must be reachable, not an unspecified IP")
			}
		}
		*target = strings.TrimRight(*target, "/")
	}
	if strings.TrimSpace(c.AccessToken) == "" || strings.ContainsAny(c.AccessToken, "\r\n") || strings.TrimSpace(c.RegistryKey) == "" {
		return c, errors.New("xxljob AccessToken and RegistryKey are required")
	}
	if c.ListenAddress == "" {
		c.ListenAddress = ":9999"
	}
	_, port, err := net.SplitHostPort(c.ListenAddress)
	if err != nil {
		return c, errors.New("xxljob ListenAddress must contain host and port")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 0 || number > 65535 {
		return c, errors.New("xxljob listen port must be 0..65535")
	}
	for _, item := range []struct {
		value    *time.Duration
		fallback time.Duration
	}{
		{&c.RequestTimeout, 5 * time.Second}, {&c.RegistryInterval, 20 * time.Second}, {&c.ShutdownTimeout, 10 * time.Second},
	} {
		if *item.value < 0 {
			return c, errors.New("xxljob durations must be non-negative")
		}
		if *item.value == 0 {
			*item.value = item.fallback
		}
	}
	if c.MaxBodyBytes < 0 || c.MaxTasks < 0 {
		return c, errors.New("xxljob size and task limits must be non-negative")
	}
	if c.MaxBodyBytes == 0 {
		c.MaxBodyBytes = 1 << 20
	}
	if c.MaxTasks == 0 {
		c.MaxTasks = 1024
	}
	return c, nil
}
