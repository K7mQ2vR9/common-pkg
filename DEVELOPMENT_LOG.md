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

当前请求链路统一采用 Request ID = Trace ID，生成和接入规则见 [统一请求 ID 规则](headInfo/TRACE_ID.md)。历史记录中分别生成或允许任意 Request ID 的约定已由此规则替换。

## 当前基线

- 建档基线为 `pre` 分支上的 `2b03a40`。
- Go 模块声明为 `github.com/K7mQ2vR9/base-pkg`，首版 `v0.0.0` 已发布到同名 GitHub 私有仓库。本地 `common-pkg/` 仍是旧仓库的工作区。

## 待办与进度

| 序号 | 修复项 | 状态 |
|---|---|---|
| 1 | JWT 签发、验签和刷新校验 | 已修改、验证并确认 |
| 2 | 删除源码与仓库中的历史凭证，确认轮换范围 | 工作区清理已验证并确认，部署凭证用途待核对 |
| 3 | 请求与 RPC 日志中的凭证脱敏 | 已修改、验证并确认 |
| 4 | 客户端 IP 的可信代理校验 | 已修改、验证并确认 |
| 5 | Kafka v2 死信处理和位点提交 | 已修改、验证并确认 |
| 6 | Kafka v2 消费和关闭流程 | 已修改、验证并确认 |
| 7 | Go 模块路径与独立发版 | `v0.0.0` 已发布并验证下载，用户要求跳过后续处理 |
| 8 | 认证中间件的回调、白名单和错误处理 | 已修改、验证并确认，包含登录用户信息扩展 |
| 9 | 统一响应和特殊错误匹配 | 已修改、验证并确认，包含跨 RPC Trace ID |
| 10 | 多语言转换和 JSON 序列化兼容 | 已恢复自动匹配、验证并确认 |
| 11 | 请求头校验、CORS 和并发上限 | 已完成确认范围内的 CORS 和并发上限调整 |
| 12 | 数据库连接错误、Redis 客户端类型和日志关闭 | MySQL、Redis 和日志关闭均已完成、验证并确认 |
| 13 | AES-GCM nonce、密钥校验和加密失败处理 | 已完成、验证并确认，使用进程级配置开关 |
| 14 | gRPC 客户端成功分支、服务端认证和状态码 | 已完成、验证并确认 |
| 15 | RabbitMQ 队列绑定、重连和重新订阅 | 已实现重试、TG 通知、统一链路、处理超时、业务消息接口、消费错误分类和终态双份确认，已验证；后续完善逐项确认 |
| 16 | 搜索客户端 TLS 与响应体关闭、XXL-JOB 监听生命周期 | Elasticsearch 7、OpenSearch、XXL-JOB 已完成并确认 |
| 17 | HTTP 请求、随机字符串、TOTP URI 和 Redis 分片工具 | 已完成、验证并确认 |
| 18 | 搜索模型 UUID 旧引用与全量编译恢复 | 已完成、验证并确认 |
| 19 | 时间工具独立管理、UTC 默认值与日历边界 | 已开发并验证，待确认 |

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

### 2026-11-17 / DEV-20261117-009 / 第 7 项发布 base-pkg v0.0.0

