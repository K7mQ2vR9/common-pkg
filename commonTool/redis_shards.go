package commonTool

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/cespare/xxhash/v2"
)

// ErrInvalidShardCount 表示业务逻辑分片数为 0；调用方须显式配置正数，不使用隐式默认值。
var ErrInvalidShardCount = errors.New("shard count must be greater than zero")

// ShardIndexXX 将 userID 的十进制文本按 xxhash64 哈希并对 shardCount 取模，返回 [0, shardCount) 内的编号。
// userID 支持全部 int64 值；shardCount 必须大于 0，非法时返回 0 和 ErrInvalidShardCount。
// 分片数为业务逻辑桶数，不是 Redis 节点数或槽数；修改它会改变部分路由，已有数据须安排迁移。
func ShardIndexXX(userID int64, shardCount uint64) (uint64, error) {
	if shardCount == 0 {
		return 0, ErrInvalidShardCount
	}
	// 保持原有十进制编码与哈希算法，合法参数的结果不随错误处理改造而变化。
	b := []byte(strconv.FormatInt(userID, 10))
	return xxhash.Sum64(b) % shardCount, nil
}

// ShardTagXX 返回 Redis Cluster hash tag，例如 {s000}，索引至少补齐三位，超过三位时自然扩展。
// shardCount 为 0 时返回空串及 ErrInvalidShardCount；调用方必须先处理错误，再用标签构造 key。
// 同标签用于让多个 key 落入同一槽，不保证不同标签属于不同槽或节点；含其他花括号的 key 前缀可能改变标签解析。
func ShardTagXX(userID int64, shardCount uint64) (string, error) {
	idx, err := ShardIndexXX(userID, shardCount)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("{s%03d}", idx), nil
}
