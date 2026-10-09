# Goal / Actor governance surfaces

## 1. Scope / Trigger

GovernanceView displays independent GoalPlanningPanel and ActorContextEditor;
read-only InstanceDetailsDialog uses a readOnly GoalPlanningPanel and GoalPanel.

## 2. Signatures

BrowserClient goalSet/updateGoalSet, goalPlanningHistory/requestGoalPlanning,
actorContext/updateActorContext. GoalPanel creates via formal Owner commands,
including candidateOnly, ownerProtected, targetActorId and profileId.

## 3. Contracts

Send expectedVersion + expectedFactsRevision + stable idempotencyKey. Actor
writes send expectedContextVersion, source reason and change/correct operation.
Preserve drafts on error; explicit reread may update version without overwriting
text. Gate responses by instance/selection epoch before assigning state. Watch
both instance and target, since two instances often share the same Owner Actor.
Manual order and dependencies are actual backend writes; source provenance is
separate from prerequisites. Auto off does not stop existing execution.
Use randomId compatibility helper. Vue bindings escape all source/diagnostic text.
Default primary buttons need foreground token inside these glass panels because
unlayered application button inheritance can override Tailwind text utilities.

## 4. Error matrix

409: preserve draft and reload authority. Candidate/capacity: show actual error,
never report activation when only saved/requested. Private/cross-instance Actor:
backend rejects. Late old-instance response: ignore. No viable/failed/waiting/
policy block: show distinct run result and review condition.

## 5. Good / Base / Bad

Good: sort, save and refresh preserves server order; edit location/timezone changes
version and related action review. Base: closed planning offers suggestions only.
Bad: local-array sort, changing current Actor on a stale response, or black-on-black
primary text. Historical wishes remain visible for explicit semantic source review.

## 6. Tests

Goal panel executable harness, GoalPlanningPanel/ActorContextEditor draft/fence
harness, BrowserClient serialization, real authenticated production-build fixture
and HTTP/BFF/DB assertions. Test desktop and 390px, no horizontal overflow/console
errors. Do not count fixture UI alone as real Provider acceptance.

## 7. Wrong / Correct

Wrong: assign `state=await request()` then check epoch; watch target only.
Correct: await a local value, check epoch, assign; watch `[instance,target]`.
