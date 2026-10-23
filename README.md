# go-mvu

Build web apps in Go. Pages are made on your server, and forms work with or
without JavaScript.

## Stability

go-mvu is not stable yet. Its API may change at any time.

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
- [See a small function-based view](examples/contact/view.go)
- [Build shorter views and lists](docs/guides/components.md#shorter-composition)
- [Design interactive apps without application JavaScript](docs/guides/interactions.md)
- [Browse examples](examples/README.md)
- [Prepare for launch](docs/guides/production.md)
- [Security guide](docs/guides/security.md)
- [All guides](docs/guides/index.md)
- [What’s new](CHANGELOG.md)

The counter is for learning, not for public deployment. Read the
[production guide](docs/guides/production.md) before putting an app online.
