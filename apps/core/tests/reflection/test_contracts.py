from fluctlight_core.cognition.contracts import ReflectionProposal
from fluctlight_core.providers.contracts import ModelRole, ProviderProvenance


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
