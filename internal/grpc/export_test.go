package grpc

// The REPL imports this package for its feature-value serialization, so a test
// comparing the two runs as an external package and borrows these.
var (
	MustNewServiceForTest = mustNewService
	QueryModelForTest     = queryModel
)
