package appMiddleware

import (
	"github.com/K7mQ2vR9/base-pkg/result"
	"net/http"
)

type AuthMiddlewareOption func(m *UserAuthMiddlewareWithErr)

func WithCheckOptionWithErr(check CheckRequestTokenFuncWithErr) AuthMiddlewareOption {
	return func(m *UserAuthMiddlewareWithErr) {
		m.check = check
	}
}

type UserAuthMiddlewareWithErr struct {
	check        CheckRequestTokenFuncWithErr
	noVerifyPath map[string]int
}

func NewUserAuthMiddlewareWithErr(noVerifyPath map[string]int, ops ...AuthMiddlewareOption) *UserAuthMiddlewareWithErr {
	res := &UserAuthMiddlewareWithErr{
		noVerifyPath: noVerifyPath,
	}
	for _, op := range ops {
		op(res)
	}
	return res
}

func (m *UserAuthMiddlewareWithErr) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		newReq, err := MustAuthTokenRequestWithErr(r, m.check, m.noVerifyPath)
		if err != nil {
			result.HttpErrorResult(r.Context(), w, err)
			return
		}
		next(w, newReq)
	}
}
