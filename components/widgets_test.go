package components

import (
	"context"
	"strings"
	"testing"

	"github.com/dmundt/go-mvu/html"
)

func TestComponentsRenderAccessibleMarkup(t *testing.T) {
	cases := []struct {
		name      string
		component html.Component
		expected  []string
	}{
		{"accordion", Accordion{ID: "faq", Exclusive: true, Items: []AccordionItem{{Title: "Q", Body: Text("A"), Open: true}}},
			[]string{`<details class="accordion__item" name="faq" open>`, `<summary class="accordion__trigger">Q</summary>`}},
		{"tabs", Tabs{ID: "t", Label: "Sections", Active: "b", HTMX: true, Items: []Tab{{ID: "a", Label: "A", URL: "?tab=a"}, {ID: "b", Label: "B", URL: "?tab=b"}}},
			[]string{`aria-label="Sections"`, `id="t-tab-b" href="?tab=b" aria-current="page"`, `hx-get="?tab=a"`, `hx-select="#t"`, `aria-labelledby="t-tab-b"`}},
		{"dialog", Dialog{ID: "d", Title: "Title", TriggerLabel: "Open", Body: Text("Body")},
			[]string{`popovertarget="d">Open</button>`, `class="dialog" popover role="dialog" id="d" aria-labelledby="d-title"`, `popovertargetaction="hide"`, `aria-label="Close"`}},
		{"dropdown", Dropdown{Label: "Menu", AlignEnd: true, Items: []MenuItem{{Label: "Mail", URL: "mailto:a@example.test"}}},
			[]string{`<details class="dropdown dropdown--end"><summary>Menu</summary><ul class="dropdown__menu">`, `href="mailto:a@example.test"`}},
		{"tooltip", Tooltip{ID: "tip", Label: "Info", Content: "Details"},
			[]string{`aria-describedby="tip" aria-controls="tip" popovertarget="tip"`,
				`popover="auto" role="tooltip" id="tip">Details`}},
		{"avatar initials", Avatar{Name: "ada lovelace byron"},
			[]string{`class="avatar avatar--md" role="img" aria-label="ada lovelace byron"`, `>AL</span>`}},
		{"avatar image", Avatar{Name: "Ada", Src: "/ada.png", Size: AvatarLarge},
			[]string{`<img class="avatar avatar--lg" src="/ada.png" alt="Ada">`}},
		{"progress", Progress{ID: "p", Label: "Upload", Value: 1, Max: 3},
			[]string{`<label for="p">Upload</label>`, `aria-hidden="true">33%`, `max="3" value="1"`}},
		{"indeterminate progress", Progress{ID: "p", Label: "Sync"}, []string{`<progress id="p"></progress>`}},
		{"spinner", Spinner{}, []string{`role="status"`, `<span class="visually-hidden">Loading</span>`}},
		{"skeleton", Skeleton{Lines: 2}, []string{`aria-hidden="true"><span class="skeleton__line"></span><span class="skeleton__line"></span></div>`}},
		{"separator", Separator{Label: "Or"}, []string{`role="separator" aria-label="Or"`}},
		{"kbd", Kbd{Keys: []string{"Ctrl", "K"}}, []string{`<kbd class="kbd">Ctrl</kbd><span aria-hidden="true">+</span><kbd class="kbd">K</kbd>`}},
		{"empty", EmptyState{Title: "None", Action: Text("Act")}, []string{`empty-state__title">None`, `empty-state__action">Act`}},
		{"stat", Stat{Label: "Users", Value: "5", Delta: "+1", Trend: TrendUp}, []string{`<dt class="stat__label">Users</dt>`, `stat__delta--up">+1`}},
		{"steps", Steps{Label: "Checkout", Items: []string{"A", "B", "C"}, Current: 1},
			[]string{`steps__item--complete"><span class="visually-hidden">Completed: </span>A`, `aria-current="step">B`}},
		{"switch", Switch{Name: "alerts", Label: "Alerts", Checked: true},
			[]string{`type="checkbox" role="switch" id="alerts" name="alerts" value="on" checked`}},
		{"checkbox", Checkbox{Name: "terms", Label: "Terms", Error: "Required"},
			[]string{`aria-invalid="true" aria-describedby="terms-error"`, `id="terms-error">Required`}},
		{"radio", RadioGroup{Name: "plan", Legend: "Plan", Value: "b", Error: "Pick", Options: []Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}},
			[]string{`<fieldset class="radio-group" aria-describedby="plan-error"><legend>Plan</legend>`, `id="plan-1" name="plan" value="b" checked`}},
		{"textarea", Textarea{Name: "note", Label: "Note", Hint: "Short", Error: "Too long", Value: "\nhi"},
			[]string{`aria-invalid="true" aria-describedby="note-hint note-error">` + "\n\nhi</textarea>", `id="note-hint">Short`}},
		{"inline", Inline{Children: []html.Component{Text("x")}}, []string{`<div class="inline">x</div>`}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			rendered := render(t, test.component)
			for _, expected := range test.expected {
				if !strings.Contains(rendered, expected) {
					t.Errorf("missing %q in %s", expected, rendered)
				}
			}
			if strings.Contains(rendered, " style=") || strings.Contains(rendered, "<script") {
				t.Errorf("emitted inline style or script: %s", rendered)
			}
		})
	}
}

