module github.com/dmundt/go-mvu/examples/production

go 1.25.0

require (
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/dmundt/go-mvu v0.0.0
	github.com/dmundt/go-mvu/adapters/caskstore v0.0.0
	github.com/go-jose/go-jose/v4 v4.1.4
	golang.org/x/oauth2 v0.36.0
)

require (
	github.com/dmundt/go-cask v1.7.0 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	golang.org/x/sys v0.41.0 // indirect
)

replace github.com/dmundt/go-mvu => ../..

replace github.com/dmundt/go-mvu/adapters/caskstore => ../../adapters/caskstore
