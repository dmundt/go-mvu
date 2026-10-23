package web

import (
	"errors"
	"net/http"
)

var ErrMissingDecoder = errors.New("web: form decoder is required")

// FieldErrors holds explicit validation messages by control name.
type FieldErrors map[string][]string

func (e FieldErrors) Add(field, message string) {
	e[field] = append(e[field], message)
}

func (e FieldErrors) Has(field string) bool { return len(e[field]) != 0 }

func (e FieldErrors) First(field string) string {
	if len(e[field]) == 0 {
		return ""
	}
	return e[field][0]
}

func (e FieldErrors) Empty() bool {
	for _, messages := range e {
		if len(messages) != 0 {
			return false
		}
	}
	return true
}

type Form[T any] struct {
	Value  T
	Errors FieldErrors
	Dirty  bool
	Valid  bool
}

// FormDecoder explicitly decodes and validates request input. FieldErrors holds
// field-level validation failures; error reports malformed requests.
type FormDecoder[T any] func(*http.Request) (T, FieldErrors, error)

// Bind decodes r into a Form. Valid is true when no field errors exist.
func Bind[T any](r *http.Request, decoder FormDecoder[T]) (Form[T], error) {
	if decoder == nil {
		return Form[T]{}, ErrMissingDecoder
	}
	value, errs, err := decoder(r)
	if err != nil {
		return Form[T]{}, err
	}
	if errs == nil {
		errs = FieldErrors{}
	}
	return Form[T]{Value: value, Errors: errs, Dirty: true, Valid: errs.Empty()}, nil
}
