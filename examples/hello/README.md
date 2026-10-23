# Hello World

Smallest example: one page, one heading, one HTTP handler.

```powershell
go run .\examples\hello
```

Open <http://localhost:8085>.

- `view.go`: `Document` and `Heading` compose the page.
- `main.go`: `web.WritePage` writes it with the correct HTML content type.
- No model, runtime, session, forms, HTMX, or application JavaScript.
- The document includes the default theme; no separate CSS request.

For state changes and forms, continue with [counter](../counter/) or
[contact](../contact/). This intentionally small local example omits the
middleware and lifecycle setup demonstrated by [admin](../admin/).
