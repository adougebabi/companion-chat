---
{"id":"legacy-a44d590b2f4eb387bd46baa1","title":"0001-shared-world-independent-actors.md","kind":"decision","status":"current","paths":["apps/core-go","apps/web"],"source":"docs/adr/0001-shared-world-independent-actors.md#sha256=63f10cff9db72f07e6a1c62b6f7420ac07ac187fb952e0fca05981f7eec7dd50","superseded_by":"","tasks":null,"critical":false,"path":"","hash":"","body":"","reason":""}
---
# Shared worlds and independent actors

The Owner confirmed on 2026-10-30 that a world is the shared environment, with
independent human, Fluctlight and NPC identities. Multiple Fluctlight instances
joining one world share its place and NPC identities and NPC world experience;
their directed relationships and private memories remain separate. Selecting an
existing world therefore joins it, while an explicit configuration copy creates
a separate parallel world. Existing worlds retain their identities and history;
equal names or shared definition provenance never authorize automatic merging.

The earlier character-owned world model made joining behave like cloning and
coupled NPC execution and acquaintance reads to the creating character. The
implementation must separate world authorization and shared execution from each
actor's current membership, actual presence, relationships and memory. A schedule
remains a plan; membership and matching predicted locations do not prove arrival
or acquaintance.

