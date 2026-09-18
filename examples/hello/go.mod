module example.com/hello

go 1.26.0

// The example builds against the checked-out Yama, not a published version.
replace l7e.io/yama => ../..

require (
	github.com/google/wire v0.7.0
	l7e.io/yama v0.0.0
)

require (
	github.com/google/subcommands v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/tools v0.50.0 // indirect
)

tool (
	github.com/google/wire/cmd/wire
	l7e.io/yama/cmd/yama
)
