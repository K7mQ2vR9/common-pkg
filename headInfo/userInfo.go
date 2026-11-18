package headInfo

import (
	"context"
	"encoding/json"
)

type userInfoContextKey struct{}

// 使用具名类型，避免上下文的字符串表示直接输出 JSON。
type userInfoJSON string

// SetUserInfo 将认证后的用户信息存入请求上下文。空值清除已有的用户信息。
func SetUserInfo(ctx context.Context, info json.RawMessage) context.Context {
	return context.WithValue(ctx, userInfoContextKey{}, userInfoJSON(info))
}

// GetUserInfo 返回当前登录用户的 JSON 信息副本。未设置时返回 nil。
func GetUserInfo(ctx context.Context) json.RawMessage {
	info, _ := ctx.Value(userInfoContextKey{}).(userInfoJSON)
	if info == "" {
		return nil
	}
	return json.RawMessage(info)
}
