# TOTP 使用说明

使用 `github.com/K7mQ2vR9/base-pkg/commonTool` 提供的三个接口：

```text
GenerateTOTPSecret() (string, error)
BuildTOTPURI(secret, account, issuer string) (string, error)
VerifyTOTP(code, secret string) error
```

算法固定为 **HMAC-SHA1、六位数字、30 秒窗口**，Unix 起始时间为 0。旧 `GenGoogleAuthSecret`、`BuildGoogleQR`、`CheckGoogleAuthCode` 已删除，不提供兼容包装。

## 密钥及 URI

`GenerateTOTPSecret` 从加密安全随机源读取 20 字节，直接编码为无填充的大写 Base32，共 32 个字符，不做字母取模转换。读源以 error 返回失败时，返回空串及保留底层错误链的错误，不返回部分密钥、不回退随机源。部分 Go 版本的系统随机源会在底层致命失败时终止进程，此类运行时行为无法转为普通错误。

`BuildTOTPURI` 和 `VerifyTOTP` 使用相同的密钥校验规则：

- 仅接受规范的无填充大写 Base32，字符集为 `A-Z`、`2-7`。
- 解码后至少 16 字节，生成器默认使用 20 字节。
- 拒绝空值、小写、空白、CR/LF、`=` 填充、非法长度及非零尾部填充位，不自动清理输入。

最小密钥长度依据 [RFC 4226 第 4 节](https://www.rfc-editor.org/rfc/rfc4226.html#section-4)，生成长度采用其推荐的 160 位。

`account` 和 `issuer` 均必填，必须是非空白的有效 UTF-8 文本，禁止冒号及控制字符。账号可使用邮箱或业务标识，发行方使用应用名称；其余原文保留。标签分别编码，路径中的 `/` 编码为 `%2F`，查询参数由标准库构造，中文、空格、`&`、`#`、`?`、`+`、`%` 均作为值处理，不拼入额外参数。标签前缀与查询参数中的发行方相同，遵循 [Google Authenticator URI 规范](https://github.com/google/google-authenticator/wiki/Key-Uri-Format)。

```go
package examples

import "github.com/K7mQ2vR9/base-pkg/commonTool"

// BeginTOTPEnrollment 返回待绑定密钥及 URI，供业务保存待确认状态和本地二维码组件使用。
func BeginTOTPEnrollment(account, issuer string) (secret, uri string, err error) {
    secret, err = commonTool.GenerateTOTPSecret()
    if err != nil {
        return "", "", err
    }
    uri, err = commonTool.BuildTOTPURI(secret, account, issuer)
    if err != nil {
        return "", "", err
    }
    return secret, uri, nil
}
```

URI 包含完整共享密钥，与密钥采用相同保护方式。业务将 URI 交给本地二维码组件，不上传第三方二维码网站，不记录到日志。用户首次提交有效验证码后，再由业务原子确认绑定。公共包只返回 URI，不生成二维码图片、不发起外部请求。

## 验证码校验

验证码通过 JSON 等接口传递时使用字符串，例如 `"005924"`，不得先转为整数。`VerifyTOTP` 严格要求六位 ASCII 数字，不接受空格、正负号、全角数字或长度不足；格式合法的候选值用常量时间比较。

服务端只读取一次当前 Unix 时间，计算当前窗口及上一个窗口，两者均完成比较后返回；不计算未来窗口。Unix 起始窗口不计算负窗口。按秒的窗口示例：

| 服务端时间 | 参与校验的窗口 |
|---|---|
| Unix 秒 29 | 0～29 秒 |
| Unix 秒 30 | 30～59 秒及 0～29 秒 |
| Unix 秒 60 | 60～89 秒及 30～59 秒 |

上一个窗口用于容忍临近切换的提交延迟；设备时间超前仍可能失败，服务端和设备须保持时钟同步。时间计算沿用 [RFC 6238](https://www.rfc-editor.org/rfc/rfc6238.html)，测试使用其 SHA1 测试向量，并按本工具固定的六位长度取原八位表值的末六位。

```go
package examples

import (
    "errors"

    "github.com/K7mQ2vR9/base-pkg/commonTool"
)

// CheckTOTPValue 演示校验及错误分类；调用前由业务加载当前用户的待确认或已绑定密钥。
// 返回成功只表示数学匹配，业务仍须原子消费验证码，再确认绑定或继续登录操作。
func CheckTOTPValue(code, storedSecret string) (matched bool, err error) {
    err = commonTool.VerifyTOTP(code, storedSecret)
    switch {
    case err == nil:
        return true, nil
    case errors.Is(err, commonTool.ErrInvalidTOTPCode), errors.Is(err, commonTool.ErrTOTPMismatch):
        return false, nil
    default:
        return false, err
    }
}
```

| 错误 | 含义 |
|---|---|
| `ErrInvalidTOTPSecret` | 密钥格式非法或解码后不足 16 字节 |
| `ErrInvalidTOTPCode` | 验证码不是六位 ASCII 数字 |
| `ErrTOTPMismatch` | 验证码未匹配两个候选窗口 |

使用 `errors.Is` 分类，不匹配错误字符串。错误文本不包含密钥、URI、账号或验证码输入。业务对外按统一认证错误契约返回，密钥配置异常由服务端处理。

## API、RPC 与业务状态

工具无状态且支持并发，相同窗口内重复提交正确验证码仍会数学匹配。业务须结合用户和密钥版本，原子记录验证码消费状态、限制尝试次数，并管理绑定确认、解绑和恢复流程；多实例共享相同消费状态，去重有效期须覆盖剩余可接受窗口。不能先查询“未使用”再单独写入，这会产生并发重复消费。

RPC 数据模型服务负责密钥与消费状态的受控存取，API 调用 RPC 编排绑定或验证业务。密钥需要可用于 HMAC 校验，按项目密钥管理方案加密保存，不使用只可单向验证的密码哈希替代。公共包本轮不新增数据库、Redis 消费记录、请求限流或业务接口。

定向验证使用标准向量、本机随机源、独立 Python 参考结果和注入的读源故障，不连接真实认证器应用；扫码与部署时钟同步仍需接入环境联调。
