package appMiddleware

import (
	"github.com/doubunv/common-pkg/headInfo"
	"github.com/doubunv/common-pkg/result"
	"github.com/doubunv/common-pkg/result/xcode"
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
	ctx := r.Context()
	token := headInfo.GetJwtToken(ctx)

	wPathBool := verifyPath(r.URL.Path, noVerifyPath)
	if !wPathBool {
		blServiceStatus := checkServiceStatus(r, token)
		if !blServiceStatus {
			return r, xcode.StatusServiceUnavailable
		}
	}

	newReq := r.WithContext(ctx)

	return newReq, nil
}
