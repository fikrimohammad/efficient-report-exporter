// Package integration holds tests and benchmarks that need a real connection to
// the project's dependencies (MySQL, S3-compatible MinIO, Infisical, etcd).
//
// Everything in this package is gated behind the `integration` build tag, so the
// unit test suite (`go test ./...`) never compiles or runs it:
//
//	go test -tags integration ./integration/...
//	go test -tags integration -run '^$' -bench . -benchmem ./integration/...
package integration
