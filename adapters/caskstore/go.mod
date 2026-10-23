module github.com/dmundt/go-mvu/adapters/caskstore

go 1.24.0

require (
	github.com/dmundt/go-cask v1.7.0
	github.com/dmundt/go-mvu v0.0.0
	github.com/gofrs/flock v0.12.1
)

require golang.org/x/sys v0.41.0 // indirect

replace github.com/dmundt/go-mvu => ../..
