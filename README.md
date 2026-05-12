# solvela-go

> **This SDK has moved.** Source now lives in the Solvela monorepo:
> https://github.com/solvela-ai/solvela/tree/main/sdks/go

**New import path:**

```go
import solvela "github.com/solvela-ai/solvela/sdks/go"
```

```
go get github.com/solvela-ai/solvela/sdks/go
```

Issues, PRs, and discussion: https://github.com/solvela-ai/solvela/issues

The monorepo's `sdks/go/` is byte-equivalent to this repo's `main` at archive
time, with the Go module path rewritten. Existing `go get
github.com/solvela-ai/solvela-go` pins still resolve from
`proxy.golang.org`'s cache; new code should use the path above.
