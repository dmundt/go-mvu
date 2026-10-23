package shared

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
}

func (avatar Avatar) RenderHTML(_ context.Context, out *html.Context) error {
	size := avatar.Size
	if size == "" {
		size = AvatarMedium
	}
	if size != AvatarSmall && size != AvatarMedium && size != AvatarLarge {
		return fmt.Errorf("shared: invalid avatar size %q", size)
	}
	if strings.TrimSpace(avatar.Name) == "" {
		return errors.New("shared: avatar requires name")
	}
	if avatar.Src != "" {
		if err := validURL(avatar.Src); err != nil {
			return err
		}
		out.Static(`<img class="avatar avatar--`)
		out.Static(string(size))
		out.Static(`"`)
		attr(out, "src", avatar.Src)
		attr(out, "alt", avatar.Name)
		out.Static(">")
		return out.Err()
	}
	out.Static(`<span class="avatar avatar--`)
	out.Static(string(size))
	out.Static(`" role="img"`)
	attr(out, "aria-label", avatar.Name)
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
}

func (progress Progress) RenderHTML(_ context.Context, out *html.Context) error {
	if err := validID(progress.ID); err != nil {
		return err
	}
	if progress.Label == "" {
		return errors.New("shared: progress requires label")
	}
	if progress.Max < 0 || progress.Max > 0 && (progress.Value < 0 || progress.Value > progress.Max) {
		return fmt.Errorf("shared: progress value %v outside 0..%v", progress.Value, progress.Max)
	}
	out.Static(`<div class="progress-field"><div class="progress-field__header"><label`)
	attr(out, "for", progress.ID)
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
	attr(out, "id", progress.ID)
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
	out.Static(`<span class="spinner" role="status"`)
	optionalAttr(out, "id", spinner.ID)
	out.Static(`><span class="spinner__icon" aria-hidden="true"></span><span class="visually-hidden">`)
	out.Text(label)
	out.Static("</span></span>")
	return out.Err()
}

// Skeleton renders decorative placeholder lines while content loads.
type Skeleton struct {
	Lines int
}

func (skeleton Skeleton) RenderHTML(_ context.Context, out *html.Context) error {
	lines := skeleton.Lines
	if lines == 0 {
		lines = 3
	}
	if lines < 1 || lines > 12 {
		return fmt.Errorf("shared: skeleton lines %d outside 1..12", skeleton.Lines)
	}
	out.Static(`<div class="skeleton" aria-hidden="true">`)
	for range lines {
		out.Static(`<span class="skeleton__line"></span>`)
	}
	out.Static("</div>")
	return out.Err()
}

// Separator renders a horizontal rule, optionally with a centered label.
type Separator struct {
	Label string
}

func (separator Separator) RenderHTML(_ context.Context, out *html.Context) error {
	if separator.Label == "" {
		out.Static(`<hr class="separator">`)
		return out.Err()
	}
	out.Static(`<div class="separator separator--labelled" role="separator"`)
	attr(out, "aria-label", separator.Label)
	out.Static(`><span aria-hidden="true">`)
	out.Text(separator.Label)
	out.Static("</span></div>")
	return out.Err()
}

// Kbd renders a keyboard key or key combination.
type Kbd struct {
	Keys []string
}

func (kbd Kbd) RenderHTML(_ context.Context, out *html.Context) error {
	if len(kbd.Keys) == 0 {
		return errors.New("shared: kbd requires keys")
	}
	for _, key := range kbd.Keys {
		if key == "" {
			return errors.New("shared: kbd key must not be empty")
		}
	}
	out.Static(`<span class="kbd-combo">`)
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
}

func (stat Stat) RenderHTML(_ context.Context, out *html.Context) error {
	trend := stat.Trend
	if trend == "" {
		trend = TrendNeutral
	}
	if trend != TrendNeutral && trend != TrendUp && trend != TrendDown {
		return fmt.Errorf("shared: invalid trend %q", stat.Trend)
	}
	if stat.Label == "" || stat.Value == "" {
		return errors.New("shared: stat requires label and value")
	}
	out.Static(`<dl class="stat"><dt class="stat__label">`)
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
}

func (steps Steps) RenderHTML(_ context.Context, out *html.Context) error {
	if steps.Label == "" || len(steps.Items) == 0 {
		return errors.New("shared: steps require label and items")
	}
	if steps.Current < 0 || steps.Current >= len(steps.Items) {
		return fmt.Errorf("shared: current step %d outside 0..%d", steps.Current, len(steps.Items)-1)
	}
	out.Static(`<ol class="steps"`)
	attr(out, "aria-label", steps.Label)
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
