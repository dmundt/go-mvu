package middleware

import (
	"net/http"

	"github.com/dmundt/go-mvu/httpx"
)

// Extension is an optional, explicitly configured HTTP middleware.
type Extension func(http.Handler) http.Handler

func extensionError(handler httpx.ErrorHandler) httpx.ErrorHandler {
	if handler == nil {
		return httpx.RespondError
	}
	return handler
}

func rejectExtension(handler httpx.ErrorHandler, w http.ResponseWriter, r *http.Request, status int) {
	handler(w, r, &httpx.Error{Status: status})
}
