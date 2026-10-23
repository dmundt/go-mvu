// Package validation provides explicit, reflection-free form validation.
package validation

import (
	"errors"
	"net/http"
)

var ErrMissingDecoder = errors.New("validation: decoder is required")

type Errors map[string][]string

func (e Errors) Add(field, message string) {
	e[field] = append(e[field], message)
}

func (e Errors) Has(field string) bool { return len(e[field]) != 0 }

func (e Errors) First(field string) string {
	if len(e[field]) == 0 {
		return ""
	}
	return e[field][0]
}

func (e Errors) Empty() bool {
	for _, messages := range e {
		if len(messages) != 0 {
			return false
		}
	}
	return true
}

type Form[T any] struct {
	Value  T
	Errors Errors
	Dirty  bool
	Valid  bool
}

// Decoder explicitly decodes and validates request input. Errors holds
// field-level validation failures; error reports malformed requests.
type Decoder[T any] func(*http.Request) (T, Errors, error)

// Bind decodes r into a Form. Valid is true when no field errors exist.
func Bind[T any](r *http.Request, decoder Decoder[T]) (Form[T], error) {
	if decoder == nil {
		return Form[T]{}, ErrMissingDecoder
	}
	value, errs, err := decoder(r)
	if err != nil {
		return Form[T]{}, err
	}
	if errs == nil {
		errs = Errors{}
	}
	return Form[T]{Value: value, Errors: errs, Dirty: true, Valid: errs.Empty()}, nil
}
