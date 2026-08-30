import asyncio
from typing import Any, cast

import pytest
from fluctlight_core.cognition.contracts import ReflectionProposal
from fluctlight_core.providers.contracts import ModelRole, ProviderProvenance
from fluctlight_core.reflection.service import (
    ReflectionCoordinator,
    ReflectionValidationError,
    validate_reflection_payload,
)


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


def test_reflection_validation_rejects_malformed_memory_without_semantic_defaults() -> None:
    with pytest.raises(ReflectionValidationError, match=r"\.type is required"):
        validate_reflection_payload(
            {"memory_candidates": [{"content": "missing type", "confidence": 0.4}]}
        )

    with pytest.raises(ReflectionValidationError, match="confidence must be between"):
        validate_reflection_payload(
            {
                "memory_candidates": [
                    {"type": "episodic", "content": "fact", "confidence": 2}
                ]
            }
        )


def test_reflection_validation_rejects_relationship_without_numeric_metrics() -> None:
    with pytest.raises(ReflectionValidationError, match="metrics must be an object"):
        validate_reflection_payload(
            {"relationship_candidates": [{"target_actor_id": "human-1"}]}
        )


def test_reflection_validation_rejects_unsupported_enum_before_any_apply() -> None:
    with pytest.raises(ReflectionValidationError, match="type is unsupported"):
        validate_reflection_payload(
            {
                "memory_candidates": [
                    {"type": "not-a-memory", "content": "fact", "confidence": 0.4}
                ]
            }
        )


def test_reflection_coordinator_forwards_one_transaction_to_all_candidate_writers() -> None:
    calls: list[tuple[str, object]] = []

    class Memory:
        async def record(self, memory: Any, *, tx: object | None = None) -> Any:
            calls.append(("memory", tx))
            return memory

    class Relationships:
        async def record_update(self, update: Any, *, tx: object | None = None) -> Any:
            calls.append(("relationship", tx))
            return type("Relationship", (), {"id": "relationship-1"})()

    proposal = ReflectionProposal(
        proposal_id="proposal-tx",
        fluctlight_id="fl-1",
        from_sequence=1,
        to_sequence=2,
        base_state_revision=4,
        payload={
            "memory_candidates": [
                {"type": "episodic", "content": "fact", "confidence": 0.8}
            ],
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
            correlation_id="corr-tx",
            token_budget=512,
        ),
    )
    transaction = object()
    asyncio.run(ReflectionCoordinator(  # type: ignore[arg-type]
        Memory(), Relationships()
    ).apply(proposal, tx=transaction))
    assert calls == [("memory", transaction), ("relationship", transaction)]

    with pytest.raises(ReflectionValidationError, match="trend is unsupported"):
        validate_reflection_payload(
            {
                "relationship_candidates": [
                    {
                        "target_actor_id": "human-1",
                        "metrics": {"trust": 0.4},
                        "trend": "not-a-trend",
                    }
                ]
            }
        )
