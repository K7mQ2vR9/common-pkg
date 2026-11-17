package appMiddleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/K7mQ2vR9/base-pkg/consts"
	"github.com/K7mQ2vR9/base-pkg/ctxMd"
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
	"google.golang.org/grpc/metadata"
)

type (
	CheckRequestTokenFuncWithErr func(r *http.Request, token string) (int64, error)
	CheckRequestTokenFunc        func(r *http.Request, token string) int64
)

func verifyPath(urlPath string, noVerifyPath map[string]int) bool {
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
	ctx := r.Context()

	token := strings.Trim(r.Header.Get(consts.HeaderToken), "")
	wPathBool := verifyPath(r.URL.Path, noVerifyPath)
	if !wPathBool && token == "" {
		return r, xcode.UserNotFound
	}

	if token != "" {
		var tokenUid int64 = 0
		if checkToken != nil {
			tokenUid = checkToken(r, token)
		}
		if tokenUid > 0 {
			md := ctxMd.SetMdCtxFromOut(ctx, consts.TokenUid, strconv.FormatInt(tokenUid, 10))
			ctx = metadata.NewOutgoingContext(ctx, md)
		} else {
			if !wPathBool {
				if tokenUid == -1 {
					return r, xcode.TokenPermissionDeniedWithMsg(headInfo.GetPath(ctx))
				}
				return r, xcode.TokenInvalid
			}
		}
	}

	newReq := r.WithContext(ctx)

	return newReq, nil
}

func MustAuthTokenRequestWithErr(r *http.Request, checkToken CheckRequestTokenFuncWithErr, noVerifyPath map[string]int) (*http.Request, error) {
	ctx := r.Context()

	token := strings.Trim(r.Header.Get(consts.HeaderToken), "")
	wPathBool := verifyPath(r.URL.Path, noVerifyPath)
	if !wPathBool && token == "" {
		return r, xcode.TokenNotLoginWithMsg("User not login.")
	}

	if token != "" {
		var tokenUid int64 = 0
		var err error
		if checkToken != nil {
			tokenUid, err = checkToken(r, token)
			if err != nil {
				return r, xcode.TokenSpecial(err)
			}
		}
		if tokenUid > 0 {
			md := ctxMd.SetMdCtxFromOut(ctx, consts.TokenUid, strconv.FormatInt(tokenUid, 10))
			ctx = metadata.NewOutgoingContext(ctx, md)
		} else {
			if !wPathBool {
				if tokenUid == -1 {
					return r, xcode.TokenPermissionDeniedWithMsg(headInfo.GetPath(ctx))
				}
				return r, xcode.TokenInvalid
			}
		}
	}

	newReq := r.WithContext(ctx)

	return newReq, nil
}
