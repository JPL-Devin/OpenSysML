# Phase 4: Runtime APIs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose Systemica runtime capabilities (eval, instantiate, execute actions/state machines) via Python client

**Architecture:** Extend protobuf schema with 4 new RPCs (Evaluate, Instantiate, ExecuteAction, ExecuteState), implement gRPC service handlers calling existing `internal/core/runtime` functions, add Python wrapper methods in Connection class plus new Instance class

**Tech Stack:** Go 1.23+, protobuf 7.35.1+, grpcio 1.83.0+, existing Systemica runtime (eval.go, instance.go, action_executor.go, state_executor.go)

---

## File Structure

**Go (gRPC Service):**
- `api/proto/sysml.proto` - Add EvaluateRequest/Response, InstantiateRequest/Response, ExecuteActionRequest/Response, ExecuteStateRequest/Response messages + 4 RPCs
- `internal/grpc/service.go` - Add 4 RPC handlers calling runtime package
- `internal/grpc/convert.go` - Add Value → protobuf conversion (ValueToProto), Instance → protobuf conversion (InstanceToProto)

**Python (Client):**
- `pysysml/instance.py` - New Instance class wrapping protobuf Instance message
- `pysysml/connection.py` - Add `eval()`, `instantiate()`, `execute_action()`, `execute_state()` methods
- `pysysml/errors.py` - New RuntimeError exception for execution failures
- `pysysml/__init__.py` - Export Instance, RuntimeError, add module-level eval()/instantiate() helpers

**Tests:**
- `internal/grpc/runtime_test.go` - Go unit tests for 4 new RPCs
- `tests/test_runtime.py` - Python unit tests with mocked RPCs
- `tests/test_runtime_integration.py` - Integration tests against real service with calc/action/state fixtures

---

## Task Breakdown

### Task 1: Extend Protobuf Schema with Runtime RPCs (Go)
**Objective:** Add 4 new RPC definitions + message types to api/proto/sysml.proto

**Files:**
- Modify: `api/proto/sysml.proto`
- Generate: `api/proto/sysml_pb2.py`, `api/proto/sysml_pb2_grpc.py` (Python stubs)
- Generate: `api/proto/sysml.pb.go`, `api/proto/sysml_grpc.pb.go` (Go stubs)

### Task 2: Implement Go Runtime RPC Handlers
**Objective:** Add Evaluate, Instantiate, ExecuteAction, ExecuteState handlers in internal/grpc/service.go

**Files:**
- Modify: `internal/grpc/service.go`
- Modify: `internal/grpc/convert.go` (add ValueToProto, InstanceToProto)
- Create: `internal/grpc/runtime_test.go` (unit tests for 4 RPCs)

### Task 3: Implement Python Instance Class
**Objective:** Create pysysml/instance.py wrapping protobuf Instance message

**Files:**
- Create: `pysysml/instance.py`
- Create: `tests/test_instance.py` (unit tests)

### Task 4: Add Python Runtime Methods to Connection
**Objective:** Add eval(), instantiate(), execute_action(), execute_state() methods to Connection class

**Files:**
- Create: `pysysml/errors.py` (RuntimeError exception)
- Modify: `pysysml/connection.py` (add 4 runtime methods)
- Create: `tests/test_runtime.py` (unit tests with mocked RPCs)

### Task 5: Add Module-Level Runtime Helpers
**Objective:** Add pysysml.eval(), pysysml.instantiate() convenience functions

**Files:**
- Modify: `pysysml/__init__.py` (add eval/instantiate, export Instance/RuntimeError)
- Modify: `tests/test_api.py` (add tests for module-level helpers)

### Task 6: Integration Testing
**Objective:** End-to-end tests with real service using calc/action/state fixtures

**Files:**
- Create: `tests/test_runtime_integration.py` (7 integration tests)
- Verify: All tests pass against real sysml-grpc service

---

## Definition of Done

- [ ] `pysysml.eval("2 + 2")` returns `4`
- [ ] `instance = pysysml.instantiate("PartName")` returns Instance object
- [ ] `result = conn.execute_action("ActionName", inputs={})` returns outputs dict
- [ ] Runtime errors raise `pysysml.RuntimeError` with message + diagnostics
- [ ] Integration tests pass for all 4 runtime operations
- [ ] All existing tests still pass (no regressions)
- [ ] Go tests pass: `go test ./...`
- [ ] Python tests pass: `pytest tests/`

---

