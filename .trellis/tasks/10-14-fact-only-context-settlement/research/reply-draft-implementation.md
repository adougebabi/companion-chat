# F20 implementation evidence

Implemented the authorized private Fact-only draft pointer recovery. The user supplied only `reply_draft_scope_invalid`, no raw production arguments; this report does not identify the original pointer or claim a live incident replay.

## Production behavior

- `fact_only_reply_capability.go / factOnlyReplyService.prepare`: canonical ID remains `reply_draft_ + stableDigest(privateReplyLogicalKey(ctx, invocation))`; the existing logical key algorithm is unchanged. The optional pointer is compared without resolving it or selecting another row. A mismatch emits `reply.draft.reconciled` before canonical lookup; only warning code/disposition/counts/reason/surface/capability plus existing correlation identities are logged. No raw pointer, draft body, media, config or secrets enter this report.
- `reconcileReplyDraftHint`: blank/current hint produces no warning. All mismatches use `reply_draft_hint_ignored / continue_canonical_draft / not_current_draft`; the program does not pretend to distinguish stale/foreign/unparseable pointers without reading them.
- `factOnlyReplyCapability.Prepare / ExecuteDirectTx`: warning survives the frozen prepared envelope into successful completed/accepted/replayed output. Optional `warnings` is declared in the same private reply output schema. Actual business status remains committed/needs_action/waiting; failures are not converted to successful/no-effect receipts.
- `provider_ref_codec.go`: `decodePrivateReplyArguments` permits unresolved optional draft/evidence aliases to reach scoped reconciliation. Required action/target refs retain strict alias decoding and actual media scope checks. Existing `decodeReplyArguments` stays evidence-only.
- `adk_conversation_runtime.go`: minimal additional native switch branch selects the new private decoder only for `factOnlyReplyCapability`; Fact-only Moments retains the prior decoder. The file also contains the main session's F19 changes, which were preserved.
- Prepared/published/ready cache, frozen config/binding, v5 and durable style quota, generation leases, current source/owner/profile/publication guards and canonical action resume are unchanged. Cancellation is checked before recovery diagnostics and DB access.

## Actual RED / GREEN

Before changing behavior, the old inline pointer comparison was extracted without changing its error semantics into a production helper used by `prepare`. A compiled test executed four mismatch cases (old source, foreign, unknown short alias, arbitrary malformed string) and failed with the actual `reply_draft_scope_invalid / ErrUnauthorized`; the existing reply codec test failed with `provider_context_ref_alias_unknown`. This is controlled representation evidence, not original production parameters. PG prepare/publication RED was not run in this subagent.

After recovery changes, pure tests and race tests passed. Final command:

```sh
FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL= GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test -race ./internal/core -run 'Test(ReplyDraft|ProviderContextRefCodec)' -count=1 -v
```

Result: 8 top-level PASS (including existing parser coverage), 6 top-level PG SKIP. Four hint subcases passed. Exact output: `/tmp/reply-draft-final-tests.log`.

Broader targeted race selector also PASS:

```sh
FLUCTLIGHT_LIVE_PROVIDER_TEST=0 GO_CORE_TEST_DATABASE_URL= GOCACHE=/tmp/fluctlight-go-cache go -C apps/core-go test -race ./internal/core -run 'Test(ReplyDraft|ReplyEvidence|ReplyMedia|ProviderContextRefCodec|FactOnly.*(Registry|Definition)|PrivateChat.*(Style|Repair|Contract))' -count=1 -v
```

Includes scoped registry, existing evidence, media, native image intent, conservative diagnosis, frozen expression contract and same-model tool-free repair. PG children/parents remain SKIP, not persistence acceptance. Output: `/tmp/reply-draft-controlled-tests.log`. Controlled Provider uses fake transport; no real model calls or real publication.

`go vet ./internal/core`, `go build ./...`, gofmt for all four changed Go files, and `git diff --check` passed. Full combined workspace gate belongs to the main session.

## Task-owned PG tests prepared, not executed here

- `TestReplyDraftRecoveryPreparePublishReplay`: foreign owner/instance/conversation draft with secret text/asset/config; canonical new generation, cached wrong-pointer replay, real publication warning, published replay despite Writer assignment failure, exactly one Writer/message, foreign row unchanged.
- `TestReplyDraftRecoveryProviderFailureAndCancellation`: true Provider failure, same-run terminal reuse/no regeneration, cancellation, no sent message.
- `TestReplyDraftRecoveryCanonicalMediaOnly`: canonical pending action, foreign action resource rejection, same canonical cached media, result-aware adjustment from canonical failure, ready replay without another Writer. No real image execution.
- `TestReplyDraftRecoveryNativeBoundary`: actual app native invoker and private registry/codec route, unknown optional alias reaches reconciliation, one committed message and warning.
- `TestReplyDraftRecoveryClosedSourceCannotPublish`: actual failed source after preparation blocks publication despite wrong pointer.
- `TestReplyDraftRecoveryDatabaseFailureRemainsFailure`: closes only isolated fixture pool; DB failure remains error and Writer is never called.

Execute with an explicitly task-owned disposable URL only, through existing isolatedCoreTestRepository. No environment DB was connected by this subagent. Real Provider/ComfyUI/Temporal/production workflow: NOT_RUN. No commit, push, deployment, task/spec/state change by this implementation work.
