package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dmundt/go-mvu/validation"
)

var csrfInput = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

func TestContactFormValidationAndSubmit(t *testing.T) {
	handler, err := newHandler([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	page := getPage(t, client, server.URL)
	token := csrfInput.FindStringSubmatch(page)
	if token == nil {
		t.Fatal("page missing CSRF input")
	}
	for _, want := range []string{
		`<form method="post" action="/contact" class="contact-form" hx-post="/contact"`,
		`hx-target="#contact-form" hx-swap="outerHTML"`,
		`name="name" type="text"`,
		`name="email" type="email"`,
		"<select",
		"<textarea",
		`type="checkbox"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %q", want)
		}
	}

	invalid := url.Values{
		"_csrf":   {token[1]},
		"name":    {"A<script>"},
		"email":   {"not-an-email"},
		"company": {"Example"},
		"topic":   {"unknown"},
		"message": {"Too short"},
	}
	response := postForm(t, client, server.URL, invalid, false)
	for _, want := range []string{
		"Enter a valid email address.",
		"Choose a topic.",
		"Enter at least 20 characters.",
		"Confirm we may reply to your request.",
		`value="A&lt;script&gt;"`,
		`value="Example"`,
		"\nToo short</textarea>",
	} {
		if !strings.Contains(response, want) {
			t.Errorf("invalid form response missing %q", want)
		}
	}
	if strings.Contains(response, `value="A<script>"`) {
		t.Fatal("unescaped form value in response")
	}

	valid := url.Values{
		"_csrf":     {token[1]},
		"name":      {"Ada Lovelace"},
		"email":     {"ada@example.test"},
		"company":   {"Analytical Engines"},
		"topic":     {"support"},
		"message":   {"Please help me with a question."},
		"agreement": {"yes"},
	}
	response = postForm(t, client, server.URL, valid, false)
	if !strings.Contains(response, "Request received") ||
		strings.Contains(response, `name="email"`) {
		t.Fatalf("valid form response = %s", response)
	}
}

func TestContactValidationSwapsOnlyFormForHTMX(t *testing.T) {
	handler, err := newHandler([]byte(strings.Repeat("h", 32)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	page := getPage(t, client, server.URL)
	token := csrfInput.FindStringSubmatch(page)
	if token == nil {
		t.Fatal("page missing CSRF input")
	}

	values := url.Values{
		"_csrf":   {token[1]},
		"name":    {"Ada"},
		"email":   {"ada@example"},
		"topic":   {"sales"},
		"message": {"Not long enough"},
	}
	response := postForm(t, client, server.URL, values, true)
	if !strings.HasPrefix(response, `<section id="contact-form">`) ||
		strings.Contains(response, "<!doctype") {
		t.Fatalf("HTMX response is not form fragment: %s", response)
	}
	for _, want := range []string{
		"Enter a valid email address.",
		"Enter at least 20 characters.",
		`aria-invalid="true"`,
	} {
		if !strings.Contains(response, want) {
			t.Errorf("fragment missing %q", want)
		}
	}
}

func TestContactEmailValidation(t *testing.T) {
	for email, valid := range map[string]bool{
		"ada@example.test":       true,
		"ada@example":            false,
		"ada@example.":           false,
		"not-an-email":           false,
		"Ada <ada@example.test>": false,
	} {
		if got := validEmail(email); got != valid {
			t.Errorf("validEmail(%q) = %v, want %v", email, got, valid)
		}
	}
}

func TestContactChoicesAndLengthBoundaries(t *testing.T) {
	valid := ContactDetails{
		Name: "Ada", Email: "ada@example.test", Topic: "support",
		Message: strings.Repeat("x", messageMinLength), Agreement: true,
	}
	for _, option := range contactTopics() {
		details := valid
		details.Topic = option.Value
		errs := validation.Errors{}
		validateContact(details, errs)
		if errs.Has(topicField) {
			t.Errorf("rendered topic %q rejected: %v", option.Value, errs)
		}
	}
	for _, topic := range []string{"", "unknown"} {
		if validContactTopic(topic) {
			t.Errorf("invalid topic %q accepted", topic)
		}
		details := valid
		details.Topic = topic
		errs := validation.Errors{}
		validateContact(details, errs)
		if !errs.Has(topicField) {
			t.Errorf("invalid topic %q missing field error", topic)
		}
	}
	for _, length := range []int{
		messageMinLength - 1, messageMinLength, messageMaxLength, messageMaxLength + 1,
	} {
		details := valid
		details.Message = strings.Repeat("界", length)
		errs := validation.Errors{}
		validateContact(details, errs)
		wantError := length < messageMinLength || length > messageMaxLength
		if errs.Has(messageField) != wantError || utf8.RuneCountInString(details.Message) != length {
			t.Errorf("message length %d: errors = %v", length, errs)
		}
	}
	for _, length := range []int{nameMinLength - 1, nameMinLength} {
		details := valid
		details.Name = strings.Repeat("界", length)
		errs := validation.Errors{}
		validateContact(details, errs)
		if errs.Has(nameField) != (length < nameMinLength) {
			t.Errorf("name length %d: errors = %v", length, errs)
		}
	}
}

func getPage(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	response, err := client.Get(baseURL + "/contact")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d: %s", response.StatusCode, body)
	}
	return string(body)
}

func postForm(
	t *testing.T,
	client *http.Client,
	baseURL string,
	values url.Values,
	htmxRequest bool,
) string {
	t.Helper()
	request, err := http.NewRequest(
		http.MethodPost, baseURL+"/contact", strings.NewReader(values.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if htmxRequest {
		request.Header.Set("HX-Request", "true")
		request.Header.Set("X-CSRF-Token", values.Get("_csrf"))
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST status = %d: %s", response.StatusCode, body)
	}
	return string(body)
}
