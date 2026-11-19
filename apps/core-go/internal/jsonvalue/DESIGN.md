# JSON value decoder design

## Goal and ownership

Prevent silent precision loss when arbitrary Actor data is copied through Core, frozen authority, capability snapshots and Writer evidence. Core and capability use one implementation. No business schema, token policy, reference algorithm or source text changes belong here.

## Decisions

On 2026-11-19, selected `encoding/json.Decoder.UseNumber` followed by normalization of interface-valued numeric leaves. A blanket UseNumber change would alter ordinary number types; decoding everything into float64 would corrupt integers and decimals. The decoder retains float64 only when its shortest JSON numeric spelling is numerically equal to the source literal. Decimal coefficient/exponent comparison takes linear space/time without constructing enormous integers for exponent values.

Reflection visits decoded exported fields, maps and arrays to preserve the same behavior for typed envelopes containing interface fields. Raw byte slices, including json.RawMessage snapshots, are excluded from normalization. Declared int/float fields retain standard-library decoding. Invalid JSON follows the standard unmarshal error path.

## Compatibility and limitations

Existing rounded snapshots remain rounded and keep their identity. No attempt is made to infer lost digits. Hash-bound snapshots retain raw bytes in typed decoding; existing generic map copies retain their usual deterministic JSON serialization. JSONB canonicalization and actual frozen-reference validation are covered by Core integration tests.

Consumers that deliberately convert a retained json.Number to float64 still own any arithmetic precision tradeoff. This module guarantees JSON numeric roundtrip value, not arbitrary-precision business arithmetic or preservation of insignificant numeric whitespace/spelling for ordinary float-compatible values.

## Security boundary

The module has no I/O, logging, executable parsing, credential handling or authority inference. It never strips keys or modifies user prose. Standard JSON parsing controls syntax; existing domain validation still controls ownership, schema and ranges. Tests cover huge exponents without allocating exponent-sized big integers, malformed input, raw snapshots and ordinary numeric compatibility.

## History

2026-11-19: introduced for the independently authorized Actor precision repair, with shared Core/capability copy and recovery integration.
