// Package protoconnect holds the Connect bindings for SysMLService, generated
// from the same api/proto/sysml.proto the gRPC bindings are generated from.
package protoconnect

//go:generate protoc -I .. --connect-go_out=../.. --connect-go_opt=module=github.com/Open-MBEE/OpenSysML ../sysml.proto
