package components

import "github.com/dmundt/go-mvu/html"

// Column arranges children vertically. Use Stack for custom gaps or attributes.
func Column(children ...html.Component) Stack {
	return Stack{Children: children}
}

// Row arranges children horizontally with wrapping. Use Inline for attributes.
func Row(children ...html.Component) Inline {
	return Inline{Children: children}
}

// FormLayout stacks labels, controls and feedback with full-width inputs.
func FormLayout(children ...html.Component) Stack {
	return Stack{
		Children: children,
		Attrs:    html.Attrs{html.A("class", "form-fields")},
	}
}