func TestTabsUseKeyboardNativeNavigationSemantics(t *testing.T) {
	rendered := render(t, Tabs{
		ID: "settings", Label: "Settings sections", Active: "profile",
		Items: []Tab{
			{ID: "profile", Label: "Profile", URL: "?tab=profile"},
			{ID: "security", Label: "Security", URL: "?tab=security"},
		},
	})
	for _, expected := range []string{
		`<nav class="tabset__list" aria-label="Settings sections">`,
		`href="?tab=profile" aria-current="page"`,
		`href="?tab=security"`,
		`aria-labelledby="settings-tab-profile"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("missing navigation semantics %q in %s", expected, rendered)
		}
	}
	if strings.Contains(rendered, `role="tab"`) || strings.Contains(rendered, `role="tablist"`) {
		t.Fatalf("link navigation must not claim scripted tab semantics: %s", rendered)
	}
}

func TestTooltipUsesNativePopoverDismissal(t *testing.T) {
	rendered := render(t, Tooltip{ID: "help", Label: "Help", Content: "More information"})
	for _, expected := range []string{
		`aria-describedby="help" aria-controls="help" popovertarget="help"`,
		`popover="auto" role="tooltip" id="help">More information</span>`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("missing popover tooltip behavior %q in %s", expected, rendered)
		}
	}
}

func TestComponentsEscapeDynamicContent(t *testing.T) {
	payload := `<img src=x onerror="alert(1)">`
	for _, component := range []html.Component{
		Accordion{Items: []AccordionItem{{Title: payload}}},
		Tooltip{ID: "t", Label: payload, Content: payload},
		Dropdown{Label: payload, Items: []MenuItem{{Label: payload, URL: "/x?a=1&b=2"}}},
		Avatar{Name: payload},
		Stat{Label: payload, Value: payload, Delta: payload},
		Steps{Label: payload, Items: []string{payload}},
		Kbd{Keys: []string{payload}},
		Textarea{Name: "n", Label: payload, Value: "</textarea>" + payload},
	} {
		rendered := render(t, component)
		if strings.Contains(rendered, "<img src=x") || strings.Contains(rendered, "</textarea><img") {
			t.Errorf("%T did not escape content: %s", component, rendered)
		}
	}
}

func TestComponentsRejectInvalidInput(t *testing.T) {
	for name, component := range map[string]html.Component{
		"exclusive accordion without ID":   Accordion{Exclusive: true, Items: []AccordionItem{{Title: "A"}}},
		"exclusive accordion two open":     Accordion{ID: "a", Exclusive: true, Items: []AccordionItem{{Title: "A", Open: true}, {Title: "B", Open: true}}},
		"tabs unknown active":              Tabs{ID: "t", Label: "L", Active: "x", Items: []Tab{{ID: "a", Label: "A", URL: "/"}}},
		"tabs javascript URL":              Tabs{ID: "t", Label: "L", Active: "a", Items: []Tab{{ID: "a", Label: "A", URL: "javascript:alert(1)"}}},
		"tabs duplicate":                   Tabs{ID: "t", Label: "L", Active: "a", Items: []Tab{{ID: "a", Label: "A", URL: "/"}, {ID: "a", Label: "B", URL: "/"}}},
		"dialog selector-unsafe ID":        Dialog{ID: "a b", Title: "T", TriggerLabel: "O"},
		"dialog invalid variant":           Dialog{ID: "d", Title: "T", TriggerLabel: "O", TriggerVariant: "loud"},
		"dropdown javascript URL":          Dropdown{Label: "M", Items: []MenuItem{{Label: "x", URL: "javascript:x"}}},
		"avatar data URL":                  Avatar{Name: "A", Src: "data:image/png;base64,AA"},
		"avatar invalid size":              Avatar{Name: "A", Size: "xl"},
		"progress over max":                Progress{ID: "p", Label: "P", Value: 5, Max: 4},
		"skeleton too many lines":          Skeleton{Lines: 13},
		"stat invalid trend":               Stat{Label: "L", Value: "V", Trend: "sideways"},
		"steps current out of range":       Steps{Label: "L", Items: []string{"A"}, Current: 1},
		"radio option without label":       RadioGroup{Name: "r", Legend: "L", Options: []Option{{Value: "a"}}},
		"textarea without label":           Textarea{Name: "n"},
		"switch with invalid ID":           Switch{ID: "1x", Name: "n", Label: "L"},
		"kbd without keys":                 Kbd{},
		"empty state without title":        EmptyState{},
		"tooltip without content":          Tooltip{ID: "t", Label: "L"},
		"textarea rows over maximum":       Textarea{Name: "n", Label: "L", Rows: 51},
		"tabs without label":               Tabs{ID: "t", Active: "a", Items: []Tab{{ID: "a", Label: "A", URL: "/"}}},
		"dialog without trigger label":     Dialog{ID: "d", Title: "T"},
		"progress with negative maximum":   Progress{ID: "p", Label: "P", Max: -1},
		"radio group with unsafe name":     RadioGroup{Name: "a\"b", Legend: "L", Options: []Option{{Value: "a", Label: "A"}}},
		"checkbox missing name and an ID":  Checkbox{Label: "L"},
		"dropdown without items":           Dropdown{Label: "M"},
		"steps without label":              Steps{Items: []string{"A"}},
		"accordion item without the title": Accordion{Items: []AccordionItem{{}}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := html.Render(context.Background(), &strings.Builder{}, component); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
