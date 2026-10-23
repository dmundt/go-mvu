// Package validation provides explicit, reflection-free form validation.
package validation

import "net/http"

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
type Decoder[T any] interface {
	Decode(*http.Request) (T, Errors, error)
}

// DecoderFunc adapts a function to Decoder.
type DecoderFunc[T any] func(*http.Request) (T, Errors, error)

func (fn DecoderFunc[T]) Decode(r *http.Request) (T, Errors, error) { return fn(r) }

// Bind decodes r into a Form. Valid is true when no field errors exist.
func Bind[T any](r *http.Request, decoder Decoder[T]) (Form[T], error) {
	value, errs, err := decoder.Decode(r)
	if err != nil {
		return Form[T]{}, err
	}
	if errs == nil {
		errs = Errors{}
	}
	return Form[T]{Value: value, Errors: errs, Dirty: true, Valid: errs.Empty()}, nil
}
