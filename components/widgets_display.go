package components

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/dmundt/go-mvu/html"
)

// AvatarSize selects an avatar's dimensions.
type AvatarSize string

const (
	AvatarSmall  AvatarSize = "sm"
	AvatarMedium AvatarSize = "md"
	AvatarLarge  AvatarSize = "lg"
)

// Avatar renders an image or, without Src, the name's initials.
type Avatar struct {
	Name string
	Src  string
	Size AvatarSize
	// Attrs adds passthrough attributes to the root element.
	Attrs html.Attrs
}

func (avatar Avatar) RenderHTML(_ context.Context, out *html.Context) error {
	size := avatar.Size
	if size == "" {
		size = AvatarMedium
	}
	if size != AvatarSmall && size != AvatarMedium && size != AvatarLarge {
		return fmt.Errorf("components: invalid avatar size %q", size)
	}
	if strings.TrimSpace(avatar.Name) == "" {
		return errors.New("components: avatar requires name")
	}
	if avatar.Src != "" {
		if err := validURL(avatar.Src); err != nil {
			return err
		}
		out.Static(`<img class="avatar avatar--`)
		out.Static(string(size))
		out.ExtraClass(avatar.Attrs)
		out.Static(`"`)
		attribute(out, "src", avatar.Src)
		attribute(out, "alt", avatar.Name)
		out.PassAttrs(avatar.Attrs, "src", "alt")
		out.Static(">")
		return out.Err()
	}
	out.Static(`<span class="avatar avatar--`)
	out.Static(string(size))
	out.ExtraClass(avatar.Attrs)
	out.Static(`" role="img"`)
	attribute(out, "aria-label", avatar.Name)
	out.PassAttrs(avatar.Attrs, "role", "aria-label")
	out.Static(`><span aria-hidden="true">`)
	out.Text(initials(avatar.Name))
	out.Static("</span></span>")
	return out.Err()
}

func initials(name string) string {
	var result []rune
	for _, word := range strings.Fields(name) {
		result = append(result, unicode.ToUpper([]rune(word)[0]))
		if len(result) == 2 {
			break
		}
	}
	return string(result)
}

// Progress renders a labelled native progress bar. Max zero renders an
// indeterminate bar.
type Progress struct {
	ID    string
	Label string
	Value float64
	Max   float64
	// Attrs apply to the div.progress-field wrapper (e.g. hx-* polling).
	Attrs html.Attrs
}

func (progress Progress) RenderHTML(_ context.Context, out *html.Context) error {
	if err := validID(progress.ID); err != nil {
		return err
	}
	if progress.Label == "" {
		return errors.New("components: progress requires label")
	}
	if progress.Max < 0 || progress.Max > 0 && (progress.Value < 0 || progress.Value > progress.Max) {
		return fmt.Errorf("components: progress value %v outside 0..%v", progress.Value, progress.Max)
	}
	out.Static(`<div class="progress-field`)
	out.ExtraClass(progress.Attrs)
	out.Static(`"`)
	out.PassAttrs(progress.Attrs)
	out.Static(`><div class="progress-field__header"><label`)
	attribute(out, "for", progress.ID)
	out.Static(">")
	out.Text(progress.Label)
	out.Static("</label>")
	percent := 0
	if progress.Max > 0 {
		percent = int(math.Round(progress.Value / progress.Max * 100))
		out.Static(`<span class="progress-field__value" aria-hidden="true">`)
		out.Int(percent)
		out.Static("%</span>")
	}
	out.Static("</div><progress")
	attribute(out, "id", progress.ID)
	if progress.Max > 0 {
		out.Static(` max="`)
		out.Float64(progress.Max)
		out.Static(`" value="`)
		out.Float64(progress.Value)
		out.Static(`">`)
		out.Int(percent)
		out.Static("%")
	} else {
		out.Static(">")
	}
	out.Static("</progress></div>")
	return out.Err()
}

// Spinner renders an animated loading status. Its ID can be used as an HTMX
// indicator.
type Spinner struct {
	ID    string
	Label string
	Attrs html.Attrs
}

func (spinner Spinner) RenderHTML(_ context.Context, out *html.Context) error {
	if spinner.ID != "" {
		if err := validID(spinner.ID); err != nil {
			return err
		}
	}
	label := spinner.Label
	if label == "" {
		label = "Loading"
	}
	out.Static(`<span class="spinner`)
	out.ExtraClass(spinner.Attrs)
	out.Static(`" role="status"`)
	optionalAttribute(out, "id", spinner.ID)
	out.PassAttrs(spinner.Attrs, "role", when(spinner.ID, "id"))
	out.Static(`><span class="spinner__icon" aria-hidden="true"></span><span class="visually-hidden">`)
	out.Text(label)
	out.Static("</span></span>")
	return out.Err()
}

// Skeleton renders decorative placeholder lines while content loads.
type Skeleton struct {
	Lines int
	Attrs html.Attrs
}

