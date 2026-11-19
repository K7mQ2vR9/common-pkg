package dbM

import (
	"errors"
	"fmt"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm/logger"
)

// MySQLConfig 封装单个 MySQL 连接池的连接、超时及日志配置。
// 可选字段为 nil 时使用默认值，显式传入的零值按各字段注释处理。
type MySQLConfig struct {
	// DataSource 为必填 DSN，包含地址、认证、数据库及其他驱动参数。
	DataSource string `json:",optional"`
	// StartupTimeout 覆盖探活和版本探测，默认 5s，必须大于 0，继承调用方更早的截止时间。
	StartupTimeout *time.Duration `json:",optional"`
	// DialTimeout 为每次建连的超时，未填时沿用 DSN timeout，DSN 也未填时使用 5s。
	// 显式 0 表示由上下文决定拨号截止时间。
	DialTimeout *time.Duration `json:",optional"`
	// ReadTimeout 为单次网络读取超时，未填时沿用 DSN readTimeout，默认 0。
	// 0 表示由上下文决定读取截止时间。
	ReadTimeout *time.Duration `json:",optional"`
	// WriteTimeout 为单次网络写入超时，未填时沿用 DSN writeTimeout，默认 0。
	// 0 表示由上下文决定写入截止时间。
	WriteTimeout *time.Duration `json:",optional"`
	// MaxIdleConns 为最大空闲连接数，默认 10，0 表示关闭空闲连接保留。
	MaxIdleConns *int `json:",optional"`
	// MaxOpenConns 为最大连接数，默认 80，0 表示不限制连接数。
	MaxOpenConns *int `json:",optional"`
	// ConnMaxLifetime 为连接最大生命周期，默认 30m，0 表示不按生命周期回收。
	ConnMaxLifetime *time.Duration `json:",optional"`
	// ConnMaxIdleTime 为连接最大空闲时间，默认 10m，0 表示不按空闲时间回收。
	ConnMaxIdleTime *time.Duration `json:",optional"`
	// LogLevel 默认为 warn，可配置 silent、error、warn 或 info。
	LogLevel string `json:",optional"`
	// SlowThreshold 为慢 SQL 阈值，默认 200ms，0 表示关闭慢 SQL 阈值检查。
	SlowThreshold *time.Duration `json:",optional"`
	// ParameterizedQueries 默认 true，SQL 日志保留参数占位符，false 时可输出实际参数。
	ParameterizedQueries *bool `json:",optional"`
}

// mysqlSettings 保存应用默认值并完成校验后的配置，仅用于初始化连接池。
type mysqlSettings struct {
	startupTimeout  time.Duration
	maxIdleConns    int
	maxOpenConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
	driver          *mysqlDriver.Config
	log             logger.Config
}

// settings 统一处理 YAML 和 Go 直接传参的默认值，校验过程不会建立数据库连接。
func (config MySQLConfig) settings() (mysqlSettings, error) {
	settings := mysqlSettings{
		startupTimeout:  mysqlConfigValue(config.StartupTimeout, 5*time.Second),
		maxIdleConns:    mysqlConfigValue(config.MaxIdleConns, 10),
		maxOpenConns:    mysqlConfigValue(config.MaxOpenConns, 80),
		connMaxLifetime: mysqlConfigValue(config.ConnMaxLifetime, 30*time.Minute),
		connMaxIdleTime: mysqlConfigValue(config.ConnMaxIdleTime, 10*time.Minute),
		log: logger.Config{
			SlowThreshold:        mysqlConfigValue(config.SlowThreshold, 200*time.Millisecond),
			ParameterizedQueries: mysqlConfigValue(config.ParameterizedQueries, true),
			Colorful:             true,
		},
	}
	if settings.startupTimeout <= 0 {
		return settings, errors.New("mysql StartupTimeout must be positive")
	}
	if settings.maxIdleConns < 0 || settings.maxOpenConns < 0 {
		return settings, errors.New("mysql connection counts must be non-negative")
	}
	if settings.maxOpenConns > 0 && settings.maxIdleConns > settings.maxOpenConns {
		return settings, errors.New("mysql MaxIdleConns must not exceed MaxOpenConns")
	}
	level := strings.ToLower(strings.TrimSpace(config.LogLevel))
	if level == "" {
		level = "warn"
	}
	var valid bool
	settings.log.LogLevel, valid = map[string]logger.LogLevel{
		"silent": logger.Silent,
		"error":  logger.Error,
		"warn":   logger.Warn,
		"info":   logger.Info,
	}[level]
	if !valid {
		return settings, errors.New("mysql LogLevel must be silent, error, warn or info")
	}
	if strings.TrimSpace(config.DataSource) == "" {
		return settings, errors.New("mysql DataSource is required")
	}
	driver, err := mysqlDriver.ParseDSN(config.DataSource)
	if err != nil {
		return settings, &mysqlStartupError{operation: "parse DSN", cause: err}
	}
	dialDefault := driver.Timeout
	if dialDefault == 0 {
		dialDefault = 5 * time.Second
	}
	driver.Timeout = mysqlConfigValue(config.DialTimeout, dialDefault)
	driver.ReadTimeout = mysqlConfigValue(config.ReadTimeout, driver.ReadTimeout)
	driver.WriteTimeout = mysqlConfigValue(config.WriteTimeout, driver.WriteTimeout)
	for _, timeout := range []struct {
		name  string
		value time.Duration
	}{
		{"DialTimeout", driver.Timeout},
		{"ReadTimeout", driver.ReadTimeout},
		{"WriteTimeout", driver.WriteTimeout},
		{"ConnMaxLifetime", settings.connMaxLifetime},
		{"ConnMaxIdleTime", settings.connMaxIdleTime},
		{"SlowThreshold", settings.log.SlowThreshold},
	} {
		if timeout.value < 0 {
			return settings, fmt.Errorf("mysql %s must be non-negative", timeout.name)
		}
	}
	settings.driver = driver
	return settings, nil
}

// mysqlConfigValue 保留显式零值，仅在未配置时返回默认值。
func mysqlConfigValue[T any](configured *T, fallback T) T {
	if configured != nil {
		return *configured
	}
	return fallback
}
