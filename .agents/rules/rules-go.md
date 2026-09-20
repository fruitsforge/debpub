# Go Developer Rules (v1.22+)

These rules govern Go codebase generation, prioritizing simplicity, idiomatic patterns, memory efficiency, and modern Go constructs.

## 1. Code Simplicity & Design Philosophy
- **Interfaces**: Do NOT over-abstract. Enforce the proverb: *"Accept interfaces, return structs"*. Avoid creating interfaces with only one implementation unless required for mocking external I/O boundaries.
- **Project Structure**: Follow the standard Go project layout. Keep `main` packages thin; encapsulate core logic in internal modules under `/internal` to prevent external dependency leakage.
- **Composition over Inheritance**: Use embedding sparingly and only when it strictly models composition. Never embed structs to simulate class-based inheritance.

## 2. Error Handling & Control Flow
- **Error Wrapping**: Always use `%w` with `fmt.Errorf` when propagating errors, providing clear contextual messages. Do not use raw error string concatenation.
- **Error Verification**: Always use `errors.Is` for checking specific sentinel errors (e.g., `io.EOF`) and `errors.As` for checking custom error types. Never compare error strings directly.
- **Errors Join**: Use `errors.Join` to combine multiple errors (e.g., when cleaning up resources in deferred blocks).
- **No Silence**: Never ignore returned errors. Avoid using `_` for errors unless explicitly justified.
- **Panics**: Do NOT use `panic` or `recover` for normal control flow or error handling. Use them only for unrecoverable startup/runtime states.

## 3. Concurrency & Contexts
- **Resource/Leak Prevention**:
  - Every goroutine must have a defined lifecycle. Ensure goroutines exit cleanly by using cancelable contexts, closed channels, or `sync.WaitGroup`.
  - Always close channels on the sender side, never on the receiver side.
- **Context Propagation**:
  - Always pass `context.Context` as the first parameter of functions performing network, database, or filesystem operations. Name it `ctx`.
  - Do not store contexts inside structs unless necessary (e.g., in request-scoped structures).
  - Use `context.WithCancelCause` and `context.Cause` to propagate specific reasons for cancellation.
- **Race Conditions**:
  - Avoid sharing mutable state across goroutines. If mutability is required, use `sync.Mutex` or `sync.RWMutex`.
  - Prefer atomic operations (`sync/atomic`) for simple primitive counters.

## 4. Modern Go Features (Go 1.21 & 1.22+)
- **Structured Logging**: Standardize on `log/slog` for structured logging. Avoid using standard `log` or third-party loggers (like Logrus) unless requested by the project settings.
- **Slices & Maps Pack**: Leverage `slices` and `maps` packages for generic collection operations (e.g., `slices.Contains`, `slices.Clone`, `maps.Keys`).
- **Iterators**: Use Go 1.22 range-over-integers (`for i := range 10`) and range-over-functions (iterators) where custom sequence iteration is required.
- **Built-in Helpers**: Use `min`, `max`, and `clear` built-in functions instead of custom math helpers or loops.

## 5. Memory & Performance Optimization
- **Allocation Minimization**: Pre-allocate slices and maps using `make([]T, 0, capacity)` or `make(map[K]V, capacity)` when the target size is known beforehand.
- **Object Pooling**: Use `sync.Pool` to reuse short-lived, frequently allocated objects (e.g., byte buffers, JSON decoders/encoders) under high concurrent load.
- **Pointers vs. Values**: Pass small structs (e.g., configurations, simple data objects) by value to reduce GC pressure and escape analysis allocations. Use pointers for large structs or when mutability is intended.

## 6. Testing & Quality Assurance
- **Table-Driven Tests**: Always structure unit tests using the table-driven test pattern.
  ```go
  func TestCalculate(t *testing.T) {
      tests := []struct {
          name     string
          input    int
          expected int
          wantErr  bool
      }{
          {"positive input", 5, 10, false},
      }
      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              got, err := Calculate(tt.input)
              if (err != nil) != tt.wantErr {
                  t.Errorf("Calculate() error = %v, wantErr %v", err, tt.wantErr)
                  return
              }
              if got != tt.expected {
                  t.Errorf("Calculate() = %v, expected %v", got, tt.expected)
              }
          })
      }
  }
  ```
- **Linter Enforcements**: Code must satisfy a strict `golangci-lint` configuration enforcing `revive`, `errcheck`, `gosec`, `staticcheck`, `govet`, and `unused`.
- **Security Check**: Enforce `govulncheck` to scan for known vulnerabilities in dependencies.

## 7. Idiomatic Defer & Resource Cleanup
- **Read vs. Write Closures**:
  - For **readers** (`io.ReadCloser`, network responses, read-only file handles), use `defer r.Close()` immediately after checking the acquisition error.
  - For **writers** (`*gzip.Writer`, `*os.File`, `*bzip2.Writer`, `*xz.Writer`), `Close()` flushes critical unwritten buffers and checksums. **Never discard write `Close()` errors.**
  - **Idiomatic Writer Defer Pattern**: Use a deferred closure with a named return error:
    ```go
    func writeCompressed(data []byte) (err error) {
        w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
        if err != nil {
            return err
        }
        defer func() {
            if cerr := w.Close(); cerr != nil && err == nil {
                err = cerr
            }
        }()
        _, err = w.Write(data)
        return err
    }
    ```
- **Loop Scoping**:
  - Never place raw `defer` inside long-running loops or stream processors. Because `defer` executes upon function exit (not loop iteration), handles accumulate until the outer function terminates.
  - Encapsulate loop bodies in dedicated helper functions or immediate closures (`func() error { ... }()`) so `defer` executes per iteration.
- **DFA & Branch Completeness**:
  - Ensure every resource allocated is guaranteed to close along all exit paths (early errors, break conditions, panics).
  - Leveraging `defer` guarantees branch completeness and eliminates static analyzer / IDE Data Flow Analysis (DFA) resource leak warnings.

