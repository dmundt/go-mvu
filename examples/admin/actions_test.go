package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/httpx"
)

func TestSaveDecoderRetainsInvalidValuesAndRejectsMalformedIDs(t *testing.T) {
	for _, test := range []struct {
		name  string
		role  string
		valid bool
	}{
		{"A", "Admin", false},
		{strings.Repeat("x", 81), "Viewer", false},
		{"Valid", "Owner", false},
		{"Valid", "Editor", true},
	} {
		values := url.Values{"id": {"1"}, "name": {test.name}, "role": {test.role}}
		request := httptest.NewRequest(http.MethodPost, "/admin/save", strings.NewReader(values.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		message, err := decodeSave(request)
		if err != nil {
			t.Fatal(err)
		}
		form := message.(Save).Form
		if form.Valid != test.valid || form.Value.Name != test.name || form.Value.Role != test.role {
			t.Fatalf("form = %+v", form)
		}
	}
	for _, id := range []string{"", "bad", "-1", "99"} {
		values := url.Values{"id": {id}}
		request := httptest.NewRequest(http.MethodPost, "/admin/save", strings.NewReader(values.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_, err := decodeSave(request)
		var rejection *httpx.Error
		if !errors.As(err, &rejection) || rejection.Status != http.StatusBadRequest {
			t.Fatalf("id=%q: error = %v", id, err)
		}
	}
}

func TestSaveNameLimitsCountRunesAndShareErrorMessage(t *testing.T) {
	for _, length := range []int{nameMinLength - 1, nameMinLength, nameMaxLength, nameMaxLength + 1} {
		name := strings.Repeat("\u00e9", length)
		values := url.Values{"id": {"1"}, "name": {name}, "role": {roleAdmin}}
		request := httptest.NewRequest(http.MethodPost, "/admin/save", strings.NewReader(values.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		message, err := decodeSave(request)
		if err != nil {
			t.Fatal(err)
		}
		form := message.(Save).Form
		valid := length >= nameMinLength && length <= nameMaxLength
		if form.Valid != valid || form.Value.Name != name {
			t.Fatalf("length=%d: form = %+v", length, form)
		}
		if !valid {
			want := fmt.Sprintf("Use %d to %d characters.", nameMinLength, nameMaxLength)
			if !strings.Contains(fmt.Sprint(form.Errors), want) {
				t.Fatalf("length=%d: errors = %v", length, form.Errors)
			}
		}
	}
}
