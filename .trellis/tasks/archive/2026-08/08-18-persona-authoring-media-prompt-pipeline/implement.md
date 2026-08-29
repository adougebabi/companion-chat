# Implementation Plan: Persona authoring and media prompt composition

1. Inspect interview normalization/preview/activation, current blueprint JSON, foundation revision routes, context composition, and all media job producers/consumers.
2. Define versioned character-card and system-capability schema, defaults, provenance rules, and a migration only where normalized querying is required.
3. Extend the adaptive interview with one high-value question at a time, preview/skip compatibility, AI-inferred fields, and safe validation.
4. Refactor context composition into named layer helpers so chat, evolution, event narration, and media jobs share authority order without duplicating raw prompt assembly. Add the application-owned fixed reply-form rule as the final common chat constraint.
5. Add a shared user-visible assistant-output sentence segmentation/validation helper; update interactive streaming/SSE, proactive-message persistence, and the active chat client to deliver ordered multi-message replies while preserving a compatible single-message response during transition. Exclude internal structured JSON calls.
6. Replace the deterministic visual-intent compiler with a server-owned `MediaConceptEnvelopeV1` boundary that attaches authoritative identity/current-state facts but performs no natural-language/regex/default inference about subjects, objects, camera, pose, wardrobe, lighting, or constraints.
7. Define and validate `PersonaMediaConceptV1` and `MediaPromptTemplateV1`. Route chat media authorization, direct activity media, model-driven activity media, and inspector test media through an AI-persona concept stage followed by `imagePromptMasterContract`; use one fixed, rule-free template renderer for the provider prompt.
8. Remove all semantic visual heuristics from `mediaIntentFor`, `compileMediaPrompt`, and adjacent helpers. In particular remove server-side people counting/“共 X 人”, people-versus-object classification, selfie/POV/external-camera detection, device/geometry/pose/wardrobe/light defaults, keyword parsing, and semantic negative-prompt construction.
9. Preserve provider calls outside transactions and existing job lease semantics. Persist the envelope, persona concept, master-template result, rendered final prompt, and staged failure diagnostics; a concept/master failure retries the durable media job and, on exhaustion, fails only the media target with no semantic fallback prompt.
10. Update the debug contract/UI to show the persona concept and filled fixed-template result separately from the final rendered provider prompt, retaining redaction and persona isolation.
11. Add tests for layer immutability, reply-form segmentation/order, proactive-output handling, provenance, persona isolation, model-owned media concept/template flow, no-server-heuristic regressions, malformed concept/master retry-to-failed behavior, all media producers, and original-item media completion.
12. Add a confirmed tester-facing persona deletion flow: transactionally remove all selected-persona dependent storage, preserve unrelated personas/shared media assets, expose a destructive API, and reset the active browser selection.
13. Expand the interview/blueprint into a versioned, provenance-preserving character card with role, personality, appearance, and user-facing interaction-rule fields; retain a one-question adaptive/skip flow.
14. Protect the message composer draft from background refresh/render cycles, and make state resolution event-first so the displayed and prompted current location cannot conflict with a still-active life event.

## Validation

```sh
node --check server.js
node --check src/companion-main.js
npm test
git diff --check
```

Manual/provider matrix: initialize a persona with skipped/inferred fields; revise foundation; send a multi-sentence chat/proactive completion and confirm it becomes ordered one-sentence bubbles; create a café/shopping life event; inspect the persona concept and filled template; exercise declared selfie, external-capture, and POV concepts plus clothing/prop/animal examples; run valid/invalid concept/master responses; submit image/video through ComfyUI; confirm skeleton replacement and no new unread item.
Also delete a deliberately incorrect test persona and confirm it disappears after refresh while an unrelated persona, its messages, and its activity remain available.
Type an unfinished draft while bootstrap/activity polling runs and confirm it remains unchanged; create an active library event during a routine/schedule “class” period and confirm detail/chat context use the event location.

## Rollback

Keep existing blueprint and media job data readable. Completed media/assets remain intact. A rollback may disable new media submissions or retain legacy historical job rendering, but it must never reactivate the removed server-side visual-semantic heuristic compiler as a fallback for new jobs.
