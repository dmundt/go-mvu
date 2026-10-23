module github.com/dmundt/go-mvu/examples/production

go 1.25.0

require (
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/dmundt/go-mvu v0.0.0
	github.com/go-jose/go-jose/v4 v4.1.4
	golang.org/x/oauth2 v0.36.0
)

replace github.com/dmundt/go-mvu => ../..
