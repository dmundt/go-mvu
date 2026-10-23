package main

import (
	"context"

	"github.com/dmundt/go-mvu/components"
	"github.com/dmundt/go-mvu/html"
)

// UserCard renders one user as a card.
type UserCard struct {
	Name  string
	Email string
}

func (card UserCard) RenderHTML(ctx context.Context, out *html.Context) error {
	out.Component(ctx, components.Card{
		Attrs: html.Attrs{html.A("class", "user-card")},
		Body: components.Stack{Children: []html.Component{
			components.Heading{Level: 2, Text: card.Name},
			components.Paragraph(card.Email),
		}},
	})
	return out.Err()
}
