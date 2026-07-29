# simd-unityyaml

SIMD structural indexing for the **Unity YAML dialect** — the rare YAML subset
where a [simdjson](https://github.com/simdjson/simdjson)-style stage 1 is
actually possible.

## Why this can exist

Full YAML famously resists SIMD parsing: indentation is significant, you
can't know whether a token opens a block mapping until you see `:` (unbounded
lookahead), and unquoted scalars make everything context-dependent. That's
why there is no "simdyaml".

Unity's serialization format is the exception. It is strictly **line-based**,
every document starts with a fixed anchor header (`--- !u!<classID>
&<fileID>`), and indentation follows rigid rules. Structure can therefore be
recovered from two vectorizable scans: newline positions and fixed-prefix
matches. This library does exactly that.

## What it does (v0.1 — stage 1)

`uyaml.Index(data)` returns a compact structural **tape**:

- byte offset of every line start (`LineStarts`)
- every document header (`Docs`): classID, fileID, `stripped` flag, line

The newline scan runs on a NEON kernel on arm64 (hand-written Go assembly,
**no cgo**) with a portable SWAR fallback everywhere else. Both are
differentially fuzzed against an independent scalar reference — the SIMD
path must be bit-identical to scalar truth.

```go
tape, err := uyaml.Index(prefabBytes)
for _, d := range tape.Docs {
    fmt.Println(d.ClassID, d.FileID, d.Stripped)
}
```

## Numbers (Apple M4 Pro, 8 MiB synthetic Unity YAML)

| | throughput |
|---|---|
| newline scan, NEON kernel | ~75 GB/s |
| newline scan, SWAR fallback | ~7.6 GB/s |
| `Index` end-to-end (NEON) | ~3.1 GB/s |
| `Index` end-to-end, pure scalar reference | ~1.8 GB/s |

Reproduce with `go test -bench . -run '^$'`, or point
`UYAML_BENCH_FILE=/path/to/big.unity` at a real scene. Note the honest
caveat: stage 1 builds a structural index, which is less work than a full
parse — compare against full parsers accordingly.

## Honesty contract

This library never guesses. A line that starts with `--- !u!` but fails to
parse is returned as a `Doc` with `Malformed: true` — counted, never
dropped, never repaired. Inputs over the 4 GiB tape limit are rejected with
an explicit error, not truncated.

## When NOT to use this

If your workload is many small files, your bottleneck is almost certainly
syscalls, not parsing — we measured a real Unity asset CLI at 64% syscall
time, where SIMD parsing has a ~5% ceiling. This library pays off on large
single buffers (multi-MB scenes, mmap'd files, server-side pipelines).
Measure first.

## Roadmap

- [x] stage 1: newline tape + document headers (NEON + SWAR + scalar oracle)
- [ ] key-colon positions per line (the second vectorizable scan)
- [ ] AVX2 kernel (amd64)
- [ ] stage 2: field extraction into a typed tape (`m_Name`, `fileID:`/`guid:` refs)
- [ ] adoption as an optional backend in [unity-lens](https://github.com/cocone-development/unity-lens)

## CLI

```
go install github.com/epheria/simd-unityyaml/cmd/uyaml@latest
uyaml index Assets/Scenes/Main.unity
uyaml bench Assets/Scenes/Main.unity
```

MIT licensed.
