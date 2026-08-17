// Package e2e contains end-to-end tests that run the real compiled server as an OS
// process and exercise it over a real HTTP socket. They are guarded by the `e2e`
// build tag so they don't run in the normal (fast) `go test ./...`.
//
// Run them with:
//
//	go test -tags e2e ./e2e/...   (or: make test-e2e)
package e2e
