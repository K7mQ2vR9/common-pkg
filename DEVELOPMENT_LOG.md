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

- 建档基线为 `pre` 分支上的 `2b03a40`。
- Go 模块声明为 `github.com/K7mQ2vR9/base-pkg`。本地开发目录仍为 `common-pkg/`，远端仓库和发布版本待第 7 项处理。

## 待办与进度

| 序号 | 修复项 | 状态 |
|---|---|---|
| 1 | JWT 签发、验签和刷新校验 | 已修改、验证并确认 |
| 2 | 删除源码与仓库中的历史凭证，确认轮换范围 | 工作区清理已验证并确认，部署凭证用途待核对 |
| 3 | 请求与 RPC 日志中的凭证脱敏 | 已修改、验证并确认 |
| 4 | 客户端 IP 的可信代理校验 | 已修改、验证并确认 |
| 5 | Kafka v2 死信处理和位点提交 | 已修改、验证并确认 |
| 6 | Kafka v2 消费和关闭流程 | 已修改并验证，待确认 |
| 7 | Go 模块路径与独立发版 | 模块路径已按用户要求修改并验证，远端及发版待处理 |
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

### 2026-11-17 / DEV-20261117-005 / 第 4 项客户端 IP 可信代理校验

- **进度** 用户要求进入下一项，第 3 项记为已确认。第 4 项已修改并验证，等待确认。
- **改动** HTTP 入口默认使用连接对端 IP。仅当对端命中已配置的可信代理 CIDR 时，从右向左检查 `X-Forwarded-For` 链，取第一个不可信的有效 IP。若整条链均为可信节点，则取最左侧有效 IP。可信段内出现无效内容时回退到连接对端。`ClientIp` 和 `AdminClientIp` 都使用同一解析结果，`GetAdminClientIp` 拒绝包含多个地址或格式无效的 metadata。
- **接入** 可在 `app.ConfigAppServer` 中配置 `TrustedProxyCIDRs`，调用 `app.WithTrustedProxyCIDRs(config.TrustedProxyCIDRs)` 检查错误后，将返回的选项传入 `app.NewServerMiddleware`。单独使用 HTTP 头中间件时可调用 `appMiddleware.WithTrustedProxyCIDRs`。只配置会在 `X-Forwarded-For` 末尾追加连接对端 IP 的代理。IP 解析采用 `X-Forwarded-For`，原有 `CF-Connecting-IP`、`TZ-Connecting-IP` 和 `X-Real-IP` 不参与取值。
- **验证** 使用临时 Go overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -v ./app -run '^Test(TrustedProxyResolution|TrustedProxyMiddlewareConfiguration|AdminClientIPMetadata)$' -count=1`，三组通过。覆盖默认直连、可信与不可信对端、多级与多行转发链、IPv6、IPv4 映射地址、异常链回退、配置校验及 HTTP metadata 传递。`go build -mod=readonly -buildvcs=false ./...` 和 `git diff --check` 通过。测试使用内存请求，启动时提示缺少 `etc/language`，未连接实际代理或变更部署配置。
- **测试清理** 登记结果后已删除临时测试用例、overlay 配置和构建缓存，并核对工作区无本次新增的测试文件。
- **下一步** 等用户确认第 4 项后处理第 5 项 Kafka v2 死信和位点提交。

### 2026-11-17 / DEV-20261117-006 / 更新公共包 Go 模块路径

- **用户要求** 将 `headInfo` 的导入路径改为 `github.com/K7mQ2vR9/base-pkg/headInfo`。
- **改动** 将 `go.mod` 的模块路径改为 `github.com/K7mQ2vR9/base-pkg`，同步更新包内 Go 源码和现有测试的导入路径。正式发布仍待处理。
- **验证** `go list -mod=readonly -f '{{.ImportPath}}' ./headInfo` 返回新路径。`go build -mod=readonly -buildvcs=false ./...` 通过。`go test -mod=readonly -buildvcs=false ./... -run '^$' -count=1` 编译了测试包，未执行测试用例。独立临时 Go 模块通过本地 `replace` 导入并调用新路径的 `headInfo.GetHead`，运行通过。`git diff --check` 通过。
- **测试清理** 登记验证结果后删除临时导入验证模块及构建缓存。核对工作区无本次新增的测试文件。
- **限制与下一步** 本次验证使用本地源码，Git `origin` 仍为 `github.com/K7mQ2vR9/common-pkg`。远端仓库和版本发布未调整，尚未验证通过 GitHub 下载新模块。第 4 项仍等待确认。

### 2026-11-17 / DEV-20261117-007 / 第 5 项 Kafka v2 死信与位点提交

- **进度** 用户要求进入下一项，第 4 项记为已确认。第 5 项已修改并验证，等待确认。
- **改动** 分区 worker 只提交按拉取顺序连续完成的消息。业务重试耗尽后，死信投递失败会保留失败消息和后续消息；重试该消息时只重试死信投递。已完成的连续前缀必须等 broker 确认提交后才清除。提交失败后优先重试提交，避免在同一 worker 内重复执行业务或投递死信。失败期间暂停接收新消息到该 worker 的批次，保留批次边界。单条消费入口直接按顺序处理消息。
- **接入** 消费者须设置 GroupID。可通过 `WithDLQTopic` 配置同集群中已存在的独立死信主题，内置生产者同步等待 Kafka 副本确认，并在死信消息附上原主题、分区和位点。`WithDLQHandler` 优先，回调应在持久化成功后返回 nil。没有配置死信处理时，处理失败的消息保持未提交。
- **验证** 使用临时 Go overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -v ./amqp/kafka/v2 -run '^Test(DLQFailureBlocksLaterOffsets|CommitFailureRetainsProgress|MissingDLQDoesNotCommit|KafkaDLQPublisher)$' -count=1`，四组通过。覆盖死信失败后提交已完成前缀、恢复后继续、批量处理和死信各自成功但提交失败、缺少 DLQ、生产者配置及消息封装。`go build -mod=readonly -buildvcs=false ./...` 和 `git diff --check` 通过。未连接真实 Kafka broker，未验证重平衡或进程退出后的重投。批量处理器仍需幂等，进程崩溃或 broker 回执丢失时可能重投。
- **测试清理** 登记结果后已删除临时测试用例、overlay 配置和构建缓存，并核对工作区无本次新增的测试文件。
- **下一步** 等用户确认第 5 项后处理第 6 项 Kafka v2 消费和关闭流程。

