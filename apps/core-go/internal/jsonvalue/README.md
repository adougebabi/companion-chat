# jsonvalue

`jsonvalue.Unmarshal` preserves arbitrary business numbers at JSON copy and recovery boundaries shared by Core and the capability runtime. It depends only on the Go standard library.

Ordinary JSON numbers keep the existing `float64` representation in interface fields. Numbers whose numeric value would change on float64 serialization remain `json.Number`. Typed fields retain their declared type; `json.RawMessage` bytes remain unchanged. Strings, keys, schemas, booleans and null keep their original meaning.

```go
var value any
err := jsonvalue.Unmarshal([]byte(`9007199254740993`), &value)
// value is json.Number("9007199254740993"); json.Marshal retains that integer.
```

This decoder does not validate business authority, repair old rounded data, alter reference identities or perform arithmetic. Existing callers own validation and error handling.

`decode.go` contains the decoder; `decode_test.go` covers nested values, typed envelopes, raw snapshots, numeric compatibility and invalid input. See [DESIGN.md](DESIGN.md) for compatibility decisions. Run `go test ./internal/jsonvalue` from `apps/core-go`.
