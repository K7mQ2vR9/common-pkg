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
| 15 | RabbitMQ 队列绑定、重连和重新订阅 | 已实现重试队列及耗尽后 TG 通知，已验证，待确认 |
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
