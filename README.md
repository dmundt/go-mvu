# go-mvu

Build web apps in Go. Pages are made on your server, and forms work with or
without JavaScript.

## Stability

go-mvu is not stable yet. Its API may change at any time. Prototype cleanup
removes compatibility aliases rather than retaining deprecated alternatives;
see [migration notes](CHANGELOG.md#unreleased).

Reusable controls and widgets share one `components` package.
Upgrading from `shared`? See the [migration notes](CHANGELOG.md#unreleased).
Middleware keeps CSRF, security headers, streaming-aware recovery, and
diagnostics. Applications own request limits and cache policy; see
[HTTP middleware](docs/guides/middleware.md#request-bounds).

## Try it

You need Go 1.24 or newer.

```powershell
git clone https://github.com/dmundt/go-mvu.git
Set-Location go-mvu
go run .\examples\counter
```

Open <http://localhost:8080> and try the counter.

## Learn more

- [Start with Hello World](examples/hello/)
- [Explore an admin dashboard](examples/admin/)
- [Build your first app](docs/guides/first-app.md)
- [Try a form with validation](docs/guides/forms.md)
- [Build views with small Go functions](examples/counter/view.go)
- [Build shorter views and lists](docs/guides/components.md#shorter-composition)
- [Choose theme previews or saved preferences](docs/guides/components.md#action-aware-navigation)
- [Design interactive apps without application JavaScript](docs/guides/interactions.md)
- [Browse examples](examples/README.md)
- [Prepare for launch](docs/guides/production.md)
- [Security guide](docs/guides/security.md)
- [All guides](docs/guides/index.md)
- [What’s new](CHANGELOG.md)

The counter is for learning, not for public deployment. Read the
[production guide](docs/guides/production.md) before putting an app online.
