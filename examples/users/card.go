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
	out.Static(`<article class="card user-card">`)
	out.Component(ctx, components.Heading{Level: 2, Text: card.Name})
	out.Component(ctx, components.Paragraph(card.Email))
	out.Static(`</article>`)
	return out.Err()
}
