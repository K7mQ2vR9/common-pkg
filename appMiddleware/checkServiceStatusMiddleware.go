package appMiddleware

import (
	"github.com/K7mQ2vR9/base-pkg/headInfo"
	"github.com/K7mQ2vR9/base-pkg/result"
	"github.com/K7mQ2vR9/base-pkg/result/xcode"
	"net/http"
)

type CheckServiceStatusFunc func(r *http.Request, token string) bool
type CheckServiceStatusMiddlewareOption func(m *CheckServiceStatusMiddleware)

func WithCheckServiceStatusCheckOption(check CheckServiceStatusFunc) CheckServiceStatusMiddlewareOption {
	return func(m *CheckServiceStatusMiddleware) {
		m.check = check
	}
}

type CheckServiceStatusMiddleware struct {
	check        CheckServiceStatusFunc
	noVerifyPath map[string]int
}

func NewCheckServiceStatusMiddleware(noVerifyPath map[string]int, ops ...CheckServiceStatusMiddlewareOption) *CheckServiceStatusMiddleware {
	res := &CheckServiceStatusMiddleware{
		noVerifyPath: noVerifyPath,
	}
	for _, op := range ops {
		op(res)
	}
	if res.check == nil {
		panic("service status checker is required: use WithCheckServiceStatusCheckOption")
	}
	return res
}

func (m *CheckServiceStatusMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		newReq, err := CheckServiceStatusRequest(r, m.check, m.noVerifyPath)
		if err != nil {
			result.HttpErrorResult(r.Context(), w, err)
			return
		}
		next(w, newReq)
	}
}

func CheckServiceStatusRequest(r *http.Request, checkServiceStatus CheckServiceStatusFunc, noVerifyPath map[string]int) (*http.Request, error) {
	if isWhitelistedPath(r.URL.Path, noVerifyPath) {
		return r, nil
	}
	if checkServiceStatus == nil || !checkServiceStatus(r, headInfo.GetJwtToken(r.Context())) {
		return r, xcode.StatusServiceUnavailable
	}
	return r, nil
}
