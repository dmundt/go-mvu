package main

import (
	"context"

	"github.com/dmundt/go-mvu/html"
)

// Explicit low-level demo: class-free semantic HTML uses default theme styles.
// Keep direct markup here, not in the gallery's application composition.
var usersTable = html.ComponentFunc(func(_ context.Context, out *html.Context) error {
	out.Static(`<table><caption>Users</caption>`)
	out.Static(`<thead><tr><th scope="col">Name</th><th scope="col">Email</th></tr></thead>`)
	out.Static(`<tbody>`)
	out.Static(`<tr><td>Ada Lovelace</td><td>ada@example.test</td></tr>`)
	out.Static(`<tr><td>Grace Hopper</td><td>grace@example.test</td></tr>`)
	out.Static(`</tbody></table>`)
	return out.Err()
})