### 2026-11-17 / DEV-20261117-008 / 第 6 项 Kafka v2 消费和关闭流程

- **进度** 用户要求进入下一项，第 5 项记为已确认。第 6 项已修改并验证，等待确认。
- **改动** 消费者只允许运行一个拉取与分发循环。`Close()` 取消拉取和投递并等待收尾，由消费循环退出后关闭 worker 队列。worker 按分区排空已接收的消息，在 Reader 和 DLQ Writer 仍可用时提交已完成位点。Reader 返回 `io.EOF` 时进入收尾，临时读取错误的重试等待可被取消。重复或并发关闭等待同一次收尾，Reader 和 Writer 的关闭错误纳入消费返回结果。
- **接入** `WithShutdownTimeout` 可设置缓冲处理的时限，默认 10 秒。超时会取消 worker 上下文并由消费方法返回超时错误，未完成的位点保持未提交。业务及死信回调需响应上下文取消。`Close()` 由消费回调之外的调用方调用。
- **验证** 使用临时 Go overlay，执行 `go test -race -mod=readonly -buildvcs=false -overlay <临时配置> ./amqp/kafka/v2 -run '^Test(CloseStopsBlockedDispatchBeforeDraining|ReaderEOFDrainsPartialBatch|ShutdownTimeoutCancelsWorkerWithoutCommit|CancelInterruptsFetchRetryAndDuplicateStart|CallerDeadlineAndCloseBeforeStart)$' -count=3`，五组通过。另以相同参数运行 `-run '^TestShutdownRetriesPendingDLQBeforeDraining$' -count=3`，验证关闭时先恢复死信并提交，再处理后续缓冲消息。`go build -mod=readonly -buildvcs=false ./...` 和 `git diff --check` 通过。
- **限制** 使用模拟 Reader 和回调，未连接真实 Kafka broker。多个分区共享拉取与分发循环，故障分区缓冲填满时仍会产生跨分区背压。与 Kafka broker 的重平衡和关闭期间的重投待联调。
- **测试清理** 登记结果后已删除临时测试用例、overlay 配置和构建缓存，并核对工作区无本次新增的测试文件。
- **下一步** 等用户确认第 6 项后处理第 7 项剩余的远端和独立发版工作。
