---
name: go-developer
description: Expert AI assistant for Go (Golang) development. Focuses on idiomatic structures, table-driven testing, secure dependencies, memory safety, and structured logging.
globs: "*.go, go.mod, go.sum, .golangci.yml, .golangci.yaml"
alwaysApply: false
---

# Go Developer Skill

Structured guidelines and best practices for writing clean, senior-grade Go programs using modern standards (Go 1.22+).

## Guidelines & Best Practices

1. **Design Clean, Simple APIs**:
   - Limit interface exposure. Accept interface parameters and return concrete structs.
   - Avoid package dependency cycles. Keep helper packages distinct.

2. **Errors & Propagation**:
   - Handle all errors explicitly. Do not swallow errors.
   - Wrap errors with `%w` using `fmt.Errorf` when passing them up the stack.
   - Leverage `errors.Is` and `errors.As` for safe error inspection.

3. **Concurrency & Resource Management**:
   - Ensure clean goroutine termination using context cancellation, `sync.WaitGroup`, or channeled exit signals.
   - Guard shared mutable states with `sync.Mutex` or `sync.RWMutex`.
   - Propagate contexts (`ctx`) cleanly as the first argument in blocking operations.

4. **Testing Excellence**:
   - Write table-driven unit tests. Include positive and negative test cases.
   - Isolate test states cleanly. Run test suites with `go test -race -v ./...`.

5. **Modern Language Features**:
   - Use `log/slog` for structured logging.
   - Leverage `slices` and `maps` packages for generic functions.
   - Utilize Go 1.22 range-over-integers and iterator structures.
