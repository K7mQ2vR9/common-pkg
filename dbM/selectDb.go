package dbM

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"gorm.io/gorm"
)

// SelectDbConfig 将业务数据库标识与独立的 MySQL 连接配置关联。
type SelectDbConfig struct {
	// DbKey 为必填的数据库唯一标识，供 GetDb 选择对应连接池。
	DbKey string
	// MySQL 保存连接及可选调优参数，未填写的可选参数使用默认值。
	MySQL MySQLConfig
}

var (
	// ErrSelectDbUninitialized 表示 nil/零值管理器未初始化，或内部连接池条目为空。
	ErrSelectDbUninitialized = errors.New("mysql database manager is not initialized")
	// ErrDbNotConfigured 表示指定 DbKey 不存在，使用 errors.Is 判断，不匹配错误字符串。
	ErrDbNotConfigured = errors.New("mysql database key is not configured")
	// ErrSelectDbClosed 表示管理器已开始关闭，不能再取得新的数据库会话。
	ErrSelectDbClosed = errors.New("mysql database manager is closed")
)

// SelectDb 持有初始化后只读的多库连接映射，支持并发取库和幂等关闭，使用后不可按值复制。
// 请求通过 GetDb 取得独立上下文会话；服务退出前先停止新业务并按需等待在途业务收尾。
type SelectDb struct {
	// dbMap 仅在构造期间写入，关闭时保留映射，避免与并发取库产生 map 读写竞争。
	dbMap map[string]*gorm.DB
	// closed 在开始关闭时置位，拒绝后续 GetDb，不撤销已经返回的 GORM 会话。
	closed atomic.Bool
	// closeOnce 保证连接池回收只执行一次，并让并发 Close 等待同一关闭结果。
	closeOnce sync.Once
	// closeErr 保存第一次完整关闭的汇总结果，由 closeOnce 同步发布给所有 Close 调用方。
	closeErr error
}

// NewSelectDb 校验全部配置后依次初始化连接池，失败时关闭已创建的连接池。
// 空配置会创建可正常关闭的空管理器；启动上下文不控制初始化成功后的实例生命周期。
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
// nil/零值、已开始关闭和未知 DbKey 分别返回可用 errors.Is 判断的分类错误；上下文校验优先。
func (s *SelectDb) GetDb(ctx context.Context, dbKey string) (*gorm.DB, error) {
	if ctx == nil {
		return nil, errors.New("mysql query context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.dbMap == nil {
		return nil, ErrSelectDbUninitialized
	}
	if s.closed.Load() {
		return nil, ErrSelectDbClosed
	}
	db, exists := s.dbMap[dbKey]
	if !exists {
		return nil, fmt.Errorf("mysql DbKey %q: %w", dbKey, ErrDbNotConfigured)
	}
	if db == nil {
		return nil, fmt.Errorf("mysql DbKey %q: %w", dbKey, ErrSelectDbUninitialized)
	}
	return db.WithContext(ctx), nil
}

// Close 先标记关闭并拒绝后续取库，再尝试释放全部连接池，即使部分失败也继续并汇总错误。
// 支持 nil/零值、重复及并发调用，始终返回第一次完整关闭的结果；本方法不强制取消在途查询。
// 已返回的会话按 database/sql 关闭规则处理，调用方须先停止新业务并按需等待收尾。
func (s *SelectDb) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		keys := make([]string, 0, len(s.dbMap))
		for key := range s.dbMap {
			keys = append(keys, key)
		}
		// 按键排序，确保多库关闭和错误诊断顺序稳定，不依赖 map 遍历顺序。
		sort.Strings(keys)
		var closeErrs []error
		for _, key := range keys {
			db := s.dbMap[key]
			if db == nil {
				closeErrs = append(closeErrs, fmt.Errorf("close mysql %q: %w", key, ErrSelectDbUninitialized))
				continue
			}
			pool, err := db.DB()
			if err == nil {
				if pool == nil {
					err = ErrSelectDbUninitialized
				} else {
					err = pool.Close()
				}
			}
			if err != nil {
				closeErrs = append(closeErrs, fmt.Errorf("close mysql %q: %w", key, err))
			}
		}
		s.closeErr = errors.Join(closeErrs...)
	})
	return s.closeErr
}
