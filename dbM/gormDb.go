package dbM

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectMySQL 按配置创建连接池，并在启动上下文的截止时间内完成探活和版本探测。
// 未配置的可选字段使用默认值，调用方负责在服务退出时关闭返回的连接池。
func ConnectMySQL(ctx context.Context, config MySQLConfig) (*gorm.DB, error) {
	if ctx == nil {
		return nil, errors.New("mysql startup context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	settings, err := config.settings()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, settings.startupTimeout)
	defer cancel()
	connector, err := mysqlDriver.NewConnector(settings.driver)
	if err != nil {
		return nil, &mysqlStartupError{operation: "configure connector", cause: err}
	}
	pool := sql.OpenDB(connector)
	ready := false
	defer func() {
		if !ready {
			_ = pool.Close()
		}
	}()

	pool.SetMaxIdleConns(settings.maxIdleConns)
	pool.SetMaxOpenConns(settings.maxOpenConns)
	pool.SetConnMaxLifetime(settings.connMaxLifetime)
	pool.SetConnMaxIdleTime(settings.connMaxIdleTime)

	if err := pool.PingContext(ctx); err != nil {
		return nil, mysqlProbeError(ctx, "ping", err)
	}

	dialector := &mysql.Dialector{Config: &mysql.Config{
		DSNConfig: settings.driver,
		Conn:      mysqlStartupPool{DB: pool, ctx: ctx},
	}}
	db, err := gorm.Open(dialector, &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Discard,
	})
	if err != nil {
		return nil, mysqlProbeError(ctx, "detect server version", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 启动上下文仅用于版本探测，初始化后由业务会话提供查询上下文。
	dialector.Conn = pool
	db.ConnPool = pool
	db.Statement.ConnPool = pool
	db.Logger = logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), settings.log)
	ready = true
	return db, nil
}

// mysqlStartupPool 为驱动内部的 SELECT VERSION() 注入启动上下文。
type mysqlStartupPool struct {
	*sql.DB
	ctx context.Context
}

func (p mysqlStartupPool) QueryRowContext(_ context.Context, query string, args ...any) *sql.Row {
	return p.DB.QueryRowContext(p.ctx, query, args...)
}

type mysqlStartupError struct {
	operation string
	cause     error
}

func (err *mysqlStartupError) Error() string {
	return "mysql " + err.operation + " failed"
}

func (err *mysqlStartupError) Unwrap() error {
	return err.cause
}

func mysqlProbeError(ctx context.Context, operation string, cause error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("mysql %s: %w", operation, err)
	}
	return &mysqlStartupError{operation: operation, cause: cause}
}
