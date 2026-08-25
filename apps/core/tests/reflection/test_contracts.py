import asyncio
from typing import Any, cast

from fluctlight_core.cognition.contracts import ReflectionProposal
from fluctlight_core.providers.contracts import ModelRole, ProviderProvenance
from fluctlight_core.reflection.service import ReflectionCoordinator


def test_reflection_proposal_keeps_explicit_evidence_and_provenance() -> None:
    proposal = ReflectionProposal(
        proposal_id="proposal-1",
        fluctlight_id="fl-1",
        from_sequence=1,
        to_sequence=2,
        base_state_revision=4,
        payload={"memory_candidates": []},
        evidence_refs=("event-1",),
        provenance=ProviderProvenance(
            role=ModelRole.REFLECTION,
            endpoint_id="endpoint-1",
            model_id="model-1",
            prompt_version="prompt-v1",
            schema_version="reflection.v1",
            correlation_id="corr-1",
            token_budget=512,
        ),
    )
    assert proposal.provenance.role is ModelRole.REFLECTION


def test_reflection_coordinator_uses_stable_relationship_idempotency_key() -> None:
    proposal = ReflectionProposal(
        proposal_id="proposal-1",
        fluctlight_id="fl-1",
        from_sequence=1,
        to_sequence=2,
        base_state_revision=4,
        payload={
            "memory_candidates": [],
            "relationship_candidates": [
                {
                    "target_actor_id": "human-1",
                    "metrics": {"trust": 0.8},
                    "expected_revision": 0,
                }
            ],
        },
        evidence_refs=("event-1",),
        provenance=ProviderProvenance(
            role=ModelRole.REFLECTION,
            endpoint_id="endpoint-1",
            model_id="model-1",
            prompt_version="prompt-v1",
            schema_version="reflection.v1",
            correlation_id="corr-1",
            token_budget=512,
        ),
    )
    keys: list[str] = []

    class Memory:
        async def record(self, memory: Any) -> Any:
            return memory

    class Relationships:
        async def record_update(self, update: Any) -> Any:
            keys.append(update.idempotency_key)
            return type("Relationship", (), {"id": "relationship-1"})()

    coordinator = ReflectionCoordinator(cast(Any, Memory()), cast(Any, Relationships()))
    asyncio.run(coordinator.apply(proposal))
    asyncio.run(coordinator.apply(proposal))

    assert keys == [
        "reflection:proposal-1:relationship:0",
        "reflection:proposal-1:relationship:0",
    ]
