// This is a DELIBERATELY-UNBUILDABLE fixture module. It lives in its own module
// (example.com/externalmod), outside l7e.io/yama/v2, and tries to import
// l7e.io/yama/v2/internal/bridge. Go's internal/ import-path rule must reject
// that with a compile error. externalmod_test.go in package yama drives `go build`
// here and asserts the failure — proving the module boundary is real, not assumed.
//
// It sits under testdata/ so the parent module's `go build ./...` ignores it, and
// the replace directive pins l7e.io/yama/v2 to the local checkout so resolution
// reaches the real internal package (and thus the real internal/ error) rather
// than a download failure.
module example.com/externalmod

go 1.25.0

require l7e.io/yama/v2 v2.0.0

replace l7e.io/yama/v2 => ../..
