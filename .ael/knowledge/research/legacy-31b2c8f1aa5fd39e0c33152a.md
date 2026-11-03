---
{"id":"legacy-31b2c8f1aa5fd39e0c33152a","title":"design.md","kind":"research","status":"historical","paths":[],"source":".trellis/research/11-02-ft-r2-defects/design.md#sha256=cce04e3d1afc4a8d8429e7204b02a25292e220ffdeedb145a74ad0fde3194b88","superseded_by":"","tasks":null,"critical":false,"path":"","hash":"","body":"","reason":""}
---
# Boundary and design
D8 lives in creation-draft/controller/page/view and creation-world completion. Default true was accepted as authorization, and the daily option remained available with existing worlds. Require explicit confirmation at domain-command completion; gate UI on authoritative complete catalog. Backend activation remains world-free. World mutation owns atomic base scaffolding, including immutable NPC source provenance and no acquaintance/presence claims. Existing world data is never repaired/merged/deduped.

D7/D9/D10: retain independent world clocks; distinguish clock lifetime from bounded per-hour work, preserve world/hour staging uniqueness and advisory lock, skip incomplete worlds before model/Kev. Owner requests retain original durable identity and CAS; retry transient model failures with bounded backoff and refresh stale snapshots before another assessment.

D11: parse origin list in startup config; canonical HTTP(S) origin validation and exact request membership; keep current CSRF/session protections. No token limits or prompt truncation changes.

D12: generated templates intentionally stay outside the manual settings directory, while Life Event reads only expose actual applied results. Add world-scoped read-only generation/clock presentation to the selected world's events tab. Diagnostics creation-time pagination can bury permanent clocks; independently read and deduplicate clocks, and filter selected intent types before pagination. Extend existing diagnostic filters/OpenAPI; preserve all generation, Kev, execution and Owner contracts. No migration or prompt changes.

