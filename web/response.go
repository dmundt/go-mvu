package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/dmundt/go-mvu/core"
	"github.com/dmundt/go-mvu/html"
)

type RenderMode uint8

const (
	// RenderPage renders the full document.
	RenderPage RenderMode = iota + 1
	// RenderFragment renders Result.Fragment.
	RenderFragment
	// RenderNothing writes 204.
	RenderNothing
	// RenderAuto: HTMX (non-boosted, non-history) → fragment;
	// else Location set → redirect; else full page.
	RenderAuto
)

type Result struct {
	Mode       RenderMode
	Fragment   string
	Status     int
	Location   string
	PushURL    string
	ReplaceURL string
	// CacheControl overrides the default no-store policy for this result.
	// Set explicitly only when response content is safe to cache.
	CacheControl string
}

func (result Result) StatusOr(fallback int) int {
	if result.Status == 0 {
		return fallback
	}
	return result.Status
}

func (runtime *Runtime[K, Model, Msg]) writeResult(
	w http.ResponseWriter, r *http.Request, model Model, result Result,
) error {
	tracked, ok := w.(*ResponseWriter)
	if !ok {
		tracked = &ResponseWriter{ResponseWriter: w}
		w = tracked
	}
	request := NewRequestContext(r, runtime.theme)
	mode := result.Mode
	cacheControl := result.CacheControl
	if cacheControl == "" {
		cacheControl = "no-store"
	}
	w.Header().Set("Cache-Control", cacheControl)
	if mode == RenderAuto {
		Vary(w.Header(), "HX-Request", "HX-Boosted", "HX-History-Restore-Request")
		switch {
		case request.IsHTMX && !request.HTMX.IsBoosted && !request.HTMX.IsHistory:
			mode = RenderFragment
		case result.Location != "":
			http.Redirect(w, r, result.Location, result.StatusOr(http.StatusSeeOther))
			return nil
		default:
			mode = RenderPage
		}
	} else if result.Location != "" {
		http.Redirect(w, r, result.Location, result.StatusOr(http.StatusSeeOther))
		return nil
	}
	if err := r.Context().Err(); err != nil {
		runtime.fail(w, r, err)
		return err
	}
	if result.PushURL != "" {
		HTMXPushURL(w, result.PushURL)
	}
	if result.ReplaceURL != "" {
		HTMXReplaceURL(w, result.ReplaceURL)
	}
	trace := TraceFromContext(r.Context())
	var err error
	switch mode {
	case RenderNothing:
		NoContent(w)
		return nil
	case RenderPage:
		var page html.Page
		done := trace.Start(PhaseView)
		page, err = runtime.view.Page(r.Context(), model, request)
		done()
		if err == nil {
			err = r.Context().Err()
		}
		if err == nil {
			done = trace.Start(PhaseRender)
			err = WritePage(r.Context(), tracked, result.StatusOr(http.StatusOK), page)
			done()
		}
	case RenderFragment:
		var fragment html.Fragment
		done := trace.Start(PhaseView)
		fragment, err = runtime.view.Fragment(r.Context(), result.Fragment, model, request)
		done()
		if err == nil {
			err = r.Context().Err()
		}
		if err == nil {
			done = trace.Start(PhaseRender)
			err = WriteFragment(r.Context(), tracked, result.StatusOr(http.StatusOK), fragment)
			done()
		}
	default:
		err = fmt.Errorf("%w: %d", ErrUnknownRender, mode)
	}
	if err == nil {
		return nil
	}
	if tracked.Started {
		if errors.Is(err, http.ErrAbortHandler) {
			panic(http.ErrAbortHandler)
		}
		runtime.logger.ErrorContext(r.Context(), "render failed after response started",
			"path", r.URL.Path, "error", err)
		return &responseAbortError{cause: err}
	}
	runtime.fail(w, r, err)
	return err
}

func (runtime *Runtime[K, Model, Msg]) fail(w http.ResponseWriter, r *http.Request, err error) {
	err = normalizeError(err)
	runtime.onError(w, r, err)
}

type responseAbortError struct{ cause error }

func (e *responseAbortError) Error() string { return "response aborted: " + e.cause.Error() }

func (e *responseAbortError) Unwrap() error { return e.cause }

func isEffectError(err error) bool {
	var effect *EffectError
	return errors.As(err, &effect)
}

func isResponseAbort(err error) bool {
	var aborted *responseAbortError
	return errors.As(err, &aborted)
}

func staleVersionError() error {
	return &Error{
		Status: http.StatusConflict, Public: http.StatusText(http.StatusConflict),
		Cause: core.ErrConflict,
	}
}

func normalizeError(err error) error {
	if isEffectError(err) {
		return &Error{
			Status: http.StatusInternalServerError, Public: http.StatusText(http.StatusInternalServerError),
			Cause: err,
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return &Error{
			Status: http.StatusRequestEntityTooLarge, Public: http.StatusText(http.StatusRequestEntityTooLarge),
			Cause: err,
		}
	}
	return err
}

// WritePage renders into a buffer-free stream. Headers are sent before the
// body, so a mid-render failure yields a truncated 200; callers log it.
func WritePage(ctx context.Context, w http.ResponseWriter, status int, page html.Page) error {
	if page == nil {
		return errors.New("nil HTML page")
	}
	return write(ctx, w, status, page)
}

func WriteFragment(ctx context.Context, w http.ResponseWriter, status int, fragment html.Fragment) error {
	if fragment == nil {
		return errors.New("nil HTML fragment")
	}
	return write(ctx, w, status, fragment)
}

func write(ctx context.Context, w http.ResponseWriter, status int, component html.Component) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return html.Render(ctx, w, component)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
