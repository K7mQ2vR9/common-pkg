package dbM

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// SelectDbConfig 将业务数据库标识与独立的 MySQL 连接配置关联。
type SelectDbConfig struct {
	// DbKey 为必填的数据库唯一标识，供 GetDb 选择对应连接池。
	DbKey string
	// MySQL 保存连接及可选调优参数，未填写的可选参数使用默认值。
	MySQL MySQLConfig
}

type SelectDb struct {
	dbMap map[string]*gorm.DB
}

// NewSelectDb 校验全部配置后依次初始化连接池，失败时关闭已创建的连接池。
func NewSelectDb(ctx context.Context, config []SelectDbConfig) (*SelectDb, error) {
	if ctx == nil {
		return nil, errors.New("mysql startup context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keys := make(map[string]struct{}, len(config))
	for _, entry := range config {
		if strings.TrimSpace(entry.DbKey) == "" {
			return nil, errors.New("mysql DbKey is required")
		}
		if _, exists := keys[entry.DbKey]; exists {
			return nil, fmt.Errorf("duplicate mysql DbKey %q", entry.DbKey)
		}
		keys[entry.DbKey] = struct{}{}
		if _, err := entry.MySQL.settings(); err != nil {
			return nil, fmt.Errorf("configure mysql %q: %w", entry.DbKey, err)
		}
	}

	res := &SelectDb{dbMap: make(map[string]*gorm.DB, len(config))}
	for _, entry := range config {
		db, err := ConnectMySQL(ctx, entry.MySQL)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("initialize mysql %q: %w", entry.DbKey, err), res.Close())
		}
		res.dbMap[entry.DbKey] = db
	}
	return res, nil
}

// GetDb 返回绑定本次请求上下文的会话，后续查询和事务继承其取消信号及截止时间。
func (s *SelectDb) GetDb(ctx context.Context, dbKey string) (*gorm.DB, error) {
	if ctx == nil {
		return nil, errors.New("mysql query context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db, exists := s.dbMap[dbKey]
	if !exists {
		return nil, fmt.Errorf("mysql DbKey %q is not configured", dbKey)
	}
	return db.WithContext(ctx), nil
}

// Close 释放 NewSelectDb 创建的全部连接池，可在服务退出时调用。
func (s *SelectDb) Close() error {
	var closeErrs []error
	for key, db := range s.dbMap {
		pool, err := db.DB()
		if err == nil {
			err = pool.Close()
		}
		if err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("close mysql %q: %w", key, err))
		}
	}
	return errors.Join(closeErrs...)
}
