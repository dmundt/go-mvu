package validation

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type signup struct{ Email string }

var signupDecoder = func(r *http.Request) (signup, Errors, error) {
	if err := r.ParseForm(); err != nil {
		return signup{}, nil, err
	}
	value := signup{Email: strings.TrimSpace(r.PostForm.Get("email"))}
	errs := Errors{}
	if !strings.Contains(value.Email, "@") {
		errs.Add("email", "Enter an email address.")
	}
	return value, errs, nil
}

func post(body url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestBindReportsFieldErrors(t *testing.T) {
	form, err := Bind(post(url.Values{"email": {"nope"}}), signupDecoder)
	if err != nil {
		t.Fatal(err)
	}
	if form.Valid || !form.Dirty || form.Errors.First("email") != "Enter an email address." {
		t.Fatalf("form = %+v", form)
	}
	form, err = Bind(post(url.Values{"email": {" a@b.c "}}), signupDecoder)
	if err != nil || !form.Valid || form.Value.Email != "a@b.c" || !form.Errors.Empty() {
		t.Fatalf("form = %+v, err %v", form, err)
	}
}

func TestBindPropagatesDecodeError(t *testing.T) {
	failure := errors.New("bad")
	_, err := Bind(httptest.NewRequest(http.MethodGet, "/", nil),
		func(*http.Request) (int, Errors, error) { return 0, nil, failure })
	if !errors.Is(err, failure) {
		t.Fatalf("err = %v", err)
	}
}

func TestBindRejectsMissingDecoder(t *testing.T) {
	form, err := Bind[int](httptest.NewRequest(http.MethodGet, "/", nil), nil)
	if !errors.Is(err, ErrMissingDecoder) || form.Valid || form.Dirty {
		t.Fatalf("form = %+v, err = %v", form, err)
	}
}

func TestErrors(t *testing.T) {
	errs := Errors{}
	if !errs.Empty() || errs.Has("x") || errs.First("x") != "" {
		t.Fatal("empty errors misreport")
	}
	errs.Add("x", "one")
	errs.Add("x", "two")
	if errs.Empty() || !errs.Has("x") || errs.First("x") != "one" {
		t.Fatalf("errors = %v", errs)
	}
}
