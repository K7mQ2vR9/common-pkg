# Redis 逻辑分片使用说明

```text
ShardIndexXX(userID int64, shardCount uint64) (uint64, error)
ShardTagXX(userID int64, shardCount uint64) (string, error)
```

`shardCount` 为显式配置的业务逻辑桶数，必须大于 0，无默认值。零值返回 `ErrInvalidShardCount`，可通过 `errors.Is` 判断；索引同时返回 0、标签返回空串，错误结果不能继续用于构造业务 key。

`userID` 支持全部 int64 值，包括 0、负数和边界值，业务按自身 ID 规则另行校验。算法保持为：

```text
index = xxhash64(用户 ID 的有符号十进制文本) % shardCount
tag   = "{s" + 索引的十进制文本（至少三位，左侧补零） + "}"
```

例如索引 0 的标签为 `{s000}`，索引 1000 的标签为 `{s1000}`。三位为最小宽度，不限制分片数为 1000。逻辑桶数也不限制为 Redis 的 16384 个槽，返回索引范围始终为 `[0, shardCount)`。

两个函数均无共享可变状态，可并发调用。接口已增加 `error` 返回值，不保留旧签名；有效参数的哈希、索引及标签格式保持原样。

## 构造同用户的关联 key

```go
package examples

import (
    "fmt"
    "strconv"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

// UserRedisKeys 演示固定命名空间和分片数；实际服务从统一应用配置传入 shardCount。
func UserRedisKeys(userID int64, shardCount uint64) (profileKey, settingsKey string, err error) {
    tag, err := commonTool.ShardTagXX(userID, shardCount)
    if err != nil {
        return "", "", fmt.Errorf("build user Redis keys: %w", err)
    }
    // 标签聚合多个用户，同桶仍须在 key 中保留用户 ID，防止不同用户覆盖同一个 key。
    prefix := "platform:user:" + tag + ":" + strconv.FormatInt(userID, 10)
    return prefix + ":profile", prefix + ":settings", nil
}
```

分片数由服务的统一配置管理，在启动时检查大于 0；所有读写路径和 API/RPC 实例使用同一值。不要在构造不同关联 key 时传入不同分片数。API 通过 RPC 数据模型服务编排业务；上述 key 构造示例仅供实际 Redis 数据存取方使用。

## 与 Redis Cluster 槽位的关系

Redis Cluster 根据 hash tag 内容计算 `CRC16(tag) % 16384`；同标签使多个 key 位于同一槽，不同逻辑桶标签也可能映射到同一槽或由同一节点服务。该工具只计算逻辑索引/标签，不选择 Redis 节点、不管理连接、不进行数据读写。

Redis 按第一个左花括号及其后的第一个右花括号解析，且中间不能为空。构造 key 时确保本工具的标签先被解析，固定前缀避免使用其他花括号。具体规则见 [Redis Cluster hash tags](https://redis.io/docs/latest/operate/oss_and_stack/reference/cluster-spec/#hash-tags)。

同标签会让同桶用户的数据集中到一个槽，桶数和热点分布需结合业务负载选择。同槽多 key 操作仍受具体 Redis 命令及集群迁移状态约束，不代表任意多 key 命令都可用。

## 修改分片数

取模分片不是一致性哈希，修改 `shardCount` 会使部分用户路由改变；仅改配置可能让原 key 无法按新路由找到。已有数据须制定版本化 key、旧路由读取与迁移或重建方案，并保持发布期间读写配置一致。本工具不自动迁移，也不在错误时回退到默认分片数。

定向验证使用修改前的路由结果、边界值及并发调用，不连接真实 Redis 集群或迁移已有数据。
