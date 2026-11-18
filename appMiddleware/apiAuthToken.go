package appMiddleware

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/K7mQ2vR9/base-pkg/consts"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
)

type (
	CheckRequestTokenFuncWithErr func(r *http.Request, token string) (int64, error)
	CheckRequestTokenFunc        func(r *http.Request, token string) int64
	// CheckRequestTokenFuncWithUserInfo 返回已验证的用户 ID 和可选的用户信息 JSON。
	CheckRequestTokenFuncWithUserInfo func(r *http.Request, token string) (uid int64, userInfo json.RawMessage, err error)
)

func isWhitelistedPath(urlPath string, noVerifyPath map[string]int) bool {
	if _, ok := noVerifyPath[urlPath]; ok {
		return true
	}

	for path := range noVerifyPath {
		if strings.HasPrefix(path, "/") && strings.HasSuffix(path, "*") {
			prefix := strings.TrimSuffix(path, "*")
			if strings.HasPrefix(urlPath, prefix) {
				return true
			}
		}
	}
	return false
}

func MustAuthTokenRequest(r *http.Request, checkToken CheckRequestTokenFunc, noVerifyPath map[string]int) (*http.Request, error) {
	var check CheckRequestTokenFuncWithErr
	if checkToken != nil {
		check = func(r *http.Request, token string) (int64, error) {
			return checkToken(r, token), nil
		}
	}
	return MustAuthTokenRequestWithErr(r, check, noVerifyPath)
}

// MustAuthTokenRequestWithErr 统一处理必需认证和白名单上的可选认证。
func MustAuthTokenRequestWithErr(r *http.Request, checkToken CheckRequestTokenFuncWithErr, noVerifyPath map[string]int) (*http.Request, error) {
	return mustAuthTokenRequestWithUserInfo(r, adaptTokenCheck(checkToken), noVerifyPath)
}

func adaptTokenCheck(checkToken CheckRequestTokenFuncWithErr) CheckRequestTokenFuncWithUserInfo {
	if checkToken == nil {
		return nil
	}
	return func(r *http.Request, token string) (int64, json.RawMessage, error) {
		uid, err := checkToken(r, token)
		return uid, nil, err
	}
}

func mustAuthTokenRequestWithUserInfo(r *http.Request, checkToken CheckRequestTokenFuncWithUserInfo, noVerifyPath map[string]int) (*http.Request, error) {
	uid, userInfo, err := authenticateToken(r, checkToken)
	if err == nil {
		ctx := headInfo.SetTokenUid(r.Context(), strconv.FormatInt(uid, 10))
		return r.WithContext(headInfo.SetUserInfo(ctx, userInfo)), nil
	}
	if isWhitelistedPath(r.URL.Path, noVerifyPath) {
		// 白名单允许匿名访问，但认证失败时不得沿用上下文中的用户身份。
		ctx := headInfo.SetTokenUid(r.Context(), "")
		return r.WithContext(headInfo.SetUserInfo(ctx, nil)), nil
	}
	return r, err
}

func authenticateToken(r *http.Request, checkToken CheckRequestTokenFuncWithUserInfo) (int64, json.RawMessage, error) {
	token := strings.TrimSpace(r.Header.Get(consts.HeaderToken))
	if token == "" {
		return 0, nil, xcode.UserNotFound
	}
	if checkToken == nil {
		return 0, nil, xcode.StatusServiceUnavailable
	}

	uid, userInfo, err := checkToken(r, token)
	switch {
	case err != nil:
		return 0, nil, xcode.TokenSpecial(err)
	case uid == -1:
		return 0, nil, xcode.TokenPermissionDenied
	case uid <= 0:
		return 0, nil, xcode.TokenInvalid
	case len(userInfo) != 0 && !json.Valid(userInfo):
		return 0, nil, xcode.StatusServiceUnavailable
	}
	return uid, userInfo, nil
}