func (skeleton Skeleton) RenderHTML(_ context.Context, out *html.Context) error {
	lines := skeleton.Lines
	if lines == 0 {
		lines = 3
	}
	if lines < 1 || lines > 12 {
		return fmt.Errorf("components: skeleton lines %d outside 1..12", skeleton.Lines)
	}
	out.Static(`<div class="skeleton`)
	out.ExtraClass(skeleton.Attrs)
	out.Static(`" aria-hidden="true"`)
	out.PassAttrs(skeleton.Attrs, "aria-hidden")
	out.Static(">")
	for range lines {
		out.Static(`<span class="skeleton__line"></span>`)
	}
	out.Static("</div>")
	return out.Err()
}

// Separator renders a horizontal rule, optionally with a centered label.
type Separator struct {
	Label string
	Attrs html.Attrs
}

func (separator Separator) RenderHTML(_ context.Context, out *html.Context) error {
	if separator.Label == "" {
		out.Static(`<hr class="separator`)
		out.ExtraClass(separator.Attrs)
		out.Static(`"`)
		out.PassAttrs(separator.Attrs)
		out.Static(">")
		return out.Err()
	}
	out.Static(`<div class="separator separator--labelled`)
	out.ExtraClass(separator.Attrs)
	out.Static(`" role="separator"`)
	attribute(out, "aria-label", separator.Label)
	out.PassAttrs(separator.Attrs, "role", "aria-label")
	out.Static(`><span aria-hidden="true">`)
	out.Text(separator.Label)
	out.Static("</span></div>")
	return out.Err()
}

// Kbd renders a keyboard key or key combination.
type Kbd struct {
	Keys  []string
	Attrs html.Attrs
}

func (kbd Kbd) RenderHTML(_ context.Context, out *html.Context) error {
	if len(kbd.Keys) == 0 {
		return errors.New("components: kbd requires keys")
	}
	for _, key := range kbd.Keys {
		if key == "" {
			return errors.New("components: kbd key must not be empty")
		}
	}
	out.Static(`<span class="kbd-combo`)
	out.ExtraClass(kbd.Attrs)
	out.Static(`"`)
	out.PassAttrs(kbd.Attrs)
	out.Static(">")
	for index, key := range kbd.Keys {
		if index > 0 {
			out.Static(`<span aria-hidden="true">+</span>`)
		}
		out.Static(`<kbd class="kbd">`)
		out.Text(key)
		out.Static("</kbd>")
	}
	out.Static("</span>")
	return out.Err()
}

// Trend describes the direction of a Stat change.
type Trend string

const (
	TrendNeutral Trend = "neutral"
	TrendUp      Trend = "up"
	TrendDown    Trend = "down"
)

// Stat renders a labelled metric with an optional change indicator.
type Stat struct {
	Label string
	Value string
	Delta string
	Trend Trend
	Attrs html.Attrs
}

func (stat Stat) RenderHTML(_ context.Context, out *html.Context) error {
	trend := stat.Trend
	if trend == "" {
		trend = TrendNeutral
	}
	if trend != TrendNeutral && trend != TrendUp && trend != TrendDown {
		return fmt.Errorf("components: invalid trend %q", stat.Trend)
	}
	if stat.Label == "" || stat.Value == "" {
		return errors.New("components: stat requires label and value")
	}
	out.Static(`<dl class="stat`)
	out.ExtraClass(stat.Attrs)
	out.Static(`"`)
	out.PassAttrs(stat.Attrs)
	out.Static(`><dt class="stat__label">`)
	out.Text(stat.Label)
	out.Static(`</dt><dd class="stat__value">`)
	out.Text(stat.Value)
	out.Static("</dd>")
	if stat.Delta != "" {
		out.Static(`<dd class="stat__delta stat__delta--`)
		out.Static(string(trend))
		out.Static(`">`)
		out.Text(stat.Delta)
		out.Static("</dd>")
	}
	out.Static("</dl>")
	return out.Err()
}

// Steps renders progress through an ordered process. Current is zero-based.
type Steps struct {
	Label   string
	Items   []string
	Current int
	Attrs   html.Attrs
}

func (steps Steps) RenderHTML(_ context.Context, out *html.Context) error {
	if steps.Label == "" || len(steps.Items) == 0 {
		return errors.New("components: steps require label and items")
	}
	if steps.Current < 0 || steps.Current >= len(steps.Items) {
		return fmt.Errorf("components: current step %d outside 0..%d", steps.Current, len(steps.Items)-1)
	}
	out.Static(`<ol class="steps`)
	out.ExtraClass(steps.Attrs)
	out.Static(`"`)
	attribute(out, "aria-label", steps.Label)
	out.PassAttrs(steps.Attrs, "aria-label")
	out.Static(">")
	for index, item := range steps.Items {
		switch {
		case index < steps.Current:
			out.Static(`<li class="steps__item steps__item--complete"><span class="visually-hidden">Completed: </span>`)
		case index == steps.Current:
			out.Static(`<li class="steps__item steps__item--current" aria-current="step">`)
		default:
			out.Static(`<li class="steps__item">`)
		}
		out.Text(item)
		out.Static("</li>")
	}
	out.Static("</ol>")
	return out.Err()
}
