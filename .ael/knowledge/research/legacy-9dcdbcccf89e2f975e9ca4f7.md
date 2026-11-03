---
{"id":"legacy-9dcdbcccf89e2f975e9ca4f7","title":"feed-review.md","kind":"research","status":"historical","paths":[],"source":".trellis/research/archive/2026-10/10-27-demo-workspace-replica/research/feed-review.md#sha256=eb9e635b2dc08be8612af795b758ebcea80d01117f82ecb453dd32aaf349bdad","superseded_by":"","tasks":null,"critical":false,"path":"","hash":"","body":"","reason":""}
---
# Feed independent verification

- Core contracts: moment_feed.go supports owner-scoped fid/status/q cursor reads, versioned interactions and Owner publication. createMoment worldId is used as fid by Core; no author impersonation.
- Fixed: comments readback must follow actual next_cursor when accepted comment is no longer on the first page. Regression included.
- Fixed: successful detail refresh adopts actual record into filtered feed, refuses older revisions and removes hidden rows. Regression included.
- Owner-switch concern was checked against actual store notifications and studio-draft-storage cleanup: old prefix is intentionally cleared on account switch, and late responses cannot recreate it. Actual Owner-switch regression passes; retaining old account checkpoints after logout would violate existing cleanup. No shared ownership-policy change.
- Native route, no-character global feed, author/media fields and request identity checks independently verified. Main agent ran final tests and browser checks.

