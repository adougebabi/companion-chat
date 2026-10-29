# FieldPort immediate change / blur regression — 2026-10-29

Scope: `DomainPrimitives.tsx` native event forwarding and focused component regressions only. Existing World/Goal work and concurrent Studio work were preserved.

Observed production precursor: `evidence/goal-planning-advanced-redraw102/report.json` records a `NotFoundError` during dependency checkbox change, despite save/readback assertions. `FieldPort.commit` already emits source change synchronously; the source onchange redraw removes the focused visible checkbox; its blur unconditionally emitted a second source change, reentering the redraw.

Fix: use the same `changesImmediately` decision (checkbox/radio/select/date/datetime-local) for commit and blur. Those controls commit change on value selection; blur only captures selection and forwards focusout. Text/number/textarea retain their input updates and change-on-blur behavior. File retains its existing dedicated branch. No API, CSS, command, backend, prompt, fixture or browser-check changes.

Red proof: before the production fix, `node --test apps/web/test/r11-domain-primitives.test.mjs` failed 2 of 10 tests. The new checkbox test focuses the real ported control and transfers focus synchronously inside the original carrier onchange, exercising reentrant blur: 2 changes observed, 1 expected. The date test observed a change on untouched trigger blur (1 observed, 0 expected). See `../evidence/field-port-immediate-change/unit-red.txt`.

Green proof: `node --test apps/web/test/r11-domain-primitives.test.mjs apps/web/test/r11-controls.test.mjs` passed 15/15. New checkable test covers checkbox and radio: exactly one change, max callback depth 1, exactly one focusout with the original relatedTarget, checked state and exact original FormData. Text/number/textarea tests retain draft, input/change timing, selection and focusout. Existing datetime test retains its exact one input / one change expectation. See `../evidence/field-port-immediate-change/unit-green.txt`.

Additional checks: `pnpm --filter @fluctlight/web typecheck` passed; `git diff --check` passed.

Boundary: this unit regression exercises the nested event trigger in JSDOM, not the browser's actual innerHTML-removal exception. No coordinated build or browser run was performed during concurrent Studio edits. Main session must rebuild the combined sources and rerun `verification/verify-goal-planning-ui.mjs` with all browser exception guards and explicit policy baseline rebase intact. Desktop and narrow production browser result remains pending; no fullsite status was changed.

## 2026-10-29 主线程验收更新

主线程重建并实际浏览器复验完成：goal-planning-advanced-pass103的4类场景×2视口 **8/8通过**，errors均为空；之前redraw102中的实际NotFoundError不再出现。单元15/15与实际浏览器证据互相补充。

生产build104与全量pass106（694/694、0skip）通过；这些受控HTTP结果不证明真实Core/PG/Worker/模型闭环。任务继续in_progress。
