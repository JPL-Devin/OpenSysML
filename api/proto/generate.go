package proto

// buf reads its configuration from the repository root, so the directive runs
// there; `make proto-go` pins the buf and plugin versions.
//go:generate sh -c "cd ../.. && make proto-go"
