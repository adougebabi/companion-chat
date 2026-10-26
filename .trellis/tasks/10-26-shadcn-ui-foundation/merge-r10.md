# R10 remote integration

The migration started at local 23e04bc7 without fetching. Remote 195017fc and
6cab8a7d already existed before migration commit 1a372e55. This omission caused
the divergence. Merge preserves both histories; do not abort or pick a side
wholesale.

Resolution owners:
- Keep incoming Core/browser-client world-copy contract and pure recovery helpers.
- Port incoming creation/source review, persona readback fences, account, chat,
  mobile, zero-role and media defaults into the React implementation.
- Keep React entry and no Vue/Pinia dependencies. legacy=1 remains rejected per
  the user's later explicit instruction (incoming merely ignored the parameter).
- Incoming v3-domain modules are unique domain editing/history/recovery tools,
  not an old app/router/shell. Preserve them via explicit routes in the React
  workspace, with Owner/generation cleanup. Their imperative custom controls
  remain a disclosed next migration batch; do not claim them fully shadcn.
- Preserve incoming behavioral regressions by adapting harnesses to React/state;
  old framework-specific source assertions are not a reason to restore Vue.
- No push and no full browser/live-service regression. Only checks necessary to
  finish the conflict resolution; user owns the broader acceptance pass.

Resolution completed:
- React main entry, native Media/Studio directories, explicit domain host and
  strict legacy rejection retained; no Vue/SFC files or dependencies restored.
- R10 profile, account, creation, chat and media tests now use React/shared state.
  Only the retired demo fixture and Vue/source-only harness tests were retired.
- Preserved incoming domain behavior tests for NPC save baselines, request
  localization, feed URL state, visual readback, media scope and timeline refresh.

Necessary merge validation:
- Web TypeScript check passed.
- Production build and fixture/import-graph guards passed (257 local modules).
- 63 affected tests passed after fixing the dependency scanner to distinguish
  directories from actual module files. Initial run: 62/63; targeted graph rerun:
  4/4, including the formerly failing check. No broad regression was repeated.
- Build retains the existing >500 kB shared chunk warning.
- Final fetch still resolves origin to 6cab8a7d, matching MERGE_HEAD.
- No live model/database/browser acceptance, deployment or push performed.
