# Self claim history bounded batch

## Confirmed gaps and changes

- `growth-fixture.mjs` had one revision-1 ledger row whose embedded snapshot still said revision 2; current revision 2 was absent. It now seeds complete revision-2/update and revision-1/create snapshots, before/after/candidate, IDs and claim/fid scope, base revision, confidence, evidence, provenance and source metadata. The seeded fictional story is unchanged.
- Forget and restore now append immutable ledger rows while retaining all old rows. Each has before/after, candidate with explicit Owner reason and actor ID, `owner_forget`/`owner_restore`, `owner:{owner}`, base/current revision, evidence/provenance, and updated timestamp. Forget preserves the full semantic snapshot. Restore copies only the selected complete accepted active/uncertain snapshot and appends a new revision. Absent legacy expiry/supersession remain the current boundaries through existing-object merge. Explicit nulls remain explicit. Restoration target revision is included in its receipt.
- Review remains an independent Owner acknowledgement, with review ID in its receipt, at the existing claim revision. It does not append a semantic revision or change claim/evidence/confidence. The independent review ledger survives forget/restore even when current `owner_review` clears.
- Actual MemoryPage regression: confirming a latest self baseline changed only command draft revision. Cancelling left the opened detail and secondary history stale. It now adopts the checked detail and refreshes self list/revisions/reviews, matching the existing memory command pattern.
- Actual MemoryPage regression: self “刷新当前详情” refreshed detail alone, leaving already-mounted history/reviews stale. The self branch now refreshes all relevant readers. Memory/reflection manual-refresh behavior is unchanged.
- Actual MemoryPage regression: a forget/restore receipt followed by the same revision with inconsistent status cleared accepted recovery. It now retains GET-only recovery until the matching status is read. Review retains its separate acknowledgement semantics; newer server revisions remain acceptable.
- Existing deep-memory self restore locator now explicitly selects revision 1, because full initial history exposes multiple valid restore buttons after forget.

## Verification executed

`node --test apps/web/test/demo-replica-memory.test.mjs apps/web/test/self-history-fixture.test.mjs`: **19/19 passed**. Consumer tests include the existing memory/reflection cases plus three reproduced defects, unknown original request identity replay, and accepted GET-only recovery. The three defect tests fail against the original MemoryPage, with expected stale current, missing revision history, and missing accepted recovery controls. Saved output: `evidence/self-history-unit/consumer-red.txt` and `focused-green.txt`.

The two fixture tests were also run against a temporary copy of `git show HEAD:apps/web/test/fixtures/growth-fixture.mjs`; both fail on missing revision arrays `[2,1]` and `[3,2,1]`. Saved output: `evidence/self-history-unit/fixture-red.txt`. Current fixture tests verify full snapshots, independent review, preserve evidence/provenance/value/expiry/supersession, append-only history, replay before CAS, rejected stale new identity, unavailable forgotten target, and another role remaining unchanged.

`pnpm --filter @fluctlight/web typecheck`: passed. `node --check` passed for growth fixture, `self-history-ui.mjs`, and adjusted `verify-memory-deep-ui.mjs`.

## Browser handoff — not executed in this batch

After the coordinated production build:

```sh
AUDIT_PHASE=pass104 node .trellis/tasks/10-28-demo-fullsite-audit/verification/self-history-ui.mjs
```

The script imports the installed browser tools from `/private/tmp/fluctlight-demo-audit-20261028/browser-tools`, launches Chrome, and uses `1348×925` and `390×844`, DPR 1, zh-CN, Asia/Shanghai, reduced motion. It refuses occupied port **4455**, spawns only its own loopback `ui-quality-server.mjs` child with growth=normal, and stops that child per case. Each case gets a fresh fixture and browser context using valid directory fid `fluctlight-fixture-one`. Optional `SELF_HISTORY_PORT` selects another unoccupied port; `AUDIT_CASES=normal,unknown,readback,adopt-refresh,status` limits cases.

Five cases per viewport:

1. Normal review cancellation and acknowledgement, forget cancellation/commit, complete v3/v2/v1 history, exact v1 source disclosure and restore to v4, page refresh, independent review ledger still readable.
2. Unknown POST response after actual fixture execution: latest adoption stays disabled until original admission is resolved; replay exact body/key once, no duplicate revision.
3. Accepted result with first detail GET failing: GET-only recovery, one POST total.
4. External confirmed version change while a self editor has unsent reason: explicit latest adoption preserves reason; cancel exposes confirmed current/history. External same-revision review followed by manual self detail refresh updates the mounted review ledger. These external setup POSTs are separately made with the test API request context, not counted as user page writes.
5. Accepted forget with same-revision malformed active detail: retains GET-only recovery and then reads the actual forgotten state with one POST total.

Screenshots/report are written under `evidence/self-history-${AUDIT_PHASE}`. Browser results, visual acceptance and production-build validation remain **pending main-session execution**. No fullsite completion claim.

## Limits

All fixture evidence is controlled in-memory HTTP through actual production consumers. It does not certify PostgreSQL, actual source validity, actor-fact dependency checks, real model conclusions, expiry clock transitions, or Core restoration acceptance. The fixture rejects unavailable/incomplete/non-active-or-uncertain restore snapshots but does not reproduce Core's full normalizer or dependency engine. Owner is fixture-instance configuration; actual Owner isolation remains Core-owned. No backend, prompt, CSS, Demo/reference, task coverage or spec changes were made in this batch.

## 2026-10-29 主线程验收更新

主线程实际验收：最终定向**22/22通过**。实际浏览器pass104的unknown/readback/adopt-refresh/status×双视口8组通过；normal在pass106两视口2组通过（每组21项断言），包括v1恢复到v4、刷新、旧Owner复核、正式Self详情四版历史。最新合并evidence/self-history/report-consolidated.json **10/10通过**并保留source_report。旧normal失败为验收文案/折叠展开时机假设，未为此修改产品。已人工核对手机完整来源与恢复详情、桌面正式历史。

生产build104与全量pass106（694/694、0skip）通过；这些受控HTTP结果不证明真实Core/PG/Worker/模型闭环。任务继续in_progress。
