# Feed independent verification

- Core contracts: moment_feed.go supports owner-scoped fid/status/q cursor reads, versioned interactions and Owner publication. createMoment worldId is used as fid by Core; no author impersonation.
- Fixed: comments readback must follow actual next_cursor when accepted comment is no longer on the first page. Regression included.
- Fixed: successful detail refresh adopts actual record into filtered feed, refuses older revisions and removes hidden rows. Regression included.
- Owner-switch concern was checked against actual store notifications and studio-draft-storage cleanup: old prefix is intentionally cleared on account switch, and late responses cannot recreate it. Actual Owner-switch regression passes; retaining old account checkpoints after logout would violate existing cleanup. No shared ownership-policy change.
- Native route, no-character global feed, author/media fields and request identity checks independently verified. Main agent ran final tests and browser checks.