- **进度** 用户要求进入下一项，第 6 项记为已确认。第 7 项已发布并验证，等待确认。
- **发布** 新建 [K7mQ2vR9/base-pkg](https://github.com/K7mQ2vR9/base-pkg) 私有仓库。由本地源码提交 `f753d52` 生成快照，排除 `.DS_Store`，以独立的首个提交 `4726dbab6bcfcc481d769322fd5252033a005805` 创建远端 `main` 和 `v0.0.0`。旧提交历史和旧 tag 留在原仓库。
- **验证** 发布前检查快照中的原 JWT、OpenSearch、XXL-JOB 和 SSH 凭证特征，通过。快照的 `go build -mod=readonly -buildvcs=false ./...`、`go test -mod=readonly -buildvcs=false ./... -run '^$' -count=1` 和 Git 差异检查通过。测试命令只编译测试包。远端 `git ls-remote` 确认 `main` 和 `v0.0.0` 指向发布提交。首次下载触发环境中已有的公开校验库配置，校验失败。以只针对本模块的 `GOPRIVATE`、`GONOPROXY` 和 `GONOSUMDB` 例外及 SSH 权限重跑，`go mod download -json github.com/K7mQ2vR9/base-pkg@v0.0.0` 从空缓存下载成功，`go list -m -f '{{.Version}}' github.com/K7mQ2vR9/base-pkg@latest` 返回 `v0.0.0`。
- **用户决定** 收到跳过第 7 项的指令时，远端仓库、`main` 和 `v0.0.0` 已经创建，且已验证下载。按最新要求停止第 7 项的后续操作，保留已发布的版本。
- **测试清理** 登记验证后清理了测试缓存、临时发布快照和验收下载目录。Go 下载缓存中的只读文件通过 `GOMODCACHE=<临时目录> go clean -modcache` 清理。核对临时目录不存在，工作区无本次新增测试文件。
- **当前状态** 本地旧工作区的 `origin` 仍指向 `K7mQ2vR9/common-pkg`。第 8 项认证中间件调整待开始。

### 2026-11-17 / DEV-20261117-010 / 第 8 项认证中间件统一

- **进度** 用户要求开始第 8 项。代码已修改并通过定向验证，等待确认。
- **改动** 旧版和带错误回调的认证共用同一个流程。白名单支持原有的精确匹配和尾部 `*` 前缀匹配。未提供 Token 或认证失败时以匿名身份继续，并清除上下文中原有的用户 ID。认证成功时写入已验证的用户 ID。受保护接口会拒绝认证失败，响应使用固定提示，特殊错误沿用原业务码，权限拒绝不包含内部路径。
- **接入** `app.WithCheckTokenHandleWithErrSMOption` 启用带错误的回调，旧的 `WithCheckTokenHandleSMOption` 继续可用。两种选项都传入时以后配置的为准。`app` 装配和各认证、服务状态中间件在初始化时检查必需回调。服务状态检查沿用白名单，独立调用时缺少回调返回服务不可用。
- **验证** 使用临时 Go overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -v ./app -run '^Test(AuthWhiteListAndAppOptions|AuthErrorsUseSafeMessages|MissingChecksFailDuringAssembly|ServiceStatusGuard)$' -count=1`，四组通过。覆盖新旧装配、匿名与已认证请求、白名单匹配、旧身份清除、特殊码、错误文本隔离、启动时缺少回调及状态检查。测试启动时提示缺少 `etc/language`，验证在本地内存请求中完成。`go build -mod=readonly -buildvcs=false ./...` 和 `git diff --check` 通过。
- **测试清理** 登记验证结果后已删除临时测试用例、overlay 配置和构建缓存，并核对工作区无本次新增测试文件。
- **下一步** 等用户确认后开始第 9 项统一响应和特殊错误匹配。

### 2026-11-18 / DEV-20261118-001 / 第 8 项补充登录用户信息 JSON

- **用户要求** 认证中间件注入当前登录用户的扩展 JSON，供业务使用。
- **改动** 新增 `app.WithCheckTokenHandleWithUserInfoSMOption` 和 `appMiddleware.WithCheckOptionWithUserInfo`，回调可返回用户 ID、可选的 `json.RawMessage` 和错误。只有认证成功且扩展 JSON 格式有效时才注入。业务通过 `headInfo.GetUserInfo(ctx)` 获取独立副本，范围为当前 HTTP 请求上下文，避免自动进入 RPC metadata、请求头及日志字段。匿名访问和使用旧回调时清空旧扩展，现有认证回调签名继续可用。
- **验证** 使用临时 Go overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -v ./app -run '^Test(AuthenticatedUserInfoFromCallback|InvalidAuthUserInfoIsRejected|ExistingAuthCallbacksStillWork)$' -count=1`，三组通过。覆盖认证后注入、忽略外部同名头和 metadata、扩展副本隔离、匿名清除、非法 JSON 拒绝以及旧回调兼容。测试启动时提示缺少 `etc/language`，验证在本地内存中完成。`go build -mod=readonly -buildvcs=false ./...` 和 `git diff --check` 通过。
- **测试清理** 登记验证结果后已删除临时测试用例、overlay 配置和构建缓存，并核对工作区无本次新增测试文件。
- **下一步** 等用户确认第 8 项后开始第 9 项统一响应和特殊错误匹配。

### 2026-11-18 / DEV-20261118-002 / 第 9 项统一 HTTP 错误响应和特殊错误匹配

- **进度** 用户要求进入下一项，第 8 项记为已确认。第 9 项已修改并验证，等待确认。
- **改动** `result.HttpErrorResult` 和 `logs/xcode.ErrHandlerCtx` 共用 HTTP 错误响应生成器。`app.InitAppServer` 注册带上下文的 go-zero 全局错误处理器。沿用 HTTP 200 和响应体业务码，统一 `code`、`message`、`data`、`trace`。普通错误和 5xx 错误仅展示固定提示；业务错误读取显式公开的 `Message()`，底层诊断留在服务端日志。错误响应仅翻译 `message`。旧 `ErrHandler` 签名继续可用，调用方需使用带上下文入口才能获得请求 `trace`。
- **特殊错误** `errors.Is` 按特殊错误码匹配，`IsSpecialNotifyError` 仅识别具有通知码 410 的特殊错误，支持包裹和解包底层原因。特殊错误仅公开通用文案。原解密失败和请求缺少加密数据使用明确的 400 响应体业务码及固定提示。
- **验证** 使用临时 overlay 执行 `go test -c -mod=readonly -buildvcs=false -overlay <临时配置> -o <临时二进制> ./app`，在临时语言目录中运行 `./app-response.test -test.run '^Test(HTTPErrorResponsesAgree|SpecialErrorsMatchByCode)$' -test.count=1`，两组通过。覆盖中间件与 go-zero 输出一致、普通和 5xx 错误脱敏、业务错误和特殊错误码、`trace`、文案翻译、空错误、错误链及解密提示。`go build -mod=readonly -buildvcs=false ./...` 和 `git diff --check` 通过。验证采用内存 HTTP 请求，未发布或联调外部服务。
- **测试清理** 登记验证结果后已删除临时测试用例、语言样本、overlay 配置、测试二进制和构建缓存，并核对工作区无本次新增测试文件。
- **下一步** 等用户确认第 9 项后开始第 10 项多语言和 JSON 序列化兼容。

### 2026-11-18 / DEV-20261118-003 / 第 9 项补充跨 API 与 RPC Trace ID

- **用户要求** 同一 API 请求及其多个 RPC 调用使用一致的 Trace ID。
- **改动** HTTP 入口沿用有效的 OpenTelemetry 上下文或 W3C `traceparent`，两者均缺失时初始化链路。`X-Request-ID` 作为独立的 `request_id` 保留。RPC 客户端向下游注入 `traceparent` 和同值的兼容 `trace` metadata，服务端以 W3C 链路为准规范入站和出站 `trace`。响应及 go-zero 结构化日志使用同一个 OpenTelemetry Trace ID。同步修复 RPC 成功调用后空错误引用触发的 panic，原错误映射沿用既有规则。
- **验证** 使用临时 overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> ./appMiddleware -run '^Test(TraceAcrossHTTPAndMultipleRPCs|ExistingW3CTraceIsPreserved)$' -count=3`，通过。内存 gRPC 覆盖 API 分别调用 RPC A、RPC B 及 A 再调用 B，并对比 HTTP 成功与错误响应、RPC metadata 和结构化日志。首轮测试捕获器未实现 Error 方法而失败，补齐后通过。`go build -mod=readonly -buildvcs=false ./...` 和 `git diff --check` 通过。未连接部署环境或验证链路导出。
- **测试清理** 登记验证后已删除临时测试用例、overlay 配置及构建缓存，并核对临时目录已移除。
- **后续开发** 用户明确要求以后先讨论方案，经确认后再开发。已记录在平台 `AGENTS.md`。第 10 项仍待方案讨论。

### 2026-11-18 / DEV-20261118-004 / 核对网关请求 ID 的 API 与 RPC 传递

- **用户确认** 网关优先保留前端提供的非空 `X-Request-ID`，缺失或为空时生成一次再传给 API。请求 ID 与 OpenTelemetry Trace ID 分别传递。
- **现有实现** API 优先读取 `X-Request-ID`，以 `request_id` 写入 RPC metadata。RPC 收到后继续向下游传递同一个值。现有传递逻辑已满足该规则的 API 和 RPC 部分。
- **验证** 使用临时模拟网关和内存 RPC 拦截器，执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -v ./appMiddleware -run '^TestGatewayRequestIDAcrossAPIAndRPC$' -count=3`，通过。覆盖保留前端 ID、缺失时透传网关生成的 ID、连续请求生成不同的 ID、一次 API 请求内分别调用两个 RPC 以及嵌套 RPC，验证 `trace` 独立且一致。`git diff --check` 通过。测试启动时提示缺少 `etc/language`，未连接实际网关。
- **测试清理** 登记验证后删除临时测试用例、overlay 配置和构建缓存，并核对临时目录已移除。
- **待接入** 当前平台工作区未找到网关配置。待确认实际网关种类和配置路径后接入生成规则并联调。

### 2026-11-18 / DEV-20261118-005 / 第 10 项多语言和 JSON 序列化

- **用户确认** 新建独立的 [language-pkg](../language-pkg/DEVELOPMENT_LOG.md)。从 `content_language` 读取请求语言，统一翻译错误 `message` 和业务显式选中的 `data` 字段值。
- **改动** `TranslateMessage` 将常用错误文案映射为稳定 Key，也接受词典 Key。原有 `SwitchLanguage` 入口改为调用业务类型的 `Localize(locale string) any`，其他响应直接交给原有编码流程。明文和 AES 响应共用翻译入口，JSON 编码由业务响应类型负责。
- **迁移** 如已有部署使用 `etc/language`，将词条迁移到 `language-pkg/locales` 的 JSON 文件。成功响应过去依赖自动遍历翻译的字段，需要由业务显式选择。
- **验证** 使用临时 overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^Test(LocalizedResponseData|UnmarkedResponseData|LocalizedErrorResponse|DictionaryLookup)$' -count=1 -v ./common-pkg/result ./language-pkg`，四组通过。覆盖中英文和缺少词条时的回退、AES 解密后字段值、原始业务状态、`json:"-"`、`omitempty`、自定义 `MarshalJSON`、空响应及错误码与 Trace ID。工作区 `go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...`、`go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1` 和 `git diff --check` 通过。
- **验证限制** 现有测试只做全量编译，临时用例在内存 HTTP 中运行。默认构建缓存不可写，最初工作区依赖解析也缺少本地版本和一个公开间接依赖。使用工作区 `replace`、临时缓存和 Go 校验和验证的公开依赖完成构建。模块独立发布和服务联调待进行。
- **测试清理** 登记验证后删除本次临时测试、overlay 配置、下载文件和 Go 缓存。核对临时目录已移除，项目原有五个测试文件保留。

### 2026-11-18 / DEV-20261118-006 / 第 10 项恢复自动匹配词典

- **用户确认** 响应数据恢复自动翻译。业务在展示字段中填入词典 Key，成功响应按请求语言匹配，沿用 `id` 和 `code` 的跳过规则。
- **改动** `SwitchLanguage` 先按响应类型的规则编码结构化数据，再只替换匹配词典的 JSON 字符串值。匹配覆盖嵌套对象和数组，按完整字符串匹配，字段名及大小写不敏感的 `id` 和 `code` 字符串值保持原样。原有字段标签、自定义 `MarshalJSON`、原始数字精度以及 JSON 中未匹配的值由原序列化结果保留。顶层字符串可直接翻译，字节切片沿用原先的明文和 AES 编码方式。移除第 10 项早期版本引入的 `Localize` 方法要求。
- **接入** 业务返回 `{"status":"pending","status_label":"order.status.pending"}`，中文响应输出 `"status_label":"待支付"`，英文输出 `"status_label":"Pending payment"`。未知语言或未配置的 Key 保留原值。
- **验证** 使用临时 overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestAutomatic(DataLocalization|ScalarResponses|LocalizationErrorEnvelope)$' -count=1 -v ./common-pkg/result`，三组通过。覆盖中英文、空语言和未知语言、明文与 AES、嵌套匹配、字段名和受保护字段、`omitempty`、`json:"-"`、自定义序列化、大数和指数原值、顶层字符串与字节、空值、错误码及 Trace ID。
- **验证限制** 工作区 `go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...` 和 `go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1` 通过。现有测试只做全量编译，定向测试使用内存 HTTP 和测试密钥。工作区从本机缓存和临时代理读取依赖，缺少的公开依赖从 Go 公共代理下载并由 `go.work.sum` 校验。发布和部署联调待进行。
- **测试清理** 登记验证后已删除临时测试、overlay、下载文件、日志及 Go 缓存。核对临时目录已移除，两模块无本次新增的测试文件，原有五个测试文件保留。

### 2026-11-18 / DEV-20261118-007 / 第 11 项 CORS 请求头和实例并发上限

- **用户确认** CORS 允许浏览器在预检中列出的全部请求头。API 实例共享一个可配置的并发上限。超限立即返回 HTTP `503`，预检在并发限制之前处理。
- **CORS** `OPTIONS` 响应返回 `204`，把请求的 `Access-Control-Request-Headers` 写入 `Access-Control-Allow-Headers`，并加上 `Vary: Access-Control-Request-Headers`。已覆盖 `Authorization` 和业务自定义头。已注册的 `OPTIONS` 路由经过 API 中间件，无 `OPTIONS` 路由的已注册路径可在创建 go-zero Server 时接入 `rest.WithNotAllowedHandler(appMiddleware.NewCorsMiddleware().Handler())`。该回退处理器对其他方法返回 `404`。已有自定义回退的服务可用 `CorsMiddleware.Handle` 包装原回退逻辑。业务请求的必填头校验与认证继续由各自的中间件处理。
- **并发** `ConfigAppServer.MaxConcurrency` 默认 `0`，通过 `app.WithMaxConcurrency` 接入 API 中间件。正数表示当前 Server 实例跨路由共享的名额，处理结束后释放。负数返回配置错误。超限使用统一 JSON 响应体，`code` 为 `503`，`message` 可按请求语言翻译，`trace` 复用当前请求的 Trace ID。
- **接入顺序** 先调用 `app.WithMaxConcurrency(appConf.MaxConcurrency)` 并处理返回的错误。实例级限制启用时，在创建 go-zero Server 前设置 `restConf.Middlewares.MaxConns = false`。创建 Server 时配置需要的 CORS 回退，然后将并发选项传入 `app.NewServerMiddleware(server, ...)` 并调用 `ApiUseMiddleware()`。
- **验证** 使用临时 overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^Test(SharedConcurrencyLimitAcrossRoutes|ConcurrencyAndCORSConfiguration)$' -count=3 -v ./common-pkg/app`，两组各连续通过三次。覆盖两路由共享两个名额、第三个请求的状态码及响应体、中文文案和 Trace ID、释放名额后放行、饱和时的预检、显式 `OPTIONS` 路由与 go-zero 回退、多个请求头字段、默认值和负数配置。
- **验证限制** `go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...`、`go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1` 及 `git diff --check` 通过。定向验证使用实际 go-zero 路由和内存 HTTP 请求，现有测试只做全量编译，真实浏览器、网关和部署并发值待联调。工作区依赖使用临时缓存，缺少的公开间接依赖由 `go.work.sum` 校验。
- **测试清理** 登记验证后已删除临时用例、overlay、下载文件、日志和缓存。核对临时目录不存在，工作区保留原有五个测试文件。

### 2026-11-18 / DEV-20261118-008 / 第 12 项 MySQL 连接错误和上下文

- **用户确认** 新项目直接替换旧 MySQL 接口。本轮先完成 MySQL 连接和上下文。
- **启动** `ConnectMySQL(ctx, dataSource, logLevel)` 通过同一个启动上下文执行 `PingContext` 和 `SELECT VERSION()`。每个连接池最多有 5 秒，调用方的更早截止时间或取消优先。GORM 用检测到的服务端版本配置方言。错误保留可供 `errors.Is` 或 `errors.As` 判断的原因，默认错误文本省略 DSN 和驱动的原始错误。
- **接入** 用 `NewSelectDb(startupCtx, config.SelectDb, logLevel)` 初始化连接池。空或重复的 `DbKey` 提前报错。任一库初始化失败会关闭已打开的池并返回带 `DbKey` 的错误。业务使用 `GetDb(requestCtx, dbKey)` 获取绑定请求上下文的 GORM 会话，查询和事务继承该上下文。服务退出时调用 `SelectDb.Close()`。
- **验证** 使用临时 overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestMySQL(StartupDeadline|SelectionCleanup|RequestContext)$' -count=1 -v ./common-pkg/dbM`，三组通过。覆盖探活和版本探测卡住、默认 5 秒和调用方更短的超时、第二库失败后的池清理、错误脱敏、版本和 DSN 时区保留、启动上下文结束后继续查询、请求取消和查询超时。工作区 `go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...`、`go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1` 及 Git 差异检查通过。
- **验证限制** 本机执行 `mysqld --no-defaults --initialize-insecure --datadir=<临时目录> --log-error=<临时日志> --tmpdir=<临时目录>` 时 MySQL 9.7.1 初始化进程收到 SIGSEGV。定向测试改用真实 Go MySQL 驱动连接内存协议模拟端，真实 MySQL 联调待进行。现有测试只做全量编译。
- **测试清理** 登记验证后已删除临时用例、overlay、MySQL 临时数据目录和日志、下载文件与 Go 缓存。核对临时目录已移除，原有五个测试文件保留。

### 2026-11-18 / DEV-20261118-009 / 第 12 项 Redis 单机和集群客户端

- **用户确认** 延续新项目规则，直接替换旧的 `RedisConnect`，按配置创建单机或集群客户端。
- **配置** `ConfigAppServer.RedisConf` 接收 `dbM.RedisConfig`。默认 `Type: node`，通过 `Addr` 配置单机地址和非负 `DB`。`Type: cluster` 使用 `Addrs` 中的种子地址并要求 `DB: 0`。一个种子地址也会创建集群客户端。地址采用 `host:port`。开启 `TLS` 后默认校验证书，`InsecureSkipVerify` 仅在明确启用 TLS 时可设置。
- **接入** 有 Redis 配置的服务在启动时调用 `dbM.NewRedisClient(startupCtx, *config.RedisConf)`，处理返回的 `error`。启动 PING 的超时上限为 5 秒并继承更早的调用方截止时间。失败时关闭客户端，错误文本省略原始错误中的配置和服务端细节，原因可通过错误链检查。业务通过 `redis.UniversalClient` 执行命令并传入请求上下文。客户端启用命令 I/O deadline 支持。服务退出时调用客户端的 `Close()`。
- **验证** 使用 `go test -c -mod=readonly -buildvcs=false -overlay <临时配置> -o <临时二进制> ./common-pkg/dbM` 编译定向测试，在本地回环环境执行 `<临时二进制> -test.run '^TestRedis(NodeAndCluster|StartupFailure|ConfigAndTLS)$' -test.v -test.timeout=35s`，三组通过。使用真实 Redis 单机和单节点集群验证客户端类型、单种子集群、认证和独立 DB、读写、业务请求 deadline 和客户端关闭。模拟端验证 PING 超时与失败后的连接回收、错误脱敏，以及 TLS 默认拒绝未受信任证书和显式跳过校验。首轮 TLS 用例限定了底层证书错误类型，更新临时证书和断言后通过。
- **验证限制** `go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...`、`go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1` 及 Git 差异检查通过。现有测试只做全量编译。集群验证使用本机单节点，生产多节点切换和部署配置待联调。
- **测试清理** 定向测试结束时停止并等待临时 Redis 进程退出。登记验证后删除临时用例、overlay、测试二进制、数据、日志、下载文件与 Go 缓存。核对临时目录已移除，原有五个测试文件保留。

### 2026-11-19 / DEV-20261119-001 / 补充应用配置注释

- **用户要求** 为 ConfigAppServer 添加备注，后续改动同步添加注释。
- **改动** 配置结构及各字段增加中文注释，说明默认值、用途和实际接入方式。平台 AGENTS.md 增加后续开发的注释要求。
- **核对** MySQL 当前固定使用 5 秒启动超时、10 个空闲连接、80 个最大连接、30 分钟连接生命周期、10 分钟最大空闲时间。连接参数配置化方案待确认。
- **验证** 执行 gofmt -w app/appServer.go、gofmt -l app/appServer.go 及平台和公共包的 git diff --check，均通过。核对配置字段类型与标签。本次更新为注释和开发规则，未执行构建或运行测试，未创建测试用例及临时产物。

### 2026-11-19 / DEV-20261119-002 / 独立 MySQL 配置与默认值

- **用户确认** MySQL 的连接、连接池、超时和日志参数封装为独立结构体，未配置时走默认值。每个字段补充中文注释。
- **配置与接口** 新增 `dbM.MySQLConfig`。每个 `SelectDbConfig` 通过 `MySQL` 字段组合连接配置。`ConnectMySQL(ctx, config)` 接收 `MySQLConfig`，`NewSelectDb(ctx, configs)` 按各库配置初始化。调用方使用请求上下文调用 `GetDb(ctx, key)`，服务退出时调用 `Close()`。
- **默认值** 启动超时为 5s，驱动拨号超时在 DSN 也未配置时为 5s，读写超时沿用 DSN，默认 0。最大空闲连接为 10、最大连接为 80、连接生命周期为 30m、最大空闲时间为 10m。SQL 日志级别为 warn、慢 SQL 阈值为 200ms、参数占位符默认开启。
- **参数语义** 可选数字、时长及布尔字段用指针区分缺省和显式零值，YAML 和 Go 直接传参应用相同默认值。显式配置的驱动超时覆盖 DSN。最大空闲连接为 0 表示关闭空闲连接保留，最大连接为 0 表示不限数量。连接生命周期、最大空闲时间和慢 SQL 阈值为 0 时，关闭各自的回收或阈值检查。启动超时须为正值，其余时间及连接数须非负。最大连接数为正值时，空闲连接上限须小于或等于它。全部数据库配置在打开第一个连接池前完成校验。
- **验证** 通过临时 overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestMySQLConfig(DefaultsAndYAML|Validation|Connection)$' -count=1 -v ./common-pkg/dbM`，三组通过。覆盖最小 YAML 和 Go 结构体默认值一致、显式 0 和 false、DSN 覆盖、各项非法配置、连接池最大连接数、自定义启动超时、SQL 参数过滤、方言版本和时区保留、启动上下文取消后执行查询，以及连接池关闭。
- **验证限制** `go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...`、`go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1` 和 Git 差异检查通过。定向连接测试使用实际 MySQL 驱动及内存协议模拟端，真实数据库环境的容量压测和联调待进行。现有测试仅做全量编译。
- **测试清理** 登记验证后已删除临时测试用例、overlay、日志、下载文件及 Go 缓存，核对临时目录已移除，原有五个测试文件保留。

最小配置只需填写数据库标识和 DSN，可选字段自动采用默认值。

```yaml
SelectDb:
  - DbKey: main
    MySQL:
      DataSource: "user:password@tcp(127.0.0.1:3306)/platform?parseTime=true&loc=UTC"
```

按库调整参数时，将可选字段写在同一层。

```yaml
SelectDb:
  - DbKey: main
    MySQL:
      DataSource: "user:password@tcp(127.0.0.1:3306)/platform?parseTime=true&loc=UTC"
      StartupTimeout: 5s
      DialTimeout: 5s
      ReadTimeout: 3s
      WriteTimeout: 3s
      MaxIdleConns: 10
      MaxOpenConns: 80
      ConnMaxLifetime: 30m
      ConnMaxIdleTime: 10m
      LogLevel: warn
      SlowThreshold: 200ms
      ParameterizedQueries: true
```

### 2026-11-19 / DEV-20261119-003 / 第 12 项日志关闭

- **用户确认** 修复日志关闭时直接退出进程的问题，验证重复关闭和后续资源清理。
- **改动** LogrusWriter.Close() 正常返回。Logrus 同步写日志，输出及 Hook 的生命周期由配置它们的调用方管理。补充中文注释，说明重复和并发调用的语义。
- **验证** 使用临时 overlay 执行 `go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^Test(LogrusWriterClose|LoggingShutdownContinuesCleanup)$' -count=1 -v ./common-pkg/logs`，两组通过。覆盖重复关闭、16 个并发关闭、已写日志保留、调用方输出资源的生命周期。子进程使用默认退出函数和实际 go-zero 的 logx.Close()，确认正常关闭后仍执行后续代码和 defer 清理。执行 `go build -mod=readonly -buildvcs=false ./common-pkg/logs` 和 Git 差异检查，通过。
- **验证范围** 本轮针对日志包构建和定向测试，使用子进程模拟服务退出收尾。部署环境的完整服务退出流程待联调。
- **测试清理** 登记验证结果后已删除本次临时测试用例、overlay 配置和构建缓存。核对临时目录已移除，项目原有五个测试文件保留。

### 2026-11-19 / DEV-20261119-004 / 第 12 项复查与补充修复

- **用户要求** 重新检查并处理第 12 项。本轮核查 MySQL 配置和请求上下文、Redis 启动取消与清理、日志记录与关闭流程。
- **日志问题与修复** Close() 正常返回的行为通过复核。另发现 Severe() 使用 Logrus.Fatal()，与 go-zero Severe() 的记录行为不一致。修复为使用 FatalLevel 记录并正常返回，使调用方能继续收尾。补充关闭与严重日志的中文注释。
- **Redis 问题与修复** go-redis 的网络读取 deadline 继承了上下文截止时间，但初始化过程中主动取消上下文时，正在进行的读取仍会等待原 I/O 超时。先用临时用例复现取消后仍阻塞的情况，再增加仅在初始化期间生效的 context.AfterFunc 回调，以关闭客户端中断读取。初始化完成即撤销回调，成功的客户端由调用方管理。补充 Redis 配置字段注释。
- **MySQL 核查** YAML 与 Go 直接传参的默认值一致，显式零值与 false 正确保留，结构化超时覆盖 DSN，业务会话绑定请求上下文。查询上下文独立于基础连接池，重复关闭连接池正常返回。
- **定向验证** 临时 overlay 执行日志包的 TestRecheckSevereReturns、TestRecheckCloseAndExtensions、TestRecheckShutdownSubprocess，以及数据库包的 TestRecheckMySQLConfiguration、TestRecheckMySQLRequestContext。子进程验证严重日志后仍能执行正常关闭和 defer。现有的格式化日志、结构化日志和凭证字段过滤通过回归。日志包另以 go test -race 通过。
- **Redis 验证** 编译带 overlay 的 dbM 测试二进制并在本机回环协议端运行 TestRecheckRedisStartupCancellation 和 TestRecheckRedisSuccessfulStartup。覆盖单机与集群初始化取消、连接关闭、成功后取消原启动上下文仍可继续使用客户端，并通过 -race 检测。修复前 Severe() 退出和 Redis 取消阻塞均被用例捕获，修复后通过。
- **执行命令** 以下命令使用本次临时 overlay 和测试二进制。

```text
go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestRecheck(SevereReturns|CloseAndExtensions|ShutdownSubprocess)$' -count=1 -v ./common-pkg/logs
go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestRecheckMySQL(Configuration|RequestContext)$' -count=1 -v ./common-pkg/dbM
go test -race -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestRecheck(SevereReturns|CloseAndExtensions|ShutdownSubprocess)$' -count=1 ./common-pkg/logs
go test -c -race -mod=readonly -buildvcs=false -overlay <临时配置> -o <临时二进制> ./common-pkg/dbM
<临时二进制> -test.run '^TestRecheckRedis(StartupCancellation|SuccessfulStartup)$' -test.v -test.timeout=15s
```

- **构建与限制** go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...、go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1 和 Git 差异检查通过。现有测试做全量编译。本轮数据库请求上下文检查使用 GORM 会话，Redis 使用本地协议端，生产数据库、多节点集群切换和完整部署退出流程待联调。
- **测试清理** 登记验证后已删除临时用例、overlay、测试二进制、日志、下载文件和 Go 缓存。核对临时目录及三份 overlay 目标文件均不存在，原有五个测试文件保留。

### 2026-11-19 / DEV-20261119-005 / 第 13 项 AES-GCM 实例配置及失败处理

- **用户确认** 统一使用随机 nonce，启动校验密钥，按 Server 实例保存配置。序列化及加密失败返回统一错误，完善解密失败和请求体处理。
- **加密实现** 删除固定 nonce 的 EncryptSame，纯函数 Encrypt 和 Decrypt 共用随机 nonce 及认证逻辑。密文采用无 padding 的 URL-safe Base64，字节格式为 nonce、密文和认证标签。解密同时支持带 padding 的输入，并检查 nonce 和认证标签的最小总长度。原有固定 nonce 测试调整为随机 nonce 测试，项目原有测试文件继续保留。
- **实例配置** 新增 aesGCM.Config 和只读 Cipher。Config.Enabled 默认 false，启用后 Config.Key 要求为 16、24 或 32 字节原始 UTF-8 字符串。ConfigAppServer.AESGCM 保存该配置，服务通过 app.WithAESGCM(config.AESGCM) 获取装配选项并处理错误，然后传入 app.NewServerMiddleware。调试模式和加密分别通过各自配置启用。
- **接口迁移** 原来的 WithAesKeyOption、DecryptKeyOption、DecryptWithDebugFalseOption 及全局变量已替换。独立接入解密中间件时调用 NewApiRequestDecryptMiddleware(aesGCM.Config)，处理其返回的错误。加密器通过本地请求上下文供成功响应使用。
- **错误响应** 启用 AES 时，响应序列化失败或随机源读取失败立即返回统一错误响应，响应体业务码 500，沿用 HTTP 200 和当前 Trace ID，客户端仅展示固定错误文案。请求体读取失败、无效密文或认证失败按现有 400 业务码响应。HttpSuccessResultNotAes 继续提供显式明文响应。
- **请求体** 读取原请求体后关闭其 Reader。解密后同步重建 Body、GetBody、ContentLength 和 Content-Length，清除原 TransferEncoding。普通请求按已有规则透传，非空请求体带有 business_code 时要求提供 aes_data。已提供但为空、null 或非字符串的 aes_data 返回解密错误。
- **定向验证** 使用临时 overlay 执行 go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestAES13' -count=1 -v ./common-pkg/aesGCM ./common-pkg/result ./common-pkg/app，五组通过。覆盖 16/24/32 字节密钥、随机 nonce、空明文、带 padding 密文、截断和篡改、非法密钥及短密文。真实 go-zero 路由验证两个不同密钥 Server 与关闭加密的 Server 并发处理请求，解密后的请求正文、响应及 Trace ID 正常。注入序列化和随机源失败验证错误响应，检查请求读取失败、请求体长度及 GetBody。
- **竞态与构建** 同一组临时测试加 -race 通过。合并解密封装判断后，再执行带 -race 的 TestAES13RequestHandling，通过。go test -mod=readonly -buildvcs=false ./common-pkg/aesGCM 运行原有 AES 用例，通过。go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...、go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1 及 Git 差异检查通过。
- **验证限制** 使用内存 HTTP 请求和实际 go-zero 路由，前端加解密及部署配置待联调。原有外部服务测试仅做编译。新增、修改的配置字段和对外接口已补充中文注释。
- **测试清理** 登记验证后删除本次临时用例、overlay、日志、下载文件及 Go 缓存。核对临时目录与三份 overlay 目标文件已移除，项目原有五个测试文件保留。

API 装配时，将已校验的 AES 选项与现有认证等选项一起传入。

```go
aesOption, err := app.WithAESGCM(config.AESGCM)
if err != nil {
    return err
}

middleware := app.NewServerMiddleware(server, aesOption, authOption)
middleware.ApiUseMiddleware()
```

### 2026-11-19 / DEV-20261119-006 / 第 13 项改为配置开关控制

- **用户要求** 加密器通过配置开关控制，移除请求上下文传递。
- **配置逻辑** aesGCM.Configure 校验并应用当前进程共享的 Enabled 和 Key，默认关闭。校验失败时返回错误并保留原配置。开关与已校验的加密器作为同一份只读状态原子发布。请求解密与成功响应加密直接读取配置，因此业务传入 Background 或 TODO 上下文时仍按开关执行加密。
- **接入** ConfigAppServer.AESGCM 和 app.WithAESGCM 的调用方式继续适用。WithAESGCM 先校验配置，在 ApiUseMiddleware 装配 API 时应用。同一进程的 API 共用这份开关和密钥。独立构造 NewApiRequestDecryptMiddleware 时也会应用传入的进程级配置。
- **接口调整** 删除 ContextWithCipher 和 CipherFromContext，仓库 Go 源码中的引用已清理。纯 Encrypt、Decrypt 以及独立 Cipher 继续供直接加解密使用。序列化及加密失败使用第 13 项的统一错误响应。
- **定向验证** 使用临时 overlay 执行 go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestAES(ConfigSwitch|ConfigConcurrentAccess|ResponseWithoutRequestContext|ConfigFailureResponses)$' -count=1 -v ./common-pkg/aesGCM ./common-pkg/result，通过。覆盖开关启停、非法密钥、原子配置读取、Background 和 TODO 上下文返回加密响应、序列化和随机源失败。
- **装配与并发** 带 -race 运行上述用例及 TestAESConfigServerAssembly，通过实际 go-zero 路由验证加密请求、业务丢失上下文后返回加密响应，以及关闭开关后返回普通 JSON。未使用的 WithAESGCM 选项仅校验配置。配置并发发布与读取通过竞态检测。
- **构建与限制** go test -mod=readonly -buildvcs=false ./common-pkg/aesGCM、go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...、go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1 和 Git 差异检查通过。定向验证使用内存 HTTP 与实际 go-zero 路由，前端和部署配置待联调。
- **测试清理** 登记验证后已删除本次临时用例、overlay、日志、下载文件及 Go 缓存。核对临时目录和三份 overlay 目标文件已移除，原有五个测试文件保留。

### 2026-11-19 / DEV-20261119-007 / 第 14 项 RPC 认证和错误传递

- **用户确认** 统一 RPC 白名单和认证回调，采用标准 gRPC 状态码，恢复 API 业务错误，显式复制调用 metadata。
- **服务端认证** NewRpcAuthMiddleware 接收 CheckRPCRequestFunc 并返回中间件和 error，缺少回调时初始化失败。白名单支持精确方法名和末尾 * 前缀，其他方法均执行回调，包括完全缺少 metadata 的请求。回调读取 metadata 或传输层身份并校验调用方。认证失败返回 Unauthenticated，权限不足返回 PermissionDenied，取消、超时和服务不可用使用对应的标准状态。具体服务凭证策略由 RPC 项目提供。
- **错误传递** result/xcode.ToGRPCStatus 将业务错误映射为有效的 gRPC 状态，通过标准 ErrorInfo 携带业务码、公开文案和特殊错误标记。Code.GRPCStatus、SpecialCodeError.GRPCStatus、NewRpc 及 logs/xcode.FromError 共用该规则。普通内部错误、5xx 错误和 panic 使用固定公开文案。诊断信息由服务端日志记录。
- **客户端** 成功调用继续返回 nil。失败时通过 FromGRPCError 恢复业务码、公开文案和特殊通知错误，保留底层错误链。原生取消和超时支持 errors.Is 检查 context.Canceled 和 context.DeadlineExceeded，API 业务码分别为 499 和 504。未认证、权限不足及服务不可用分别映射为 401、403 和 503。原生 gRPC 其他状态按统一映射处理。
- **metadata** 客户端在注入 rpc_name 和链路字段前显式复制 metadata，服务端为入站和出站上下文分别提供副本。当前 gRPC 库的读取函数也会返回副本，测试核对原始 metadata、并发调用各自的名称及相同 Trace ID，未将复制前行为描述为已复现的数据竞争。
- **定向验证** 临时 overlay 执行 go test -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestRPC14' -count=1 ./common-pkg/result/xcode ./common-pkg/appMiddleware ./common-pkg/rpc/interceptors，通过。覆盖业务码往返、特殊通知多级传递、nil 及 typed nil 错误、包装错误的公开文案、原生状态、缺少凭证、伪造 business_code、白名单、权限拒绝和 panic。首轮捕获业务取消文案被标准文案覆盖，增加错误详情的公开文案后通过。
- **真实协议验证** 通过 bufconn 建立实际 gRPC 服务和客户端，验证 API 错误响应、认证拒绝、多级 RPC Trace ID、业务错误与通知往返、调用过程中取消及超时。该组和全部定向用例以 -race 通过。
- **构建与限制** go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...、go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1 及 Git 差异检查通过。原有测试做全量编译。验证范围为一元 RPC，流式 RPC、部署凭证策略及生产联调待对应服务接入时确定。
- **补充验证** 对直接构造零值中间件的情况增加防护检查，受保护调用返回固定 Internal 文案。带 -race 运行 TestRPC14Authentication，通过。
- **测试清理** 登记验证后删除本次临时用例、overlay、日志、下载文件和 Go 缓存。核对临时目录和四份 overlay 目标文件均已移除，原有五个测试文件保留。

go-zero RPC Server 在启动前接入认证拦截器。checkCaller 是服务实现的认证回调，publicMethods 返回公开方法列表，nil 表示全部方法都经过认证。

```go
auth, err := appMiddleware.NewRpcAuthMiddleware(checkCaller)
if err != nil {
    return err
}
server.AddOptions(auth.ServerOption(publicMethods))
```

业务继续返回 xcode.New(code, message) 或特殊通知错误。使用基础包客户端拦截器后，API 可以从返回错误读取原业务码，公开文案由统一 HTTP 响应处理。

### 2026-11-19 / DEV-20261119-008 / 第 14 项 RPC 必传链路信息

- **用户要求** RPC metadata 为空或缺少 Trace ID 时直接返回错误。
- **入口规则** 在白名单、认证回调和业务处理前检查传入 metadata。不存在或为空时返回 InvalidArgument。有效的 traceparent 或兼容 trace 字段可提供 Trace ID，缺失、格式错误、重复值以及两者冲突都返回 InvalidArgument，基础包客户端映射为 API 业务码 400。白名单方法执行同一入口规则。
- **链路处理** 入站校验以实际传入的 metadata 为准，已有服务端 Span 的情况也执行该校验。保留调用方的 Trace ID，仅 trace 字段存在且需要建立 SpanContext 时生成本次接收的 Span ID。接收的 W3C 上下文和兼容 trace 字段继续用于后续 RPC 传递。
- **定向验证** 使用临时 overlay 执行 go test -race -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestRPCTraceGate' -count=1 ./common-pkg/appMiddleware ./common-pkg/rpc/interceptors，通过。覆盖 metadata 缺失及空值、身份字段存在但链路缺失、只有服务端 Span、空和非法 Trace ID、全零 ID、非法 traceparent、重复及冲突链路字段、白名单入口校验和原始 metadata 副本。
- **协议验证** bufconn 上的实际 gRPC 服务验证公开方法也拒绝缺少链路的调用，单独 trace 或 traceparent 能正常调用，基础包客户端注入的链路与服务端收到的 Trace ID 一致。
- **构建与限制** go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...、go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1 和 Git 差异检查通过。本轮覆盖一元 RPC，生产入口和服务凭证配置待联调。
- **测试清理** 登记验证后已删除本次临时用例、overlay、日志、下载文件和 Go 缓存。核对临时目录和两份 overlay 目标文件已移除，原有五个测试文件保留。

### 2026-11-19 / DEV-20261119-009 / 提取 RPC 日志字段名单

- **用户要求** RPC 日志字段名单定义为包级变量，请求处理时复用。
- **改动** 将 contextMetadataInLog 中的字段名单提取为私有包级变量 rpcLogKeys，并补充中文注释。函数使用它创建本次请求的日志字段。
- **验证** 执行 gofmt -w appMiddleware/rpcAuthMiddleware.go 和平台及公共包的 git diff --check，通过。使用 Python 静态对照 HEAD 中原名单与提取后的名单，字段内容和顺序一致，函数内的局部名单声明已移除。
- **验证范围与清理** 本次为字段名单提取，采用语法格式和静态核对，未执行构建或运行测试，未新增测试用例和临时产物。

### 2026-11-19 / DEV-20261119-010 / 日志字段统一到 headInfo

- **用户要求** 日志字段名单放到 headInfo，相关位置统一引用。
- **改动** 新增 headInfo.LogKeys，在包初始化时创建共用名单。RPC 的 metadata 日志注入迁移为 headInfo.ContextMetadataInLog，RPC 中间件改为调用它。HTTP 的 ContextHeadInLog 按同一名单选择本次请求的字段。
- **字段与格式** 静态对照原字段名单，名称及顺序一致。HTTP 字段使用字符串，RPC metadata 字段使用字符串列表。请求字段值由各请求独立构建，已有业务自定义日志上下文字段继续参与日志输出。
- **验证** 临时 overlay 执行 go test -race -mod=readonly -buildvcs=false -overlay <临时配置> -run '^Test(SharedHeadLogKeys|HeadLogKeysSingleSource|HeadLogValuesPerRequest)$' -count=1 ./common-pkg/headInfo，三组通过。覆盖 HTTP 和 RPC 共用名单、字段格式、凭证及未知字段排除、业务自定义字段、原始 metadata 和并发请求的值隔离。测试中临时替换名单，确认两个入口均读取同一来源，测试结束后恢复。
- **构建与范围** go build -mod=readonly -buildvcs=false ./common-pkg/headInfo ./common-pkg/appMiddleware、gofmt 和 Git 差异检查通过。本轮针对相关包构建和日志行为验证。
- **测试清理** 登记验证后已删除临时用例、overlay、日志、下载文件及 Go 缓存。核对临时目录和 overlay 目标文件已移除，原有五个测试文件保留。

### 2026-11-19 / DEV-20261119-011 / 第 15 项 RabbitMQ 重试及 TG 通知

- **用户确认** 进入第 15 项，增加重试队列，并在重试耗尽后发送 TG 通知。第 14 项记为已确认，本项完成后等待确认。
- **配置与接口** 新增独立 `rabbitmq.Config`、`TelegramConfig` 和 `ConfigAppServer.RabbitMQ`。新接口为 `NewRabbitMQ(ctx, config)`、`Consume(Handler)`、`Publish(ctx, amqp.Publishing)`、`PublishBatch`、`Close`；不保留旧接口。配置字段、公开接口和关键逻辑均补充中文注释。完整默认值、消息流程和调用示例见 [RabbitMQ 接入说明](amqp/rabbitMq/README.md)。
- **连接与发布** 初始化声明业务队列并绑定具名交换机，失败立即返回并回收连接。连接、发布通道或订阅失效后重建拓扑和订阅，关闭后停止重连。所有发布启用 mandatory 和 broker 确认，区分未路由、broker 拒绝、连接未就绪和结果不确定。发布超时主动关闭当前连接以中断网络写入，取消原因保留在错误链中。默认持久化消息，并为缺少 ID 的发布生成 ID。
- **重试与归档** 默认首次处理之外重试 3 次，间隔 30s。每次失败先转入独立重试队列，broker 确认后再单条 ACK 原消息；到期定向重投原业务队列，避免对其他交换机订阅者重复广播。重试耗尽后通过专用 fanout 交换机投递失败归档和通知任务，确认后再 ACK 当前消息。转存失败不 ACK 原消息，手动确认模式下重新入队；内部头损坏直接归档，处理器 panic 按失败处理。所有 ACK/NACK 均为 `multiple=false`，不批量确认其他失败消息。
- **TG 通知** 默认关闭，启用时要求 BotToken 和 ChatID。业务注册消费者后启动独立通知消费者，发送服务名、队列、消息 ID、处理次数和错误摘要，不发送正文。成功后单条 ACK 通知任务；失败按独立间隔重试并遵守服务端 retry_after。TG 失败不占用业务消费者，失败归档继续保留；关闭 TG 时通知任务保存在队列，启用后处理积压。
- **运行语义** 默认手动确认。开启 AutoAck 时原消息已自动确认，转存失败无法恢复。断线、确认丢失和 TG 响应丢失可能导致重复，业务按正文中的稳定业务标识幂等处理，生产重发沿用 MessageId。内部转存强制持久化并保留正文、消息 ID 和常用属性，不沿用原 TTL 及生产者 UserId。处理器须响应上下文取消，超过关闭时限会断开连接并返回超时，无法强制终止用户回调。
- **定向验证** 临时 overlay 下 10 组 TestRabbit15 用例通过，并启用 -race。覆盖 YAML 与 Go 默认值、显式零值、协议数值范围、拓扑绑定和发布确认、24 个并发发布、mandatory 返回及 NACK、重试计数和延迟、转存确认先于源 ACK、转存失败不 ACK、单条确认、AutoAck 不重复确认、panic、消费者取消与连接断开后的恢复、初始化失败回收、并发重复关闭和不响应取消的回调超时。补充 net.Pipe 验证阻塞写入 deadline、初始化取消回收、等待发布锁的取消不影响其他发布，以及阻塞发布取消时保留 DeadlineExceeded。
- **通知验证** 使用模拟 HTTP Transport 验证 TG 500 后重试成功、429 的 retry_after、凭证不进入普通错误文本、UTF-16 文本长度限制、失败任务不 ACK，以及 TG 请求阻塞时业务消息继续处理。未向实际 TG 发送消息。
- **执行命令** 以下使用独立临时 GOCACHE 和 `GOPROXY=off`，依赖来自现有本地缓存，未新增依赖或下载文件。

```text
go test -mod=readonly -buildvcs=false ./common-pkg/amqp/rabbitMq -run '^$'
go test -race -mod=readonly -buildvcs=false -overlay <临时配置> ./common-pkg/amqp/rabbitMq -run '^TestRabbit15' -count=1 -v -timeout=30s
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/amqp/rabbitMq common-pkg/app/appServer.go
git -C common-pkg diff --check
git diff --check
```

- **结果与限制** 上述验证通过。broker 测试使用驱动接口的内存模拟端，网络写超时使用 net.Pipe；环境未安装 RabbitMQ，未验证实际 broker 的持久化、节点重启和生产积压。TG 使用模拟响应，真实 BotToken、ChatID 和网络环境待服务接入时配置联调。项目原有测试本轮只做全量编译。
- **测试清理** 清理前已登记上述测试命令、结果和限制；随后删除本次临时测试、overlay 和独立构建缓存。核对临时目录及仓库内的 overlay 目标文件均不存在，原有五个测试文件全部保留。

### 2026-11-19 / DEV-20261119-012 / 合并 Request ID 与 Trace ID

- **用户确认** Request ID 与 Trace ID 共用一个值，并把生成规则明确备注。当前任务实现统一 ID；RabbitMQ 处理超时、业务消息接口及其余优化仍按先讨论后开发的流程单独确认。
- **生成与校验** 新增 `headInfo.NewTraceID` 和 `ExtractTrace`。使用 crypto/rand 读取 16 个随机字节，编码为非全零的 32 位小写十六进制字符串，无时间戳、业务前缀或连字符。全零重新生成，随机源失败返回错误。入站拒绝空值、重复值、大写、非法长度、空格及字段冲突。代码中文注释及 [统一请求 ID 规则](headInfo/TRACE_ID.md) 记录生成算法、前端 Web Crypto 示例、网关职责及各层接入方式。
- **入口规则** HTTP 优先沿用合法输入，各 ID 与 traceparent 同时存在时必须相等，全部缺失时兜底生成；非法输入返回真实 HTTP 400，不进入业务。RPC 缺少 ID 仍直接返回 InvalidArgument。trace、request_id、x-request-id 和正文 trace 使用同一个值，Span ID 继续区分操作。HTTP 返回 X-Request-ID，CORS 允许前端读取该响应头。
- **原生链路顺序** 新增 `app.NewServer`、`app.WithRequestTraceRouter`，在 go-zero 路由外层校验并初始化 ID，再创建原生 Trace Span 和日志，避免只改业务上下文造成导出链路不一致。仅有 ID 的新链路按服务的 Telemetry.Sampler 决定采样，有 W3C 上游上下文时保留其采样标记。新增 `RpcAuthMiddleware.ServerOption`，通过 server.AddOptions(auth.ServerOption(...)) 将入站校验放在原生 gRPC 链路拦截器前；同一 Server 只设置一次主 UnaryInterceptor。
- **RPC 与消息传递** HTTP Head、RPC 入站/出站 metadata 和标准 W3C 头同步同一 ID，已有活动 Span 保留，复制后修改 metadata。RabbitMQ 自动复制及注入链路头，消费恢复上下文、创建消费 Span，重试、归档与 TG 告警沿用该 ID；消费者不继承生产请求的超时或取消。独立发布或外部无链路消息可生成新链路，非法消息链路不执行业务并归档。具体消息的 MessageId 保持独立。
- **接口影响** SetTrace 现在返回 (context.Context, error)，会校验 ID 并拒绝不同活动链路。API 使用 app.NewServer 或包装后的 Router；RPC 认证使用 ServerOption 在原生链路前接入。未提供旧接口兼容层。仓库内 SetTrace 无已有调用方，调用示例及 API/RPC 子项目记录已更新。
- **验证** 临时 overlay 下 10 组 TestUnified 用例以 -race 通过。覆盖 64 个并发 ID 生成、全零重生成、随机源失败、严格格式/重复/冲突校验、metadata 副本及活动 Span 保留、真实 go-zero HTTP 路由响应与错误入口、导出的原生 HTTP Span、API 到两级实际 gRPC 服务、无 traceparent 的原始 RPC 调用在原生 Span 前归一化、缺少 RPC ID 拒绝，以及 0/1 采样与上游采样标记保留。首轮临时用例因直接比较包含 TraceState 的结构而编译失败，改为比较 Trace ID/Span ID 后通过。
- **消息验证** 模拟 broker 接口与 TG HTTP Transport 验证发布、消费、延迟重投、最终归档、通知和消费内 RPC 的 ID 一致，MessageId 不变、源头不被修改、Span ID 区分操作、取消生产请求后消费者继续工作，以及无链路新任务生成和非法链路归档。TG 未向真实服务发送。提取文档中的 Web Crypto 示例，用 Node 执行 1000 次格式、非零及重复值检查，通过。
- **执行命令** 以下均使用本次独立临时 GOCACHE、GOPROXY=off 与现有本地依赖缓存；未增加或下载依赖。

```text
go test -race -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestUnified' -count=1 -v ./common-pkg/headInfo ./common-pkg/app ./common-pkg/amqp/rabbitMq -timeout=60s
node <临时目录>/frontend-id.cjs
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l <本次修改的 Go 文件>
git -C common-pkg diff --check
git diff --check
```

- **结果与限制** 定向验证、竞态检测、全量构建和项目原有测试编译通过。gRPC 使用 bufconn 上的真实协议，MQ 使用内存接口模拟端；真实网关配置、真实 RabbitMQ/TG 和生产部署联调待接入。前端示例使用 Node Web Crypto 验证，未接入实际前端应用。
- **测试清理** 清理前已登记上述命令、结果和限制；随后删除本次三份临时 Go 用例、Web Crypto 验证脚本、overlay 和独立构建缓存。核对任务临时目录及三个仓库内 overlay 目标文件均不存在，项目原有五个测试文件全部保留。

### 2026-11-19 / DEV-20261119-013 / 删除旧请求 ID 字段及接入入口

- **用户要求** 旧实现直接删除，不保留兼容。清理范围为本次统一请求 ID 留下的重复字段、别名函数及旧接入路径。
- **删除项** 删除 Head.RequestID、consts.RequestId、consts.Trace、GetTrance，以及 RPC 认证中间件的公开 Handle。RPC 认证只通过 ServerOption 接入；应用中间件和 ApiHeaderMiddleware 不再重复初始化 ID，入口统一由 app.NewServer 或包装后的 Router 负责。
- **传输与日志** HTTP 只以 X-Request-ID 对外接收和返回统一 Trace ID，正文仍使用 trace。RPC 和 RabbitMQ 只注入 W3C traceparent/tracestate，不再注入 request_id、trace 或内部 x-request-id 别名。RPC 缺少 traceparent 时直接拒绝，其他 ID 字段和已有 Span 均不能替代。HTTP 业务 Head 只保存 Trace，移除日志名单中的重复 ID 字段，日志 Trace ID 由 go-zero 从当前 Span 输出。
- **实现** 标准链路提取 ExtractTrace 与 HTTP 入口提取 ExtractRequestTrace 分开；SetTrace 直接校验并绑定 ID。EnsureTrace 只读取出站 W3C 链路，保留 metadata 副本隔离，清除不属于当前 Span 的旧 tracestate 后重新注入。新请求的 16 字节安全随机数、32 位小写十六进制、非全零生成规则保持一致。
- **文档** 更新统一 ID 规范和全局 AGENTS.md，移除规范中的旧字段及旧接入示例。公共包原 RPC 示例改为 ServerOption，全局及 API/RPC 子项目登记当前协议，历史验证记录继续作为开发过程留档。
- **定向验证** 临时 overlay 执行 6 组 TestTraceClean 用例，以 -race 通过。覆盖删除字段和公开入口、拒绝旧别名作为 RPC/HTTP 链路替代、HTTP 格式及冲突校验、metadata 仅注入 W3C、业务字段保留、日志名单去重、HTTP 原生 Span 和响应 ID 一致、两级真实 gRPC 调用保持同一 Trace ID 及不同 Span ID、实际 RPC 拒绝旧字段和重复/非法 traceparent，以及 RabbitMQ 发布、消费、重试、归档与消费内 RPC 的标准传播。消息 ID 和单条 ACK 行为保持，取消生产上下文不影响消费。
- **静态核查** Python 检查 Go 源码中无 GetTrance、旧常量引用、Head.RequestID 或 RPC 公开 Handle；链路生成及传输代码不包含旧别名写入，业务中间件的重复初始化已删除。gofmt 和 Git 差异检查通过。
- **执行命令** 以下使用独立临时 GOCACHE、GOPROXY=off 和现有本地依赖缓存。

```text
go test -race -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestTraceClean' -count=1 -v ./common-pkg/app ./common-pkg/amqp/rabbitMq -timeout=45s
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/headInfo common-pkg/app common-pkg/appMiddleware common-pkg/rpc/interceptors common-pkg/amqp/rabbitMq common-pkg/consts/system.go
git -C common-pkg diff --check
git diff --check
```

- **结果与限制** 定向测试、竞态检测、全量构建及原有测试编译通过。gRPC 使用 bufconn 实际协议，MQ 为接口模拟端；实际网关、RabbitMQ/TG 及生产服务尚未联调。本次不新增依赖，不修改其他业务模块的接口。
- **测试清理** 清理前已登记上述命令、结果和限制，随后删除两份临时用例、overlay 与独立构建缓存。核对任务临时目录和两个仓库内 overlay 目标文件均不存在，原有五个测试文件全部保留。

### 2026-11-19 / DEV-20261119-014 / RabbitMQ 单条业务处理超时

- **用户确认** 为单条业务处理增加独立超时，默认 30s；超时后复用现有重试及耗尽后的 TG 通知。回调退出后再转存，服务关闭取消不计为业务失败。
- **配置与接口** rabbitmq.Config 新增 HandlerTimeout，零值默认 30s，负值拒绝，支持按业务队列配置。计时覆盖每次业务回调，不包含排队、重试等待、发布确认或 TG HTTP。Handler 签名保持不变，传入的 ctx 带独立截止时间，回调结束后取消并回收。配置及关键行为增加中文注释，接入说明补充 YAML、时限范围和取消边界。
- **处理与转存** 在原 worker 中同步调用业务，仅派生限时子上下文；超时即使回调返回 nil 仍按失败处理。超时错误保留 context.DeadlineExceeded，消费 Span 记录错误，日志包含队列、消息 ID、时限、耗时和同一个 Trace ID。回调未退出时也会记录超时，但不提前转存或创建新的业务 goroutine。正常完成时取消定时通知，避免清理调度滞后误报成功消息。
- **重试及关闭** 转存使用原消费生命周期上下文，并由 ConfirmTimeout 独立限时，避免回调的过期上下文阻断重试或归档。等待 broker 确认后才单条 ACK 原消息，转存失败不 ACK。处理前及回调返回后均检查生命周期取消；退出或连接失效优先于超时及 nil 结果，不确认取消后的任务、不增加业务重试次数，关闭连接后由 broker 恢复投递。默认手动确认提供该语义，AutoAck 仍为尽力转存，无法恢复已自动确认的源消息。
- **定向验证** 临时 overlay 下 11 组 TestHandlerTimeout 用例以 -race 通过。覆盖 Go/YAML 缺省与显式 0 的 30s 默认、自定义时限、负值拒绝，正常处理单条确认和回调上下文回收，panic 和下游短超时保留原行为，处理超时后的独立转存、确认前不 ACK、转存 NACK/确认超时不 ACK，超时后返回 nil 仍失败，下一条消息获得新截止时间，DeadlineExceeded 错误链、Trace ID、Span 状态和超时日志不含正文。
- **生命周期及通知验证** 单 worker 的不响应取消回调超时后仍阻止后续业务启动，退出前不转存，退出后恢复消费；日志在等待退出期间已产生且每次处理只记录一次。验证提前父级 deadline、已取消上下文、超时后再关闭、正常关闭及关闭超时均不误 ACK 或推进业务重试。四次处理耗尽默认三次重试后归档，模拟 TG 发送一次并带超时摘要，AutoAck 不重复确认。bufconn 上实际 gRPC 服务验证处理器传入的截止时间取消下游请求，之后仍能成功转存重试消息。
- **执行命令** 以下使用本次独立临时 GOCACHE、GOPROXY=off 和已有本地依赖缓存，无新增依赖或下载文件。

```text
go test -race -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestHandlerTimeout' -count=1 -v ./common-pkg/amqp/rabbitMq -timeout=45s
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/amqp/rabbitMq/config.go common-pkg/amqp/rabbitMq/consume.go common-pkg/amqp/rabbitMq/conn.go
git -C common-pkg diff --check
git diff --check
```

- **结果与限制** 定向验证、竞态检测、全量构建及原有测试编译通过。RabbitMQ 使用驱动接口的内存模拟端，TG 使用模拟 HTTP Transport，未发送实际通知；下游 RPC 使用真实 gRPC 协议的内存连接。环境未安装 RabbitMQ，真实 broker 的恢复、确认超时及部署参数待联调。Go 无法强制终止忽略 ctx 的回调，超时后返回仍可能造成业务重复，调用方须按稳定业务标识幂等处理。逐条处理错误通过日志/Span/队列观察，Consume 的返回值只表示订阅初始化。
- **测试清理** 清理前已登记上述命令、结果和限制，随后删除临时 Go 用例、overlay 与独立构建缓存。核对任务临时目录和仓库内 overlay 目标文件均不存在，项目原有五个测试文件全部保留。

### 2026-11-19 / DEV-20261119-015 / RabbitMQ 业务消息接口及使用说明

- **用户确认** 简化业务接入，新增 Message、JSON 编解码、发布 ID 返回值和批量部分成功结果；旧接口直接替换。另要求独立建立带使用范例的说明文件，并确认业务自增 ID 可以用作消息 ID。
- **消息接口** 发布和消费统一使用 Message，提供 ID、正文、普通嵌套消息头、内容属性、RetryCount、Redelivered 及投递来源。Handler 改为 func(context.Context, Message) error，不暴露协议 ACK/NACK。消费前复制正文、嵌套表、数组和二进制头，业务修改 ID、正文、持久化标记或消息头不会污染重试原文。每次回调继续受 HandlerTimeout 限制，关闭及转存确认语义保留。
- **JSON 与发布** 新增 Message.DecodeJSON、NewJSONMessage、PublishJSON、WithMessageID、WithHeaders 和 WithCorrelationID。Publish(ctx, Message) 返回实际 ID 和 error，分配 ID 后即使发布失败也返回该 ID；JSON 编码失败不会发送，编解码错误保留标准错误链并使用不含正文的公开错误文本。NewJSONMessage 可在连接 broker 前生成稳定 ID 及 JSON 正文，供保存和批量发送。保留必要的高级内容属性，业务不必导入驱动。
- **ID 规则** 默认生成 36 位 UUIDv7，不是自增数字，同一毫秒不保证递增。允许用 strconv.FormatInt 将业务自增 ID 转为字符串后指定，也支持业务类型/ID/事件等组合。已指定 ID 不再生成 UUID，重发及包内消费重试沿用原 ID。消息 ID 与 Trace ID 独立，当前封装不提供自动消息去重，业务负责幂等。ID 和内容短字符串按 255 字节校验，避免驱动静默截断；非法元数据在发送前拒绝。
- **批量结果** PublishBatch(ctx, []Message) 返回 BatchResult 和 error。MessageIDs 对应输入顺序，已尝试项返回实际 ID，未尝试项保留输入 ID；ConfirmedCount 为已确认且已路由的连续前缀；FailedIndex 从 0 开始，首次失败即停止，成功或批次初始化失败为 -1。错误链支持 ErrPublishUnknown，失败那条可能已接收，其后未尝试。传入切片不修改，由调用方保留 ID 并只重发未确认部分。
- **使用说明** 新建 [USAGE.md](amqp/rabbitMq/USAGE.md)，含初始化/消费/退出的完整 main、业务自增 ID、字节正文、离线 JSON 消息准备、稳定 ID 重发、批量部分成功恢复、错误分类和幂等职责。README 作为配置与运行入口链接该文件，删除旧 Publishing 发布及 []byte 回调范例，统一链路规范改用 Message.ID 字段名称。
- **定向验证** 临时 overlay 执行 13 组 TestMessage 用例，-race 通过。覆盖结构体 JSON 往返和大 int64 精度、中文、空目标/语法/类型错误链、编码失败不发送且返回已分配 ID，显式业务数字/组合/大整数 ID 原样发送、自动 ID 与 24 个并发发布、常用属性与默认值、嵌套头和正文隔离、循环头和不支持类型拒绝、过长字段/非法 TTL 校验，关闭/未就绪/未路由/NACK/结果不确定时的 ID 保留。
- **流程验证** 覆盖批量部分成功、失败停止、已分配 ID 的尾部恢复及输入切片不变；消费元数据、JSON 解码、业务修改不影响原文及 Trace ID，单条确认时序、解码失败归档、处理超时后独立转存、关闭取消不确认。检查旧的驱动 Publishing 发布签名和 []byte Handler 已删除。4 段完整 Go 使用范例均抽取到临时文件编译通过，未执行会连接真实 broker 的示例。
- **执行命令** 以下使用本次独立临时 GOCACHE、GOPROXY=off 和现有本地依赖缓存，没有新增依赖或下载文件。

```text
go test -race -mod=readonly -buildvcs=false -overlay <临时配置> -run '^TestMessage' -count=1 -v ./common-pkg/amqp/rabbitMq -timeout=45s
go build -mod=readonly -buildvcs=false -o <临时产物1> <临时目录>/usage_1.go
go build -mod=readonly -buildvcs=false -o <临时产物2> <临时目录>/usage_2.go
go build -mod=readonly -buildvcs=false -o <临时产物3> <临时目录>/usage_3.go
go build -mod=readonly -buildvcs=false -o <临时产物4> <临时目录>/usage_4.go
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/amqp/rabbitMq/message.go common-pkg/amqp/rabbitMq/provider.go common-pkg/amqp/rabbitMq/conn.go common-pkg/amqp/rabbitMq/consume.go
git -C common-pkg diff --check
git diff --check
```

- **结果与限制** 定向测试、竞态检测、使用范例编译、全量构建和原有测试编译通过。broker 使用驱动接口模拟端，无实际 RabbitMQ/TG 联调或消息发送。JSON 解码错误仍按现有错误策略重试，不提前实现永久错误分类；消息去重、业务事务与失败任务持久化由业务接入。使用范例的实际运行需提供真实连接及服务配置。
- **测试清理** 清理前已登记上述命令、结果和限制；随后删除临时测试、overlay、4 份抽取范例源码、全部范例编译产物和独立构建缓存。核对当前文档与已编译范例一致、任务临时目录和仓库内 overlay 目标文件均不存在，原有五个测试文件全部保留。


### 2026-11-19 / DEV-20261119-016 / RabbitMQ 消费错误分类和使用范例

- **用户确认** 开发消费错误分类，并单独建立带使用范例的说明文件。本次替换此前“所有 JSON 解码错误均重试”的行为，普通业务错误仍默认重试。
- **公开接口** 新增 Permanent(err) 和 Retryable(err)，nil 输入均返回 nil，保留错误文本和错误链，支持 errors.Is/As 及 %w。先查找显式标记，外层覆盖内层；多分支按标准 errors.As 深度优先、从左到右找到第一个标记。无标记时，标准 JSON SyntaxError、UnmarshalTypeError、InvalidUnmarshalError 直接归档，其他自定义解码器错误与普通业务错误默认重试。Retryable 可覆盖默认 JSON 分类或内层 Permanent，仍受 MaxRetries 限制，不根据业务文案或 HTTP/gRPC 状态码猜测策略。
- **消费与确认** 永久错误不进入业务重试，直接转存失败交换机；确认转存后才单条 ACK，NACK 或确认超时不 ACK 源消息。服务生命周期取消优先，不重试或确认；HandlerTimeout 仍覆盖超时后返回的 nil 或永久标记，按超时重试。归档补齐缺失消息 ID，原有业务 ID、正文及合法 Trace ID 保留。
- **终态通知** 新增内部 x-base-failure-reason，分别保存 permanent、retries_exhausted、invalid_metadata；生产者的同名头不控制策略，业务重试移除旧终态原因。TG 按原因显示不可重试错误、重试耗尽或元数据错误，永久错误归档后即可通知，普通错误仅耗尽后通知。TG 仍默认关闭、独立重试并在发送成功后 ACK 通知任务。处理次数为本包 RetryCount+1，不统计 broker 重投的额外执行；元数据错误注明本次未执行，未知原因使用通用文案。
- **文档与注释** 新建 [消费错误处理说明](amqp/rabbitMq/ERROR_HANDLING.md)，包含完整领域处理器与注册、显式重试、包装和 nil 的范例，说明优先级、超时、归档和 TG 行为。同步 README、USAGE 的入口与字段校验例子，以及 Handler、DecodeJSON、TelegramConfig 和关键分支的中文注释。平台与 API/RPC 开发记录同步接入约定。
- **定向验证** 临时 overlay 的 10 组 TestProcessingPolicy 用例及竞态检测通过。覆盖 nil、errors.Is/As、%w、嵌套与多分支策略，标准 JSON 和自定义解码错误；永久错误直接归档、等待发布确认才 ACK、原文/ID/链路不变；显式重试、连续失败耗尽和零次重试；NACK/确认超时不 ACK；元数据归档原因和缺失 ID；超时与关闭优先级；三类 TG 文案、HTTP 失败后独立重试及成功后确认；普通成功与 AutoAck 不重复协议确认。
- **执行命令** 在平台根目录执行，使用临时 GOCACHE、GOPROXY=off 和现有依赖缓存，无新增依赖。overlay、测试输出、抽取的范例和编译产物均位于 /private/tmp/new-platform-rabbit-policy.Agu6KF0T。文档通过临时 Python 抽取脚本分别编译 USAGE 的 4 段和 ERROR_HANDLING 的 3 段 Go 范例，未运行需要真实 broker 的程序。

```text
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-rabbit-policy.Agu6KF0T/overlay.json -run '^TestProcessingPolicy' -count=1 -v ./common-pkg/amqp/rabbitMq -timeout=45s
go build -mod=readonly -buildvcs=false -o <临时目录>/example_N <临时目录>/example_N.go  （N=1..7，逐一执行）
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/amqp/rabbitMq/{errors,message,consume,telegram,config,conn}.go
git -C common-pkg diff --check
git diff --check
```

- **结果与限制** 10 组定向测试及 -race、7 段范例编译、全量构建、原有测试编译和格式检查均通过，RabbitMQ 文档相对链接检查通过。broker 使用驱动接口模拟端、TG 使用模拟 HTTP Transport，未连接真实 RabbitMQ 或发送实际 TG 消息；真实服务、凭证、断线恢复及部署环境仍需联调。未执行原有集成测试的业务逻辑。错误分类不提供业务去重，业务须幂等。
- **测试清理** 清理前已登记实际命令、结果和限制；随后删除本次临时测试、overlay、测试输出、7 份范例源码和编译产物及独立构建缓存。已核对任务临时目录、早先合并缓存的临时目录及仓库内 overlay 目标文件均不存在，原有五个测试文件全部保留。API/RPC 本次仅更新文档，无新增测试或临时产物。


### 2026-11-19 / DEV-20261119-017 / RabbitMQ 失败归档与通知双份确认

- **用户确认** 删除失败广播投递代码，改为先定向保存失败队列，再定向创建通知任务，两份都确认后才单条 ACK 源消息；说明部分成功后重投可能重复。
- **实现** archive 使用默认交换机和 FailedQueue/NotifyQueue 的队列名分别发布，每次均沿用 mandatory 和发布确认，各受独立 ConfirmTimeout 限制。第一份未确认不发送第二份，任一步失败或结果不确定不 ACK；两份确认后再检查生命周期取消。错误包装区分保存失败消息和创建通知任务，保留 ErrUnroutable、ErrPublishRejected、ErrPublishUnknown 与 context 错误链。
- **拓扑清理** 删除内部 failedExchange 字段、生成及冲突校验、失败 fanout 声明及两条绑定；仍声明业务、重试、失败和通知四个队列，具名业务交换机和绑定按业务配置保留。无旧投递兼容路径，不自动删除 broker 历史资源，无公开接口或新配置字段。
- **语义与说明** 两份副本使用相同消息 ID、正文、失败原因和原有链路；通知关闭时仍可靠保存通知任务。发布确认丢失、第一份成功但第二份失败、源消息 ACK 失败后重投均可能重复归档或通知，业务与归档处理须幂等。外部未指定 ID 的源消息重投可能重新生成 ID，生产者应提供稳定 ID。同步 [运行说明](amqp/rabbitMq/README.md#失败归档与通知的投递确认)、[使用说明](amqp/rabbitMq/USAGE.md) 和 [错误处理说明](amqp/rabbitMq/ERROR_HANDLING.md)，补充中文关键注释。
- **定向验证** 临时 overlay 的 10 组 TestDualArchive 测试及 -race 通过。覆盖默认/具名业务拓扑与辅助队列声明失败、删除旧保留名称约束；先归档再通知及双确认前不 ACK；两个目标分别 basic.return、NACK、确认超时、写入错误；归档前/第一份/第二份取消、取消与第二份确认同时到达、通知投递时 Close；永久错误/耗尽/JSON/业务及重试元数据错误统一归档；原文、ID、Trace 和原因一致；成功及普通重试路径；部分成功后重复处理保持已有 ID；AutoAck 不重复协议确认及源消息 ACK 失败失效连接。
- **执行命令** 在平台根目录使用任务独立 GOCACHE=/private/tmp/new-platform-rabbit-dual.B6wkAXD9/cache 和 GOPROXY=off，复用本地依赖，无下载或新增依赖。测试源码、overlay、测试输出和缓存位于同一临时目录，没有在仓库创建测试文件。

```text
gofmt -w common-pkg/amqp/rabbitMq/config.go common-pkg/amqp/rabbitMq/conn.go common-pkg/amqp/rabbitMq/consume.go
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-rabbit-dual.B6wkAXD9/overlay.json -run '^TestDualArchive' -count=1 -v ./common-pkg/amqp/rabbitMq -timeout=45s
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/amqp/rabbitMq/{config,conn,consume}.go
git -C common-pkg diff --check
git diff --check
```

- **结果与限制** 10 组定向测试及竞态检测、全量构建、原有测试编译通过；格式、diff 和说明相对链接/新增锚点检查通过，原有五个测试保留。驱动接口模拟发布、确认与路由失败，无真实 RabbitMQ/TG 联调或外部消息发送；真实队列删除、重连和部署参数仍需联调。未运行原有集成测试业务逻辑。
- **测试清理** 清理前已登记全部实际命令、结果和限制，随后删除本次临时测试、overlay、测试输出和独立构建缓存。核对任务临时目录及仓库内 overlay 目标均不存在，原有五个测试全部保留。API/RPC 本次只更新文档，无新增测试或临时产物。


### 2026-11-19 / DEV-20261119-018 / Elasticsearch 7 独立配置及请求生命周期

- **用户确认** Elasticsearch 7 与 OpenSearch 分为两套独立实现，配置、初始化、错误、连接池和文档各自维护。本轮仅开发 es/esv7，完成后等用户确认再处理 OpenSearch；XXL-JOB 尚未开始。
- **独立配置** 新增 core.Config、TLSConfig 和 TransportConfig，全部字段补齐中文用途、默认值和单位。Addresses 必填，HTTP/HTTPS 可带路径前缀，认证使用独立字段；默认系统证书验证及 TLS 1.2，自定义 CAFile 追加信任，无法读取或无有效证书初始化失败，InsecureSkipVerify 默认 false。StartupTimeout 默认 5s、RequestTimeout 默认 10s，连接/TLS/响应头阶段默认 5s，连接池三个上限默认 100，空闲保留 90s；零值取默认、负值拒绝，校验每节点空闲数不超过连接总数。配置不修改调用方对象。
- **重试与认证** MaxRetries 改为 *int，nil 默认 3，显式 0 设置 SDK DisableRetry；针对当前 v7.17.10 SDK 直接 io.EOF 仍重试的特例，在关闭重试时包装该错误，保留 errors.Is(io.EOF)，避免绕过配置。SDK 自带认证替代重复手工 Basic Auth 头，单次只发送一条认证头；网络/502/503/504 重试沿用 SDK 规则，操作总预算不随重试重置。
- **初始化与关闭** NewEs(ctx, conf)/MustNewEs(ctx, conf) 接收启动上下文并执行 Info 探活，使用 StartupTimeout 与上游较早的截止时间，读取完成再取消，失败释放空闲连接。Close 可并发和重复调用，拒绝后续请求并释放连接池空闲连接，不强制取消在途 I/O。OperationContext 派生模型请求预算，调用方完成响应处理后再 cancel。移除旧签名、手工认证函数和未启用的旧 tracing transport/孤立指标定义，无兼容包装。
- **响应与错误** ReadResponse、CheckResponse 和 Get/Search 解析均接管响应体，在成功、404、错误状态、读取/关闭/解码失败及取消时释放；错误响应最多读取 4KiB，不无限 drain。独立 ErrNotFound/ErrInvalidResponse/ErrClosed 和 ResponseError 支持 errors.Is/As，HTTP 状态与最多 128 字节 error.type 用于诊断，不输出原始正文或 error.reason；JSON/网络/上下文错误保留底层错误链。Get 的 404/found=false 返回 ErrNotFound，成功直接解码到业务指针并设置 Schema ID，修复原 &interface{} 目标传递。
- **模型操作** 创建索引、create 写入、单查、删除、局部更新和搜索均继承 model.Ctx 并应用 RequestTimeout，不再用 Background 替换。增加 nil 模型/上下文/客户端、typed nil、空索引或 ID 校验。局部更新补齐 Elasticsearch 必需的 doc 包装。Search 删除已关闭的原始响应返回值，只返回 (int64, error) 并写入目标切片；使用原始 JSON 片段保留大 int64 精度，支持对象/数字 total 和空命中，track_total_hits=true 获取准确总数，统计成本在文档注明。
- **使用说明** 新建 [Elasticsearch 7 使用说明](es/esv7/README.md)，包含独立 YAML 默认值、完整初始化/FindOne/Search 示例、低层 SDK 请求预算与响应释放、错误语义和签名变更。原有 esv7/core/es_test.go 仅更新注释中的旧签名和指针配置，原测试继续保留；未修改 OpenSearch 或 XXL-JOB 源码。
- **定向验证** 最终 20 组 TestES7 用例及 -race 通过，其中 core 13 组、model 7 组。真实本机 HTTP/TLS 验证默认不信任证书拒绝、自定义 CA 信任及显式测试跳过、无效 CA、Basic Auth 单头、连接池覆盖、初始化上下文取消/超时和阶段超时；验证 SDK 重试 0/2 次以及关闭重试时直接 EOF 仅一次、开启时重试。
- **模型与清理验证** 覆盖所有六类操作的成功/上下文超时/取消、404 映射及 doc 更新封装、父级在途取消、正文读取时限、多个重试共享预算、Close 并发与后续请求拒绝；响应体正常/失败/解码/关闭错误链、4KiB 读取及 UTF-8 摘要边界、nil 响应/目标及必要字段校验、文档 ID 写回、大 int64、空搜索结果及不同 total 格式。README 的两段完整 Go 范例抽取编译通过，未运行需要真实服务的文档程序。
- **实际执行与排障** 初次定向命令被沙箱禁止本地监听端口拦住，获得仅本地 HTTP/TLS 测试所需权限后继续。首次本地运行 core 通过，model 临时服务未读取请求体导致测试服务清理等待并触发 60s 测试超时；修正临时服务读取请求体后通过。随后补充 EOF 边界与定向验证，最终完整 20 组 -race 回归通过。未把测试服务等待问题当作产品通过结果。
- **执行命令** 平台根目录执行，使用独立 GOCACHE=/private/tmp/new-platform-es7.hk7ZJfwS/cache、GOPROXY=off，定向命令额外设置 ES7_TEST_ARTIFACTS=/private/tmp/new-platform-es7.hk7ZJfwS/artifacts。overlay 测试、本地证书、测试输出、抽取范例与缓存均在此临时目录，未下载依赖。

```text
go build -mod=readonly -buildvcs=false ./common-pkg/es/esv7/...
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-es7.hk7ZJfwS/overlay.json -run '^TestES7' -count=1 -v ./common-pkg/es/esv7/... -timeout=60s
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-es7.hk7ZJfwS/overlay.json -run '^TestES7NoRetryEOF$' -count=1 -v ./common-pkg/es/esv7/core -timeout=30s
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-es7.hk7ZJfwS/overlay.json -run '^TestES7' -count=1 -v ./common-pkg/es/esv7/... -timeout=45s
go build -mod=readonly -buildvcs=false -o <临时目录>/example_1 <临时目录>/example_1.go
go build -mod=readonly -buildvcs=false -o <临时目录>/example_2 <临时目录>/example_2.go
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/es/esv7/core/*.go common-pkg/es/esv7/model/*.go
git -C common-pkg diff --check
git diff --check
git -C common-pkg diff --exit-code -- es/esaws xxlJob
```

- **结果与限制** 最终 20 组定向测试、竞态检测、两段范例编译、全量构建和原有测试编译通过，格式和 diff 检查通过，OpenSearch/XXL-JOB 无改动。真实本机 HTTP/TLS 服务验证协议和取消，响应体关闭计数、读取故障及 EOF 使用测试替身；未连接真实 Elasticsearch 集群，也未执行原有外部服务集成测试。直接用底层 SDK 须自行提供上下文/预算并释放响应，JSON 同步计算不可强制中断，准确总数查询有统计成本。写入结果不确定和业务幂等由接入方处理。
- **测试清理** 清理前已登记实际命令、结果、初次失败原因和限制；随后删除本次两份 overlay 测试、overlay 配置、全部测试输出、CA 证书、两份范例及编译产物和独立构建缓存。核对任务临时目录与仓库内两处 overlay 目标均不存在，原有五个测试全部保留；原 ES7 测试只更新注释范例。API/RPC 本轮仅更新文档，无新增测试或临时产物。


### 2026-11-20 / DEV-20261120-001 / OpenSearch 独立配置、原生响应与聚合

- **用户确认** 接续 Elasticsearch 7，本轮独立开发 es/esaws，自行维护配置、TLS、连接池、错误和文档，不建立共用类型或兼容入口；保留 BusinessCode 后缀、聚合及 IndexSchema 全量覆盖。XXL-JOB 尚未处理。
- **独立类型与配置** 根包声明改为 esaws，自行定义 IndexTable/Schema；新增独立 core.Config/TLSConfig/TransportConfig，不引用 es/esv7。缺省系统证书验证及 TLS 1.2，自定义 CAFile 追加信任；StartupTimeout 5s、模型 RequestTimeout 10s，TCP/TLS/响应头阶段 5s，连接池三项上限 100、空闲保留 90s，省略或零值采用默认，负值及非法地址/连接上限拒绝。MaxRetries 改为 *int，nil 默认 3、显式 0 关闭，配置不修改调用方对象。
- **初始化与 SDK 边界** NewEs(ctx, conf)/MustNewEs(ctx, conf) 执行受控 Info 探活，失败释放空闲连接；删除 InitESClient 和旧 MustNewEs 签名。当前 opensearch-go v1.1.0 内部产品探活使用 Background，关闭该隐式探活，改由本包检查连接、认证、HTTP 状态和 JSON 正文。删除原不可取消的随机指数退避，不设置使用 time.Sleep 的 SDK RetryBackoff；重试仍按默认网络错误及 502/503/504 触发并共用操作额度，不额外开启超时重试。关闭重试时包装直接 io.EOF，防止 SDK 特例绕过 DisableRetry，并保留 errors.Is 链。
- **路径和生命周期** SDK v1 在重试时重复追加节点路径前缀；通过本套 Perform 记录原始路径和 RawPath，网络请求使用副本，恢复 SDK 原始路径以供下一次重试/切换节点，保留文档 ID 转义及调用方 URL。生成 API 绑定到当前实例，底层原生 Transport 保留 SDK Metrics/DiscoverNodes 能力。Close 可并发或重复调用，释放空闲连接并拒绝新请求，在途请求由自身 ctx 取消；OperationContext 提供单次模型总预算，父级更早截止时间优先。
- **原生响应与错误** 删除 Elasticsearch v7/v8 响应、聚合类型及强制转换，统一使用 opensearchapi.Response。ReadResponse/CheckResponse/Get/Search 在正常、404、错误状态、取消、读取/解码/关闭失败时释放正文，不先 drain 再取错误，也不先关闭再读取；错误正文最多读取 4KiB，错误类型摘要最多 128 字节，不直接输出原始正文或 error.reason。独立 ErrNotFound/ErrInvalidResponse/ErrClosed 和 ResponseError 支持 errors.Is/As，删除旧 EsNotFound/Es500Err；Get 的 404/found=false 返回 ErrNotFound，直接解码到业务指针并设置 Schema ID。
- **模型与聚合** 七类操作继承 model.Ctx，使用单次 RequestTimeout，删除业务调用中的 Background。BusinessCode 非空时模型文档与搜索操作使用基础名加下划线及业务码；IndexAuto 继续接受完整索引名，不自动加后缀。保留 create 自动 ID/稳定 ID、doc 局部更新和 IndexSchema 非空 ID 全量覆盖及 refresh=true。Search 返回 (int64, core.Aggregations, error)，不返回已关闭响应；Aggregations 为本套定义的 map[string]json.RawMessage，未知/嵌套聚合和服务端原始数值保留，空聚合返回空 map，空命中为空切片，固定 track_total_hits=true 并注明统计成本。
- **注释、文档与现有调用** 新增 [OpenSearch 使用说明](es/esaws/README.md)，包含独立 YAML 默认值、初始化/业务后缀/查询与聚合解码、全量覆盖及底层 SDK 超时释放范例。删除未启用旧 transport、手工重复认证头函数及孤立指标；认证由 SDK 自行处理。原核心测试仅更新注释范例，原 es/test 集成测试更新 ctx、指针重试配置、Close 和 Search 范例，用 errors.As/StatusCode 替换旧错误文案匹配，保留原测试。API/RPC 仅登记接入约定。
- **定向验证** 最终 23 组 TestOS 测试及 -race 通过，core 14 组、model 9 组。真实本机 HTTP/TLS 覆盖单次受控探活、Basic Auth 单头、连接池覆盖、默认不信任证书拒绝/自定义 CA/测试跳过/无效 CA、启动响应头和正文超时、父级取消、阶段超时；验证重试 0/2 次、直接 EOF 的关闭/开启重试、节点前缀不重复和文档 ID 转义、调用方 URL 不变、Close 并发与后续拒绝。
- **模型与解析验证** 覆盖七类操作成功及取消/超时、BusinessCode 有无后缀和显式索引名、404 错误映射、doc 更新与 refresh 覆盖写入、稳定 ID 重试时正文复用及自动 ID 行为、在途父级取消/正文读取/多次重试共用预算；正常/错误/读取及关闭失败的正文释放、摘要长度与 UTF-8 边界、nil 及必要字段校验、大 int64、total 两种格式、空命中/空聚合、嵌套桶和未知聚合原样解码。go list 生产依赖树无 Elasticsearch 或 es/esv7 引用，按任务开始时的 SHA-256 快照验证 ES7 和 XXL-JOB 文件未被本轮修改。
- **执行与结果** 使用 GOCACHE=/private/tmp/new-platform-opensearch.djrNAnWE/cache、GOPROXY=off，定向测试设置 OS_TEST_ARTIFACTS=/private/tmp/new-platform-opensearch.djrNAnWE/artifacts；本地监听测试经权限审核运行，不连接外部服务。首次 core 14 组通过，model 两个用例错误假设 SDK create 路径为 /index/_create/id；按实际 v1 原生 /index/_doc/id/_create 修正临时测试预期后通过，未为迎合测试改变业务请求。最后调整 API 绑定保留原生 Transport 能力后再次完整回归，最终 23 组通过。三段 README Go 范例抽取编译通过；全量构建、原有测试编译、gofmt 和 diff 检查通过，无下载或新依赖。

```text
go build -mod=readonly -buildvcs=false ./common-pkg/es/esaws/...
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-opensearch.djrNAnWE/overlay.json -run '^TestOS' -count=1 -v ./common-pkg/es/esaws/... -timeout=45s
go list -deps -mod=readonly -buildvcs=false ./common-pkg/es/esaws/...
go build -mod=readonly -buildvcs=false -o <临时目录>/example_N <临时目录>/example_N.go  （N=1..3，逐一执行）
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/es/esaws/core/*.go common-pkg/es/esaws/model/*.go common-pkg/es/esaws/esInterface.go common-pkg/es/test/es_test.go
git -C common-pkg diff --check
git diff --check
```

- **限制** 使用真实本地 HTTP/TLS 和当前 SDK 验证协议行为，故障正文和 EOF 使用测试替身；未连接真实 OpenSearch 集群或执行原有集成测试业务逻辑。初始化探活不替代实际集群/索引权限及版本联调；SDK 高级调用须自行提供上下文预算并释放正文，JSON 同步计算不可强制中断，准确总数和 refresh=true 有额外成本，写入结果不确定与幂等由接入方处理。当前 SDK 节点发现等高级功能保留，未新增其真实集群联调。
- **测试清理** 清理前已登记实际命令、结果、首次测试预期错误和限制；随后删除本次两份临时 overlay 测试、配置、全部测试输出、CA 文件、依赖清单、快照、三份抽取范例及编译产物和独立构建缓存。已核对任务临时目录与两处仓库 overlay 目标均不存在，原有五个测试全部保留，原 OpenSearch 测试仅按新接口同步；按删除前快照对照确认本轮未改 ES7/XXL-JOB 文件。API/RPC 仅更新文档，无新增测试或临时产物。


### 2026-11-20 / DEV-20261120-002 / XXL-JOB 监听、心跳及任务关闭生命周期

- **用户确认** 接管当前 SDK 无法完整关闭的 HTTP 服务、注册心跳和任务上下文，增加 Start(ctx)/Stop(ctx) 错误返回、明确启动结果、关闭预算及任务注册校验，更新范例和说明。用户随后明确回复“沿用现有协议”，确认现有 /run 与数组回调协议，无需修改当前实现。协议结构以官方 2.4.1 为验证基准，未将该基准视为用户指定的部署版本，实际版本待联调。
- **配置与 API** NewClient(Config) 返回 (*Client,error)，不监听或注册；配置 AdminAddress/AccessToken/RegistryKey/ExecutorAddress 必填，ListenAddress 缺省 :9999。RequestTimeout 默认 5s、RegistryInterval 20s、ShutdownTimeout 10s，HTTP 请求体上限 MaxBodyBytes 默认 1MiB，已接受未完成任务总数 MaxTasks 默认 1024；零值采用默认、负值及非法地址/端口拒绝，注册地址不得为未指定 IP。新增和修改的配置、公开接口和关键路径补齐中文注释。
- **启动与关闭** Start 使用服务生命周期 ctx，先监听再完成首次注册，监听或注册失败返回阶段错误并完成有界清理；异步 Serve 错误通过 Wait 返回，不退出进程。Stop 立即拒绝新任务、关闭监听并取消任务，等首次注册和心跳本地请求退出后再摘除，防止后台再次周期注册；ShutdownTimeout 覆盖 HTTP 收尾、摘除、任务和回调等待，超限强制关闭 HTTP/取消回调后返回超时。未启动、重复或并发 Stop 均安全。调用方 Stop/Wait 的 ctx 只中止等待，不放弃后台有界清理；父 Start ctx 取消自动触发关闭。一实例只能启动一次，关闭后须重新创建。
- **任务注册与调度** Register 改为 error 返回且批量原子提交，拒绝 nil/typed nil、空名、重复名、nil 函数和启动后追加；可选 SetLogHandler 仅启动前设置。保留现有 HandlerFun 的 string 结果签名，普通返回按成功，超时、取消、panic 按失败。每次调度独立快照避免 SDK 复用任务对象的数据竞争；同 JobID 串行 FIFO、DISCARD_LATER 拒绝忙碌任务、COVER_EARLY 取消旧任务及排队任务并等待旧处理器退出后执行最新任务，不强制并发启动不能终止的 Go 代码。kill 幂等，未退出任务仍忙碌。受控排队与回调计入统一容量，活跃 LogID 仅对相同请求去重，不提供持久化去重。
- **上下文与回调** 调度入口沿用合法 HTTP Request/Trace ID，缺省按平台规则生成独立 Trace；不继承启动请求的 ID 或 /run 响应后的取消。ExecutorTimeout 按秒从实际处理开始计时，为 0 不加执行时限，所有任务仍响应服务关闭。业务修改参数不会影响原 LogID/LogDateTime 回调；业务退出后回调沿用原 Trace，通过独立网络和关闭预算上报取消失败。回调不阻塞业务串行队列，结果摘要最多 1024 字节并规范无效 UTF-8；本轮保持单次回调，失败记录日志，不增加自动重试或持久化队列。
- **协议与日志** 五个执行器端点均要求单一正确令牌、POST 和有界 JSON 正文，拒绝重复令牌、未知 handler/策略、非法参数及非 BEAN 脚本任务。管理请求使用独立连接池、有界正文读取和 RequestTimeout，拒绝重定向，不打印令牌、完整参数或调度中心原始正文。/log 支持带 ctx 的日志提供器，默认明确提示使用应用日志系统，不伪造历史日志。删除 SDK Init/Run/Stop、MustStart、旧构造/链式注册及无参 Stop、旧 Logger 适配器和 demo.Old，依赖仅复用协议 DTO。
- **说明与示例** 新建 [XXL-JOB 使用说明](xxlJob/README.md)，说明配置、API 签名、Start ctx 的整个生命周期含义、Stop 等待取消、任务策略、链路及回调限制，并提供完整程序、主动关闭及日志提供器三段 Go 范例；demo/main.go 改为环境变量、应用退出信号、Start/Wait 和 ctx 业务示例，不运行外部执行器或无限等待。
- **定向验证** 16 组本机 HTTP/协议/生命周期测试及 -race 通过，另 1 组 UTF-8/JSON 边界 -race 通过，共 17 组。覆盖配置缺省/非法值和注册原子性；Stop-before-Start、端口冲突不注册、注册失败释放端口、首次注册时并发 Stop 无死锁；多次/并发 Stop 仅摘除一次、停止后无心跳、监听端口可重新绑定；父 ctx 自动停止、Wait 取消不停止服务；非法/重复令牌、HTTP 方法/路径、超大/null 请求、未知任务/脚本和非法 Trace；任务快照、并行独立 JobID 与 Trace、串行/丢弃/覆盖/重复活跃调度；kill 及 Stop 对排队任务失败回调；实际调度超时及 panic 脱敏；不响应 ctx 的任务关闭超时和晚退出不再发送回调；调用方 Stop 截止后后台继续清理；摘除超时及异步监听错误可观察；日志提供器、总容量、回调失败不阻塞后续业务、1024 字节 UTF-8 和额外 JSON 拒绝。
- **执行记录与排障** 初次包构建缺少 time 导入、临时测试混用有名/无名参数导致编译失败，均修正后验证通过。第一次本地端口测试的自动权限审核未在期限内完成，工具允许重试；先以沙箱内编译检查验证脚本，再按提示重试获准，16 组本地测试通过。最后规范无效 UTF-8 摘要，并以额外无监听边界用例验证。三段 README 范例抽取编译、全量构建和原有测试编译通过，格式、diff 和本地文档链接检查通过；按开始时 SHA-256 快照确认搜索和 AMQP 源码未被本轮修改。
- **实际命令** 在平台根目录使用独立 GOCACHE=/private/tmp/new-platform-xxljob.tQv5N685/cache 和 GOPROXY=off，无下载或新增依赖。临时测试、overlay、输出、源码快照、抽取范例与产物均在同一临时目录。

```text
go build -mod=readonly -buildvcs=false ./common-pkg/xxlJob/...
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-xxljob.tQv5N685/overlay.json -run '^$' ./common-pkg/xxlJob
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-xxljob.tQv5N685/overlay.json -run '^TestXXL' -count=1 -v ./common-pkg/xxlJob -timeout=45s
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-xxljob.tQv5N685/overlay.json -run '^TestXXLHelperBoundaries$' -count=1 -v ./common-pkg/xxlJob
go build -mod=readonly -buildvcs=false -o <临时目录>/example_N <临时目录>/example_N.go  （N=1..3，逐一执行）
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/xxlJob/*.go common-pkg/xxlJob/demo/main.go
git -C common-pkg diff --check
git diff --check
```

- **限制** 使用真实本机 HTTP 监听和本地调度中心替身，未连接实际调度中心或发送外部回调；版本和部署地址仍须联调，未运行原有外部服务集成测试。Go 无法强制终止忽略取消的任务/日志提供器，关闭预算耗尽后业务须自行退出；队列和回调是内存状态，进程崩溃或关闭预算耗尽不保证回调送达；正常 string 返回没有业务 error 结果通道，业务须明确现有签名语义并自行幂等。注册响应取消后的远端瞬间状态不保证，摘除失败通过错误返回并依靠调度中心过期判定或管理处理。
- **测试清理** 清理前已登记实际命令、结果、初次编译/审核问题及限制；随后删除本次临时测试、overlay、全部输出、快照、三份范例源码及编译产物和独立构建缓存。核对任务临时目录与仓库 overlay 目标均不存在，原有五个测试全部保留；删除前摘要对照确认搜索和 AMQP 未被本轮修改。API/RPC 只更新文档，无新增测试或临时产物。


### 2026-11-20 / DEV-20261120-003 / HTTP 请求客户端与有界响应

- **用户确认与范围** 接续第 17 项，按已讨论方案先处理 commonTool HTTP 请求；随机字符串、TOTP URI 和 Redis 分片本轮不修改。原 GET 10000 秒超时与注释不符，POST 无超时，两者逐次创建客户端、缺少请求上下文及响应体上限。删除旧 GetRequest/PostRequest，不保留兼容包装；仓库 Go 调用检索无残留。
- **独立配置** 新增 HTTPConfig、HTTPTransportConfig 和 NewHTTPClient，配置字段、公开接口及关键逻辑补齐中文备注。总预算默认 10s，响应正文默认 4MiB；TCP/TLS/响应头默认 5s，连接池三项上限默认 100，空闲保留 90s。零值采用默认、负值拒绝，响应上限 MaxInt64 拒绝以避免加一溢出，每节点空闲数不得超过连接总数；不修改调用方配置或进程级默认客户端。HTTPS 使用系统证书验证和最低 TLS 1.2，代理沿用标准环境变量。
- **请求与生命周期** Get/PostJSON 必须传入 ctx，保留原值和取消，父级更早截止时间优先；总预算覆盖 JSON 编码耗时及网络读取，编码失败或编码后超时不发送。共享连接池支持并发，Close 可并发重复调用，拒绝后续请求并释放空闲连接，在途请求受自身上下文控制。请求头规范大小写并复制多值切片，支持单一非空 Host 覆盖；POST 默认 application/json。
- **响应与错误** 所有 2xx 成功，非 2xx 返回可 errors.As 识别的 HTTPStatusError，同时保留 HTTPResponse 的 StatusCode/Header/Body/BodyComplete。正文最多读取上限加一字节，超限保留上限内前缀并返回 ErrHTTPResponseTooLarge；自动 gzip 按解压后大小计。状态、读取、关闭、超限和取消错误可共存，保留 errors.Is/As 链，公开错误文本不输出 URL、凭证或正文。正常、异常和取消均释放响应体，调用方无需再 Close 响应。
- **重定向与链路** 不自动跟随重定向，3xx 原样返回；不增加业务重试，POST 关闭已发送正文的自动重建/重放，标准库尚未发送正文前的连接恢复及 GET 原有恢复机制保留，共用调用预算。业务写入须自行幂等，网络错误不证明远端未处理。工具不自行生成或注入 Trace ID；跨 HTTP 传递已有 ID 的显式范例见新建 [HTTP 使用说明](commonTool/HTTP_CLIENT.md)，其中包含配置、GET、POST JSON、错误和服务关闭说明。
- **定向验证** 20 组 TestHTTPTool 及 -race 全部通过：配置缺省/覆盖/非法值及不修改输入，200/201/202/204/206、查询和多值头，大 int64 JSON/Content-Type/Host/正文不可重建，301/304/400/401/429/500/503 与错误脱敏；空正文/恰好上限/超限/容量/状态和超限并存，gzip 解压后限额；读取及关闭故障和响应快照；JSON/nil ctx/未初始化/非法 URL/Host/头校验，编码消耗预算；GET/POST 取消/总超时、父级截止、正文超时及响应头阶段超时；GET/POST 不跟随重定向、带 Idempotency-Key 的 POST 不重放、连接复用与并发 Close、在途请求正常收尾、24 个并行 POST 隔离及不信任 TLS 证书拒绝。
- **执行与排障** 定向测试以 overlay 注入，未在仓库创建测试文件；实际本机端口测试经权限审核运行。首次 19 组通过，非法头用例错误假设自定义 RoundTripper 会执行标准库头校验，调整临时测试为真实本机服务和原生 Transport 后完整 20 组通过，生产代码未为测试替身增加行为。范例复核脚本曾因源码缩进格式不同触发相等性断言，重新直接抽取当前文档源码后逐一编译通过，不涉及产品或范例编译失败。全量构建、原有测试编译、三段当前文档 Go 范例编译、gofmt 和 diff 检查均通过；按开始时 SHA-256 快照核对 AMQP、搜索、XXL-JOB 和其余三类工具未改动。
- **实际命令** 平台根目录执行，GOCACHE=/private/tmp/new-platform-http.L9u6TtTp/cache、GOPROXY=off，无新增依赖或下载。临时测试、overlay、输出、快照、范例源码/产物和缓存均在同一任务目录。

```text
go build -mod=readonly -buildvcs=false ./common-pkg/commonTool
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-http.L9u6TtTp/overlay.json -run '^$' ./common-pkg/commonTool
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-http.L9u6TtTp/overlay.json -run '^TestHTTPTool' -count=1 -v ./common-pkg/commonTool -timeout=45s
go build -mod=readonly -buildvcs=false -o <临时目录>/example_N <临时目录>/example_N.go  （N=1..3，逐一执行）
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/commonTool/curl.go common-pkg/commonTool/http_config.go
git -C common-pkg diff --check
git diff --check
rg -n 'GetRequest\(|PostRequest\(' common-pkg --glob '*.go'  （无匹配，退出码 1）
```

- **结果与限制** 20 组定向测试及竞态检测、三段范例编译、全量构建及原有测试编译通过。仅使用本机 HTTP/TLS 和故障替身，未调用外部业务接口，未执行原有外部集成测试逻辑。同步 JSON 编码无法强制中断，Close 不强制结束在途请求；实际目标接口认证、响应契约和部署网络由接入业务联调。API/RPC 本轮仅更新接入约定，无业务代码或测试产物。
- **测试清理** 清理前已登记实际命令、结果、排障和限制；随后删除本次临时测试、overlay、全部测试输出、源码快照、三份范例源码及编译产物和独立构建缓存。核对任务临时目录及仓库 overlay 目标均不存在，原有五个测试全部保留；AMQP、搜索、XXL-JOB 及其余三类工具的摘要对照通过。API/RPC 仅更新文档，无新增测试或临时产物。


### 2026-11-20 / DEV-20261120-004 / 加密安全随机字符串

- **用户确认** 第 17 项依次处理，HTTP 已确认；本轮只调整 GenerateRandomString，不改 TOTP、Redis 分片或统一 Trace ID 生成规则。长度 1～4096、固定 A-Z/a-z/0-9、使用加密安全随机源、拒绝采样及错误返回均已在开发前确认。
- **实现** 删除 tool.go 原 math/rand、逐次 rand.Seed 和无错误返回实现；新增 random_string.go，公开签名为 GenerateRandomString(length int) (string, error)，无旧签名包装。使用 crypto/rand.Reader，局部 128 字节缓冲按剩余长度批量 io.ReadFull；只接受 0～247，每个字符对应四个字节值，拒绝 248～255，避免取模偏差。非法长度在读取或分配结果前拒绝，包含极端 int 值；读取失败返回空字符串并用 %w 保留错误链，已生成前缀不返回。无全局播种、缓存或可变源设置，支持并发调用，字段范围和关键逻辑补齐中文备注。
- **接入与说明** 当前仓库 Go 源码无业务调用方，无需业务迁移；新建 [随机字符串使用说明](commonTool/RANDOM_STRING.md)，包含完整错误处理范例、长度/字符集、均匀映射、签名变化与用途边界。生成值不保证唯一，业务按需提供唯一约束、有效期或其他消费控制；本函数不替代专用 Trace ID/TOTP。API/RPC 本轮仅更新接入记录。
- **验证结果** 6 组 TestRandomString 及 -race 全部通过：非法长度（负数、0、4097、最大/最小 int）拒绝且不读源；公开接口 1/2/32/61/62/127/128/129/255/256/4095/4096 长度和字符集；确定性输入全部 256 个字节，先拒绝八个高值，再验证四轮完整字符集、每字符恰好四次且源耗尽；连续两批拒绝值、逐字节短读和跨批补足；读源即时/部分/已生成前缀后的故障、EOF/UnexpectedEOF 的 errors.Is 及不返回前缀；64 个 goroutine 各 16 次共 1024 次调用的长度/字符集与竞态检查。不以概率性的“所有结果不重复”判断正确性。
- **构建与范围核对** 一段文档 Go 范例抽取编译通过；全量构建、原有五个测试的编译（不执行用例）、gofmt、diff 检查通过。按开始时 SHA-256 快照核对除本轮 tool.go 和开发记录外的原有公共包文件未变，HTTP、AMQP、搜索、XXL-JOB、TOTP、Redis 分片和原有测试全部保留。定向测试使用临时 overlay，不在仓库新增测试文件，无下载或新依赖。
- **实际执行命令** 在平台根目录执行，独立 GOCACHE=/private/tmp/new-platform-random.gmftllno/cache、GOPROXY=off。临时测试/overlay、结果输出、源码快照、范例源码/二进制及构建缓存均在同一任务目录。

```text
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-random.gmftllno/overlay.json -run '^TestRandomString' -count=1 -v ./common-pkg/commonTool -timeout=45s
go build -mod=readonly -buildvcs=false -o /private/tmp/new-platform-random.gmftllno/example /private/tmp/new-platform-random.gmftllno/example.go
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/commonTool/tool.go common-pkg/commonTool/random_string.go
git -C common-pkg diff --check
git diff --check
rg -n 'GenerateRandomString|rand.Seed' common-pkg --glob '*.go'
```

- **已知限制** 故障源经内部读取函数注入，未替换全局 crypto/rand.Reader 或人为破坏系统熵源；真实公开接口使用本机系统随机源验证。部分 Go 版本的系统熵源发生底层致命故障会终止进程，本工具只传播以 error 返回的失败，不捕获运行时致命退出；当前工具链为 go1.26.5，模块基线 go1.22，未另装旧工具链运行。定向测试验证字符映射和拒绝逻辑，不构成对操作系统随机性的统计认证；原有外部服务集成测试只编译，未连接外部服务。
- **测试清理** 已在清理前登记实际命令、结果和限制，随后删除本次临时测试、overlay、测试输出、源码快照、文档范例源码/二进制及独立构建缓存。核对任务临时目录和仓库 overlay 目标均不存在，原有五个测试全部保留；无关原有文件 SHA-256 摘要一致，公共包新增文件仅为随机工具源码及使用说明。API/RPC 仅更新文档，无新增测试或临时产物。


### 2026-11-20 / DEV-20261120-005 / TOTP 密钥、URI 与校验

- **用户确认与范围** 随机字符串已确认，继续第 17 项 TOTP；固定 HMAC-SHA1、六位、30 秒，接受当前及上一个窗口，不计算未来窗口。删除旧接口，统一密钥生成、URI 和字符串验证码校验，不修改 Redis 分片或其他工具。
- **接口与密钥** 删除 google_code.go 的 GenGoogleAuthSecret、BuildGoogleQR、CheckGoogleAuthCode 及三个旧私有转换/计算函数，新增 totp.go 的 GenerateTOTPSecret、BuildTOTPURI、VerifyTOTP，无兼容包装。新密钥由 crypto/rand.Reader 读取 20 字节，直接 Base32 编码为 32 个大写无填充字符；读源失败不返回部分值并保留错误链。输入密钥统一严格校验字母表、无填充和规范尾部位，拒绝空值、小写、空白、CR/LF 和非法编码，按 RFC 4226 最小要求解码后至少 16 字节；生成仍采用推荐的 20 字节。配置常量、公开接口、错误及关键流程补齐中文注释。
- **URI** account/issuer 必填、非纯空白、有效 UTF-8，不含冒号或控制字符，保留原值。net/url 分别编码标签与查询，RawPath 保护嵌入斜杠，空格按 %20 编码，发行方前缀与 issuer 参数一致；固定 algorithm=SHA1/digits=6/period=30。只返回 otpauth URI，不生成图片或访问二维码服务，错误文本不输出账号、密钥或 URI。
- **校验与错误** VerifyTOTP 接收字符串，严格六位 ASCII 数字，保留前导零，不自动补零或清理空白。一次读取服务端时间，8 字节大端计数器和 RFC 动态截断，当前及前一候选均做常量时间比较，不提前命中返回；起始窗口避免减一下溢，负 Unix 时间拒绝。ErrInvalidTOTPSecret、ErrInvalidTOTPCode、ErrTOTPMismatch 支持 errors.Is。工具无状态且可并发，重复消费、尝试次数限制及绑定确认由业务原子处理。
- **说明与接入** 新建 [TOTP 使用说明](commonTool/TOTP.md)，包含密钥规则、URI 编码、窗口边界、两个完整 Go 范例、错误分类及 API/RPC 状态管理职责；URI 按密钥保护并交本地二维码组件，绑定需首次校验确认，RPC 负责受控状态存取，API 编排。当前仓库无旧接口业务调用方，源码检索无旧函数残留；API/RPC 只登记接入约定。
- **定向验证** 12 组 TestTOTP 及 -race 全部通过：确定性随机字节编码和真实系统生成；短读、EOF、UnexpectedEOF 和注入故障不返回前缀；16/20/32/64 字节密钥及全部非法格式/非规范填充位；中文/空格/斜杠/加号/百分号/查询字符 URI 往返和五个参数唯一性；非法标签；RFC 4226 十组六位向量；RFC 6238 六组 SHA1 向量按固定六位取末六位，覆盖前导零及 2038 年后；Python 标准库 hmac/hashlib/struct/base64 独立生成 40 组参考结果（四种密钥长度、十个时间点，含计数器超过 32 位）；当前/上一/未来/过旧窗口、Unix 起点、秒/时区边界及负时间；输入格式及错误脱敏；公开接口和重复调用保持无状态；64 goroutine 各 8 次共 512 组生成/URI/校验及竞态检查。两段当前文档 Go 范例编译通过。
- **全量检查与既有失败** commonTool 包单独构建通过，未受影响的四个原有测试包及 language-pkg 测试编译通过。全量构建与原有测试编译失败：es/esv7/model/es_model.go:75 和 es/esaws/model/es_model.go:83 仍调用已不存在的 commonTool.GenUUID，连带 es/test 无法编译。以 HEAD 原 google_code.go 字节建立基线 overlay、隐藏本轮 totp.go 后再次全量构建，同样两处失败；原文件与本轮开始快照哈希一致，确认问题在任务前已存在。本轮未改动搜索或恢复旧 UUID 接口，不把全量失败记为通过；该问题后续单独讨论处理。
- **实际命令** 平台根目录执行，独立 GOCACHE=/private/tmp/new-platform-totp.izj61k8k/cache、GOPROXY=off；定向设置 TOTP_REFERENCE=/private/tmp/new-platform-totp.izj61k8k/reference.json。临时测试和两个 overlay、Python 参考数据、测试/构建输出、原源码快照、抽取范例与产物、缓存都在此任务目录，无依赖下载或新增依赖。

```text
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-totp.izj61k8k/overlay.json -run '^TestTOTP' -count=1 -v ./common-pkg/commonTool -timeout=45s
go build -mod=readonly -buildvcs=false -o /private/tmp/new-platform-totp.izj61k8k/example_1 /private/tmp/new-platform-totp.izj61k8k/example_1.go
go build -mod=readonly -buildvcs=false -o /private/tmp/new-platform-totp.izj61k8k/example_2 /private/tmp/new-platform-totp.izj61k8k/example_2.go
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...  （失败：既有 GenUUID 缺失）
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1  （同一既有编译失败）
go build -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-totp.izj61k8k/baseline-overlay.json ./common-pkg/... ./language-pkg/...  （还原原 TOTP 后重现同一失败）
go build -mod=readonly -buildvcs=false ./common-pkg/commonTool
go test -mod=readonly -buildvcs=false ./common-pkg/commonTool ./common-pkg/aesGCM ./common-pkg/es/esv7/core ./common-pkg/es/esaws/core ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/commonTool/totp.go
git -C common-pkg diff --check
git diff --check
rg -n 'GenGoogleAuthSecret|BuildGoogleQR|CheckGoogleAuthCode|oneTimePassword|func toBytes|func toUint32' common-pkg --glob '*.go'  （无匹配，退出码 1）
```

- **限制** 本机系统随机源、标准向量及故障注入验证，无真实认证器扫码、部署时钟或外部业务联调。读源故障经内部函数注入，未替换全局随机源；Go 系统熵源的致命退出不转换为普通错误，未另装 Go 1.22 工具链验证。格式正确后的候选比较为常量时间，输入校验和整个函数不宣称恒定耗时；不保存使用状态或实现业务限流。原有集成测试未执行，仅尝试编译，全量失败如上登记。
- **测试清理** 清理前已登记实际命令、结果、基线失败及限制；随后删除本轮临时测试、两个 overlay、Python 参考数据、全部测试/构建输出、原源码快照、两份范例源码和编译产物及独立构建缓存。核对任务临时目录及仓库 overlay 目标均不存在，原有五个测试全部保留；除删除旧 TOTP 源码及更新开发记录外，全部原有公共包文件摘要一致，新增文件仅为 totp.go 和使用说明。API/RPC 本轮只更新文档，无新增测试或临时产物。


### 2026-11-20 / DEV-20261120-006 / Redis 逻辑分片错误处理

- **用户确认与范围** 第 17 项最后处理 Redis 分片，TOTP 已确认。只调整 ShardIndexXX/ShardTagXX 的错误返回和说明；分片数必须显式大于 0，不设默认，合法参数路由保持不变。上轮既有 GenUUID 编译问题另列处理，不混入本轮。
- **接口与错误** 两个函数增加 error 返回，参数 n 改名 shardCount；新增 ErrInvalidShardCount，零分片时索引返回 0、标签返回空串且支持 errors.Is，不再发生除零 panic，也不静默选用默认桶。ShardTagXX 先传播索引错误，成功后保持 {s%03d} 格式。删除旧签名，无兼容包装；当前仓库无业务调用方需要迁移。
- **路由语义与注释** 保留有符号十进制用户 ID 文本的 xxhash64 取模，支持全部 int64 ID 和非零 uint64 桶数，返回索引在 [0, shardCount)。标签至少三位，超过三位自然扩展。补齐接口和关键逻辑中文注释，明确业务桶数、Redis 槽和节点的区别，同标签同槽、不同标签可同槽，以及变更桶数的迁移影响。
- **使用说明** 新建 [Redis 分片使用说明](commonTool/REDIS_SHARDS.md)，包含新签名、错误处理、固定算法、完整关联 key 范例及配置/迁移约定。范例同时保留逻辑桶标签和用户 ID，防止同桶不同用户覆盖 key；固定前缀避免花括号改变 Redis 标签解析。工具只计算索引/标签，不选择节点、访问 Redis 或迁移数据；API 继续通过 RPC 编排，实际数据存取方统一管理分片配置。
- **定向验证** 6 组 TestRedisShards 及 -race 通过。先抽取本轮修改前原函数执行，记录 11 个用户 ID（含最小/最大 int64、0、负数及超过浮点精确范围值）与 11 个桶数（含 1、999/1000/1001、16384/16385、2^63、最大 uint64）的 121 组索引和标签，再逐项验证新实现完全一致。覆盖所有代表 ID 的零桶错误和 errors.Is、单桶固定 0/{s000}、10000 个输入的索引范围/标签形状及数值一致性、大编号无截断/溢出，以及 64 goroutine 各 128 次共 8192 组并发稳定路由。使用修改前结果作回归基准，没有靠重复实现同一哈希公式生成预期。
- **构建与既有问题** 使用说明的一段 Go 范例编译、commonTool 单独构建、四个未受影响原有测试包及 language-pkg 测试编译通过；gofmt、diff 和文档本地链接检查通过。全量构建仍失败于 es/esv7/model/es_model.go:75 与 es/esaws/model/es_model.go:83 对已删除 commonTool.GenUUID 的调用，与上轮已还原基线复核的问题一致；搜索源码按任务开始摘要核对未改，不恢复旧接口、不将全量失败记为通过。本轮未重复运行已知无法编译的 es/test 集成测试。
- **实际命令** 在平台根目录执行，独立 GOCACHE=/private/tmp/new-platform-shards.ks0fb_be/cache、GOPROXY=off；定向设置 SHARDS_GOLDEN=/private/tmp/new-platform-shards.ks0fb_be/golden.json。原函数抽取程序、121 组结果、overlay 测试/配置、测试和构建输出、源码摘要、文档范例源码/产物及缓存均在此任务目录，无依赖下载或新依赖。

```text
go run -mod=readonly -buildvcs=false /private/tmp/new-platform-shards.ks0fb_be/baseline.go  （原函数产生 golden.json）
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-shards.ks0fb_be/overlay.json -run '^TestRedisShards' -count=1 -v ./common-pkg/commonTool -timeout=45s
go build -mod=readonly -buildvcs=false -o /private/tmp/new-platform-shards.ks0fb_be/example /private/tmp/new-platform-shards.ks0fb_be/example.go
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...  （失败：既有 GenUUID 缺失）
go test -mod=readonly -buildvcs=false ./common-pkg/commonTool ./common-pkg/aesGCM ./common-pkg/es/esv7/core ./common-pkg/es/esaws/core ./language-pkg/... -run '^$' -count=1
go build -mod=readonly -buildvcs=false ./common-pkg/commonTool
gofmt -l common-pkg/commonTool/redis_shards.go
git -C common-pkg diff --check
git diff --check
rg -n 'ShardIndexXX|ShardTagXX' common-pkg --glob '*.go'
```

- **限制** 纯函数、原路由样本与并发验证，不连接真实 Redis 集群，不验证实际负载热点或执行数据迁移。原有外部集成测试未执行，相关编译检查仅使用 -run '^$'；全量构建限制如上登记。API/RPC 本轮只更新约定，无业务代码或测试产物。
- **测试清理** 清理前已登记实际命令、结果和既有失败；随后删除本次原函数抽取程序、121 组结果、临时测试及 overlay、全部测试/构建输出、源码摘要、范例源码/编译产物及独立构建缓存。核对任务临时目录及仓库 overlay 目标均不存在，原有五个测试全部保留；除 redis_shards.go 与开发记录外，原有公共包文件摘要一致，新增文件仅为使用说明。API/RPC 本轮只更新文档，无新增测试或临时产物。


### 2026-11-20 / DEV-20261120-007 / 搜索模型 UUID 旧引用修复

- **用户确认与范围** 第 17 项 Redis 分片已确认，新增第 18 项处理阻断全量构建的 ES7/OpenSearch GenUUID 旧引用。两套模型空 ID 创建均复用现有 NewUUidV7，生成失败直接返回并保留错误链；不恢复旧接口，不改 UUID 生成器、RPC/API 业务或其他组件。
- **实现与备注** 两处 documentID 在空 ID 且 generate=true 时调用 commonTool.NewUUidV7()，失败以 %w 包装包含各自组件和阶段的错误，返回空 ID；InsertSchema 在正文编码和 SDK 调用前退出。补齐私有 ID 解析与公开创建接口的中文注释，说明 UUIDv7、失败无写入、业务 ID 保留及不写回对象。已有 ID 继续原样使用，UpdateSchema 和 OpenSearch IndexSchema 缺少 ID 仍返回错误，不隐式生成。
- **使用说明** 更新两套独立 README，明确空 create ID 为 36 字符 UUIDv7、同一 SDK 操作重试复用 ID、再次业务调用会重新生成、需要稳定重发标识时预先指定 ID；生成值不是业务自增序号，同一毫秒不保证递增。无新增文档代码范例或接口签名变化。
- **定向验证** 8 组测试及 -race 全部通过，两套模型各 3 组正常行为和 1 组生成失败。正常验证指定业务 ID 在生成开关两种模式下保留、真实生成的 UUIDv7 格式/版本/variant、nil/typed nil/非法 Schema、空 ID 更新拒绝；SDK 503 后成功重试保持同一文档 ID 和正文，自动 ID 不回写且正文只编码一次，响应关闭；两次指定 ID 创建使用同一 ID，更新保留 ID。OpenSearch 另覆盖缺少 ID 的全量覆盖拒绝。
- **失败路径验证** 第二份临时 overlay 仅将 NewUUidV7 替换为返回 os.ErrPermission，分别验证 documentID/InsertSchema 的 errors.Is 错误链、空返回值、零正文编码、零 SDK/传输调用及调用方未修改；指定 ID 创建和更新仍成功，证明这两类操作不依赖生成器。通过临时 SDK 客户端工厂设置正常请求预算并使用内存 HTTP RoundTripper，无本地监听或外部请求；未在产品代码引入可替换函数变量、测试开关或兼容入口。
- **全量恢复与范围核对** 无 overlay 的全量构建和全部原有测试编译恢复通过，原 es/test 的编译阻断解除；未执行原有外部服务集成测试逻辑。Go 源码检索无 GenUUID 残留，gofmt、diff 和说明本地链接检查通过。按任务开始 SHA-256 快照核对，除两套模型源码、README 和开发记录外的原有公共包文件全部未变，UUID 生成器与原有五个测试保留。
- **实际命令** 平台根目录执行，独立 GOCACHE=/private/tmp/new-platform-uuid-ref.51cm8eoh/cache、GOPROXY=off；故障命令额外 UUID_REF_FAILURE=1。两份模型测试、两份临时 core 工厂、失败生成器、两个 overlay、测试/构建输出、源码摘要及缓存均在同一任务目录，仓库内仅通过 overlay 虚拟注入，无依赖下载或新增依赖。

```text
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-uuid-ref.51cm8eoh/overlay.json -run '^TestUUIDRef(DocumentID|CreateAndRetry|UpdateRequiresID)$' -count=1 -v ./common-pkg/es/esv7/model ./common-pkg/es/esaws/model -timeout=45s
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-uuid-ref.51cm8eoh/failure-overlay.json -run '^TestUUIDRefFailure$' -count=1 -v ./common-pkg/es/esv7/model ./common-pkg/es/esaws/model -timeout=45s
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/es/esv7/model/es_model.go common-pkg/es/esaws/model/es_model.go
git -C common-pkg diff --check
git diff --check
rg -n 'GenUUID' common-pkg --glob '*.go'  （无匹配，退出码 1）
```

- **限制** SDK 传输替身验证请求和重试行为，无真实 Elasticsearch/OpenSearch 集群联调。生成失败用临时源码 overlay 注入，未破坏系统熵源；现有生成器和其运行时行为不变，不能将 Go 的致命随机源失败转换为普通错误。自动 ID 不回写及跨业务重发可能生成新 ID 的现有行为保留，业务幂等仍由稳定 ID 控制。
- **测试清理** 清理前已登记实际命令、结果和限制；随后删除本轮两份模型测试、两份临时 core 工厂、失败生成器、两个 overlay、全部测试/构建输出、源码摘要及独立构建缓存。核对任务临时目录和四个仓库虚拟新增目标均不存在，原有五个测试全部保留；除两套模型源码、README 及开发记录外的原有公共包文件摘要一致，UUID 生成器未改动，无新增持久测试或产物。API/RPC 本轮只更新文档，无新增测试或临时产物。


### 2026-11-20 / DEV-20261120-008 / 独立时间工具与 UTC 默认值

- **用户确认与补充** 第 18 项 UUID 修复已确认，开始第 19 项时间工具；用户要求新建独立文件管理，并再次明确未指定时区默认按零时区计算。先前已确认统一 TimeTool、显式基准时间、日历边界、严格解析与秒级差值、删除旧接口的范围。
- **文件与接口** 新增 commonTool/time_tool.go，将八个旧时间函数从 tool.go 移除，无兼容包装；当前仓库无业务调用方。NewTimeTool(timezone) 构造只读实例，提供 DayStart/YesterdayStart/MonthStart/LastMonthStart、Date、FormatUnix、Parse、DiffSeconds，以及 DateTimeLayout/DateLayout 和 ErrAmbiguousLocalTime；公开接口、字段、单位、默认值和关键逻辑均补齐中文注释。tool.go 仅保留原有非时间工具，不改其行为。
- **时区默认** 空配置、UTC、零值和 nil 工具均使用 UTC（零时区），不读取或修改 time.Local。显式 IANA 时区在初始化加载，非法名称和 Local 返回错误不回退。引入标准库 time/tzdata，在系统缺少时区数据时提供内置后备，不新增第三方依赖。调用方处理构造错误，实例可跨请求并发复用，基准时间显式传入。
- **日历边界** 先将基准时刻转换到工具时区，再按年月日取日初、前一日和月初；上月从本月第一天回退，避免 3 月 31 日减月归一化回本月。相邻时区切换的偏移与边界参与候选判断，零点不存在取首个有效时刻、重复零点取较早者，整日被跳过时边界为跳变后的首个有效时刻，对应空日区间；不假设每个日历日都等于 24 小时。
- **格式与解析** FormatUnix 输入单位为秒，0 为合法 Unix 起点，负数为起点之前；Date 按指定时区直接格式化。Parse 严格 YYYY-MM-DD HH:MM:SS，拒绝非法日期、额外空白、小数秒及不规范格式，拒绝跳变中不存在或有歧义的本地时间；底层 time.ParseError 保留 errors.As 链，重复时间以 ErrAmbiguousLocalTime 供 errors.Is 判断。DiffSeconds 返回 end-start 实际秒差，允许负值，解析失败不继续计算；使用 Unix 秒差避免 time.Duration 跨约 292 年的饱和。
- **使用说明与迁移** 新建 [时间工具使用说明](commonTool/TIME_TOOL.md)，突出默认 UTC，包含接口/单位、两个完整 Go 范例、日历和夏令时行为、八个旧接口迁移表，以及 API/RPC 按业务契约传入时区的约定。当前时间直接使用 time.Now()/Unix，不新增只读当前时间的包装；工具不自动读取请求头或修改服务时区。
- **定向验证** 15 组 TestTimeTool 及 -race 全部通过：默认/零值/nil UTC、非法时区，UTC/上海/胡志明/印度/尼泊尔/洛杉矶跨日，闰年/跨年/月底和时区跨月，纽约 23/25 小时日，圣保罗零点跳过、哈瓦那重复零点、阿皮亚整日跳过；0/负秒格式化及往返，严格格式/非法日期/不存在时刻与 ParseError 链，歧义时刻及前后合法时刻；秒差正负零、夏令时实际秒差和 0001～9999 年超长跨度，错误阶段和歧义链；Python datetime/zoneinfo 在十个时区和十个时间点独立生成的 100 组日/月边界与格式化参考全部一致；64 goroutine 各 64 次共 4096 组跨实例只读调用及原始基准时间不变。
- **非零服务器时区与全量检查** 单独以 TZ=America/Los_Angeles 再执行默认 UTC 用例及 -race，验证不随服务器本地时区改变；两段说明 Go 范例均抽取编译，独立 main 实际运行输出与文档一致。无 overlay 全量构建、全部原有测试编译、gofmt、diff 和本地文档链接检查通过，Go 源码无旧时间函数残留。按开始时摘要核对，除 tool.go 与开发记录外的原有公共包文件全部未变，上一项未提交的 UUID 修复完整保留。
- **实际命令** 平台根目录执行，独立 GOCACHE=/private/tmp/new-platform-time.nz7q40l0/cache、GOPROXY=off，定向设置 TIME_TOOL_REFERENCE=/private/tmp/new-platform-time.nz7q40l0/reference.json；附加默认值命令使用 TZ=America/Los_Angeles。临时测试/overlay、100 组参考、测试/构建输出、源码摘要、抽取范例/产物和缓存均在此任务目录，无依赖下载。

```text
go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-time.nz7q40l0/overlay.json -run '^TestTimeTool' -count=1 -v ./common-pkg/commonTool -timeout=45s
TZ=America/Los_Angeles go test -race -mod=readonly -buildvcs=false -overlay=/private/tmp/new-platform-time.nz7q40l0/overlay.json -run '^TestTimeToolDefaultUTC$' -count=1 -v ./common-pkg/commonTool
go build -mod=readonly -buildvcs=false -o /private/tmp/new-platform-time.nz7q40l0/example_1 /private/tmp/new-platform-time.nz7q40l0/example_1.go
go build -mod=readonly -buildvcs=false -o /private/tmp/new-platform-time.nz7q40l0/example_2 /private/tmp/new-platform-time.nz7q40l0/example_2.go
/private/tmp/new-platform-time.nz7q40l0/example_1
go build -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/...
go test -mod=readonly -buildvcs=false ./common-pkg/... ./language-pkg/... -run '^$' -count=1
gofmt -l common-pkg/commonTool/tool.go common-pkg/commonTool/time_tool.go
git -C common-pkg diff --check
git diff --check
rg -n 'TimeToString|DiffTimeUnix|GetNowTime|GetTodayZeroTimeInt|GetYesterdayZeroTimeInt|GetMonthZeroTimeInt|GetLastMonthZeroTimeInt|GetXZeroTodayTimeInt' common-pkg --glob '*.go'  （无匹配，退出码 1）
```

- **限制** 不执行原有外部服务集成测试，仅编译；未在实际 API/RPC 服务部署或另外安装 Go 1.22 工具链。内置 tzdata 后备通过标准库接入，未删除系统数据以模拟缺失，实际规则版本仍由部署数据或 Go 内置数据决定。无偏移本地字符串不能表达回拨歧义时刻，工具返回错误，需要精确表达时使用带偏移的跨服务协议。工具不提供闰秒、业务工作日或节假日计算，不自动修改业务时区配置。
- **测试清理** 清理前已登记实际命令、结果和限制；随后删除本轮临时测试、overlay、100 组参考数据、全部测试/构建输出、源码摘要、两份范例源码/编译产物及独立构建缓存。核对任务临时目录和仓库 overlay 目标均不存在，原有五个测试全部保留；除 tool.go 和开发记录外的原有公共包文件摘要一致，包括上一项未提交的 UUID 修复；新增持久文件仅为 time_tool.go 与使用说明。API/RPC 只更新文档，无新增测试或临时产物。
