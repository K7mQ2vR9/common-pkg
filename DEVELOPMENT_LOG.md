# common-pkg 开发记录

本文件记录平台子项目 `common-pkg` 的开发进度、验证结果和待确认事项。日期采用 `Asia/Ho_Chi_Minh`，按日期追加记录并关联修复序号。

按下表序号逐项修改。每项完成后记录具体改动和实际验证结果，等用户确认后再开始下一项。

## 平台架构

| 板块 | 职责 |
|---|---|
| `common-pkg` | 平台公共包，为 RPC 和 API 等项目提供基础框架能力。 |
| RPC 层 | 对上层提供数据模型服务，通过 RPC 接口提供数据读取和写入等操作。 |
| API 层 | 面向用户提供 API，在这一层组织业务流程，调用 RPC 数据模型服务完成业务操作。 |

用户请求进入 API 层，API 层按业务流程调用 RPC 层；API 和 RPC 各自复用 `common-pkg` 提供的基础能力。

## 当前基线

- 分支为 `pre`，建档时的提交为 `2b03a40`。第 1 项代码变更仍在工作区。
- Go 模块当前声明为 `github.com/doubunv/common-pkg`。正式发布路径和版本号待第 7 项处理。

## 待办与进度

| 序号 | 修复项 | 状态 |
|---|---|---|
| 1 | JWT 签发、验签和刷新校验 | 已修改、验证并确认 |
| 2 | 删除源码与仓库中的历史凭证，确认轮换范围 | 工作区清理已验证并确认，部署凭证用途待核对 |
| 3 | 请求与 RPC 日志中的凭证脱敏 | 已修改并验证，待确认 |
| 4 | 客户端 IP 的可信代理校验 | 待处理 |
| 5 | Kafka v2 死信处理和位点提交 | 待处理 |
| 6 | Kafka v2 消费和关闭流程 | 待处理 |
| 7 | Go 模块路径与独立发版 | 待处理 |
| 8 | 认证中间件的回调、白名单和错误处理 | 待处理 |
| 9 | 统一响应和特殊错误匹配 | 待处理 |
| 10 | 多语言转换和 JSON 序列化兼容 | 待处理 |
| 11 | 请求头校验、CORS 和并发上限 | 待处理 |
| 12 | 数据库连接错误、Redis 客户端类型和日志关闭 | 待处理 |
| 13 | AES-GCM nonce、密钥校验和加密失败处理 | 待处理 |
| 14 | gRPC 客户端成功分支、服务端认证和状态码 | 待处理 |
| 15 | RabbitMQ 队列绑定、重连和重新订阅 | 待处理 |
| 16 | 搜索客户端 TLS 与响应体关闭、XXL-JOB 监听生命周期 | 待处理 |
| 17 | HTTP 请求、随机字符串、TOTP URI 和 Redis 分片工具 | 待处理 |

## 开发记录

### 2026-11-17 / DEV-20261117-001 / 第 1 项 JWT 签发、验签和刷新校验

- **状态** 已修改并完成定向验证，待用户确认。
- **改动** [`jwtToken/jwt.go`](jwtToken/jwt.go) 统一将调用方配置的密钥转换为 `[]byte`，拒绝空白或少于 32 字节的密钥，验签仅允许 `HS256`。刷新只接受未过期的管理员或用户访问令牌。保留三个公开函数的参数和返回类型。
- **验证** 使用临时 Go overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> ./jwtToken -count=1`，通过。覆盖两个有效令牌类型、错误密钥、空白和过短密钥、非 `HS256` 算法、未知或缺失令牌类型、过期或缺少过期时间。`git diff --check` 通过。临时测试和缓存已清理。
- **下一步** 等用户确认后开始第 2 项凭证清理。

### 2026-11-17 / DEV-20261117-002 / 确认平台分层和子项目职责

- **用户确认** 平台由公共 `common-pkg`、RPC 数据模型服务和面向用户的 API 三个板块协作组成。API 层调用 RPC 层实现业务逻辑。
- **改动** 在本文件记录三层职责和调用关系，明确本开发记录的范围为 `common-pkg` 子项目。
- **修复进度** 第 1 项 JWT 仍待确认，第 2 项凭证清理尚未开始。

### 2026-11-17 / DEV-20261117-003 / 第 2 项历史凭证清理

- **进度** 用户要求进入下一项，第 1 项记为已确认。第 2 项工作区清理和验证已完成，等待确认。
- **改动** 删除 `jwtToken/jwt.go` 中注释里的旧 JWT 签名密钥，以及仓库根目录的 `y`、`y.pub` SSH 密钥文件。OpenSearch 集成测试改从 `COMMON_PKG_OPENSEARCH_TEST_URL`、`COMMON_PKG_OPENSEARCH_TEST_USERNAME`、`COMMON_PKG_OPENSEARCH_TEST_PASSWORD` 读取配置，缺失时跳过。XXL-JOB 示例改从 `XXL_JOB_ADDRESS` 和 `XXL_JOB_ACCESS_TOKEN` 读取连接配置，缺失时退出。
- **验证** 使用本地依赖缓存执行 `go build -mod=readonly -buildvcs=false ./...`，构建通过。`go test -mod=readonly -buildvcs=false -v ./es/test -run '^TestLogInfo$' -count=1` 验证空配置时跳过。构建并运行 XXL-JOB 示例，确认缺少环境变量时拒绝启动。对照清理前提交，静态核查已移除的凭证值未出现在当前工作区文件，`git diff --check` 通过。未连接外部服务。
- **测试清理** 未创建测试用例。登记验证结果后已删除临时构建缓存和 XXL-JOB 示例二进制，并核对清理结果。
- **待核对** 历史 Git 提交和已发布或下载的归档中可能仍有旧凭证。部署或外部系统是否仍信任旧 JWT 密钥、SSH 密钥、OpenSearch 账号和 XXL-JOB Token，需要由对应环境的管理方确认。仍有效的凭证需要撤销或轮换。本次未改写 Git 历史或变更部署配置。
- **下一步** 等用户确认第 2 项后处理第 3 项日志凭证脱敏。

### 2026-11-17 / DEV-20261117-004 / 第 3 项请求与 RPC 日志凭证脱敏

- **进度** 用户要求进入下一项，第 2 项工作区清理记为已确认，部署凭证的用途和轮换范围仍待核对。第 3 项已修改并验证，等待确认。
- **改动** `headInfo` 移除 HTTP 日志上下文中的 JWT 字段，`Head.String()` 从凭证已清空的副本生成日志字符串。RPC 服务端按允许列表记录链路、站点、请求路径等已知字段，排除凭证字段及未列入的自定义 metadata。HTTP 认证和 RPC 处理器继续读取原始 Token，RPC 入站及出站 metadata 保持原有内容。
- **验证** 使用临时 Go overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -v ./appMiddleware -run '^Test(HTTPLogRedaction|RPCMetadataLogRedaction)$' -count=1`，两项通过。覆盖 `Head.String()`、HTTP 认证及日志字段、RPC 日志允许列表及原始 metadata 保留。测试使用内存请求和模拟凭证，启动时提示缺少 `etc/language`，未执行外部服务联调。`go build -mod=readonly -buildvcs=false ./...` 和 `git diff --check` 通过。
- **测试清理** 登记结果后已删除临时测试用例、overlay 配置和构建缓存，并核对工作区无本次新增的测试文件。
- **下一步** 等用户确认第 3 项后处理第 4 项可信代理校验。
