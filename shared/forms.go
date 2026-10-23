package shared

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
)

// Switch renders an on/off checkbox exposed to assistive technology as a
// switch.
type Switch struct {
	ID       string
	Name     string
	Label    string
	Value    string
	Checked  bool
	Disabled bool
	// Attrs apply to the <input> element.
	Attrs html.Attrs
}

func (toggle Switch) RenderHTML(_ context.Context, out *html.Context) error {
	id, err := controlID(toggle.ID, toggle.Name, toggle.Label)
	if err != nil {
		return err
	}
	out.Static(`<label class="switch"><input class="switch__input`)
	out.ExtraClass(toggle.Attrs)
	out.Static(`" type="checkbox" role="switch"`)
	writeChoice(out, id, toggle.Name, toggle.Value, toggle.Checked, toggle.Disabled)
	out.PassAttrs(toggle.Attrs, "type", "role", "id", "name", "value",
		whenSet(toggle.Checked, "checked"), whenSet(toggle.Disabled, "disabled"))
	out.Static(`><span>`)
	out.Text(toggle.Label)
	out.Static("</span></label>")
	return out.Err()
}

// RadioGroup renders a fieldset of mutually exclusive options.
type RadioGroup struct {
	Name     string
	Legend   string
	Options  []components.Option
	Value    string
	Required bool
	Disabled bool
	Error    string
	// Attrs apply to the <fieldset> element.
	Attrs html.Attrs
}

func (group RadioGroup) RenderHTML(_ context.Context, out *html.Context) error {
	if err := validID(group.Name); err != nil {
		return err
	}
	if group.Legend == "" || len(group.Options) == 0 {
		return errors.New("shared: radio group requires legend and options")
	}
	for index, option := range group.Options {
		if option.Label == "" {
			return fmt.Errorf("shared: radio option %d requires label", index)
		}
	}
	errorID := group.Name + "-error"
	out.Static(`<fieldset class="radio-group`)
	out.ExtraClass(group.Attrs)
	out.Static(`"`)
	if group.Error != "" {
		attr(out, "aria-describedby", errorID)
	}
	flag(out, "disabled", group.Disabled)
	out.PassAttrs(group.Attrs, when(group.Error, "aria-describedby"), whenSet(group.Disabled, "disabled"))
	out.Static("><legend>")
	out.Text(group.Legend)
	out.Static("</legend>")
	for index, option := range group.Options {
		out.Static(`<label class="radio"><input type="radio"`)
		writeChoice(out, group.Name+"-"+strconv.Itoa(index), group.Name, option.Value, option.Value == group.Value, option.Disabled)
		flag(out, "required", group.Required)
		if group.Error != "" {
			out.Static(` aria-invalid="true"`)
		}
		out.Static("><span>")
		out.Text(option.Label)
		out.Static("</span></label>")
	}
	fieldError(out, errorID, group.Error)
	out.Static("</fieldset>")
	return out.Err()
}

func controlID(id, name, label string) (string, error) {
	if id == "" {
		id = name
	}
	if err := validID(id); err != nil {
		return "", err
	}
	if name == "" || label == "" {
		return "", errors.New("shared: form control requires name and label")
	}
	return id, nil
}

func writeChoice(out *html.Context, id, name, value string, checked, disabled bool) {
	if value == "" {
		value = "on"
	}
	attr(out, "id", id)
	attr(out, "name", name)
	attr(out, "value", value)
	flag(out, "checked", checked)
	flag(out, "disabled", disabled)
}
