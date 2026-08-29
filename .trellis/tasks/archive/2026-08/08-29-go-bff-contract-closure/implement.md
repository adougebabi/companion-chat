# Implementation Plan

1. Record the browser OpenAPI method/path inventory and add a normalized
   artifact-to-Go route parity test.
2. Add bounded recursive Core-error detail sanitization and route tests for
   safe fields versus credentials/internal payloads.
3. Harden media proxy handling for nil/failed upstream bodies and add Range,
   header allow-list, and failure tests.
4. Add a real `httptest.Server` browser→Go BFF→fake Core integration path for
   setup, Fluctlight creation, conversation creation, and NDJSON turn output.
5. Add a real HTTP disconnect/cancellation regression test around the turn
   stream.
6. Run `gofmt`, Go race/vet/build checks, YAML/diff checks, and review the
   final diff for forbidden Core/domain/schema changes.
7. Update the task handoff/acceptance evidence, commit the stage, and leave
   remote push for explicit user confirmation.
