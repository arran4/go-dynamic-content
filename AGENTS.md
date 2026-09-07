# Agent Instructions for go-dynamic-content

When working on this repository, you **must** adhere to the following rules:

1. **Concurrency Requirements**:
   - `content.go` operates in highly concurrent environments. You must ensure thread safety, single-flight data loading, and avoid deadlocks.
   - Never use arbitrary `time.Sleep` calls in concurrency test assertions. Use proper channel synchronization, `sync.WaitGroup`, or deterministic bounds (e.g., `time.After` in a `select` block).

2. **Composability**:
   - Do not add explicit state-machine flags (`generating`, `validating`, `epoch`) to the primary `contentImpl` struct unless specifically approved.
   - Core behavior should remain composable: encapsulate stateful concurrency logic (e.g. single-flight checks, validator recursion detection) within closures or wrappers rather than modifying the root API type.

3. **Re-entrancy Requirements**:
   - User callbacks (`generate`, `isValid`, `onGenerate`, `onInvalidate`, `onClose`) must be executed strictly outside the `contentImpl.mu` lock.
   - Any re-entrant operations from these callbacks calling public methods (`Data()`, `Error()`, `HasContent()`) must inherently resolve without causing infinite recursion or deadlocks.

4. **Linting and Testing**:
   - Ensure you run formatting (`gofmt`), data race tracking (`go test -race ./...`), and vet (`go vet ./...`) before committing.
   - Maintain bounded timeout lengths natively inside test operations to gracefully detect deadlocks.
