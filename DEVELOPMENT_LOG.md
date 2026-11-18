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
| 11 | 请求头校验、CORS 和并发上限 | 已完成确认范围内的 CORS 和并发上限调整，待确认 |
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
