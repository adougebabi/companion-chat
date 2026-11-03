# Initial Code Evidence

- Active browser entry: `src/index.html` loads `src/companion-main.js`; `src/main.js` is legacy and must remain untouched.
- Current wizard: `openPersonaWizard()` creates an interview and `renderInterview()` asks the `interviewQuestions` from `server.js`; `renderInterviewPreview()` edits only foundation/interests/visualBaseline/supportingCast.
- Current server owner: `server.js` owns `interviewQuestions`, normalization, preview, activation, `lmCompletion()`, `modelJson()`, and `createPersona()`.
- Existing model pattern: `generateInitialLifeBlueprint()` uses `lmCompletion()` with a bounded timeout and deterministic fallback. It is a separate life-model stage and must not be overloaded with character extraction.
- Existing persistence: migration 4 creates `companion_interview_sessions`; additive metadata is appropriate for preserving natural-language source/provenance without storing the raw paragraph.
- Existing tests: `test/companion-api.test.mjs` exports/uses interview and blueprint helpers through `companionTestHooks`; model failure tests already mock `globalThis.fetch` and restore it in `finally`.
