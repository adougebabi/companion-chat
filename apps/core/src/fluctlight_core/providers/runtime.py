"""Configured Provider execution ports used by cognition and memory workflows."""

from __future__ import annotations

import json
from collections.abc import AsyncIterator, Awaitable, Callable
from hashlib import sha256
from typing import Any
from uuid import uuid4

from sqlalchemy import select

from fluctlight_core.cognition.contracts import (
    AssessmentEnvelope,
    CognitionFact,
    DecisionProposal,
    FrozenAction,
    RealizationResult,
    ReflectionProposal,
    ReflectionWindow,
)
from fluctlight_core.inner_state.contracts import (
    AffectDirection,
    Appraisal,
    SemanticAssessment,
    SemanticPerception,
)
from fluctlight_core.platform.persistence import UnitOfWorkFactory
from fluctlight_core.settings.service import SettingsService

from . import schema
from .adapters import OpenAICompatibleAdapter
from .contracts import ModelRole, ProviderProvenance
from .service import ProviderEndpoint, RoleAssignment

ProviderRecorder = Callable[[ProviderProvenance], Awaitable[None]]


class ConfiguredProviderRuntime:
    """Resolve explicit role assignments and execute only the requested role."""

    def __init__(
        self,
        unit_of_work: UnitOfWorkFactory,
        settings: SettingsService,
        adapter: OpenAICompatibleAdapter | None = None,
        provenance_recorder: ProviderRecorder | None = None,
    ) -> None:
        self._unit_of_work = unit_of_work
        self._settings = settings
        self._adapter = adapter or OpenAICompatibleAdapter()
        self._provenance_recorder = provenance_recorder

    async def assess(self, fact: CognitionFact, *, correlation_id: str) -> AssessmentEnvelope:
        assignment, endpoint, secret = await self._resolve(ModelRole.COGNITIVE_ASSESSMENT)
        payload = await self._adapter.complete_structured(
            assignment,
            endpoint,
            secret,
            messages=[
                {
                    "role": "user",
                    "content": json.dumps(
                        {"event_type": fact.event_type, "payload": fact.payload},
                        sort_keys=True,
                    ),
                }
            ],
            schema_version="semantic.assessment.v1",
            request_id=f"assessment:{fact.idempotency_key}",
        )
        assessment_payload = payload.get("assessment", payload)
        decision_payload = payload.get("decision")
        if not isinstance(decision_payload, dict):
            raise RuntimeError("cognitive Provider response is missing decision")
        assessment = SemanticAssessment(
            schema_version="semantic.assessment.v1",
            perception=SemanticPerception(**dict(assessment_payload["perception"])),
            appraisal=Appraisal(**dict(assessment_payload["appraisal"])),
            direction=AffectDirection(assessment_payload["direction"]),
            strength=float(assessment_payload["strength"]),
            confidence=float(assessment_payload["confidence"]),
            evidence_refs=tuple(assessment_payload.get("evidence_refs", (fact.id,))),
            model=assignment.model_id,
            model_version=str(payload.get("model_version", "configured")),
            prompt_version=str(payload.get("prompt_version", "cognition.v1")),
            source_event_id=fact.id,
            idempotency_key=fact.idempotency_key,
        )
        decision = DecisionProposal(
            action_type=decision_payload["action_type"],
            payload=dict(decision_payload.get("payload", {})),
            confidence=float(decision_payload["confidence"]),
            evidence_refs=tuple(decision_payload.get("evidence_refs", (fact.id,))),
            decision_id=str(decision_payload.get("decision_id", f"decision_{fact.id}")),
        )
        provenance = ProviderProvenance(
            role=assignment.role,
            endpoint_id=endpoint.endpoint_id,
            model_id=assignment.model_id,
            prompt_version=assessment.prompt_version,
            schema_version=assessment.schema_version,
            correlation_id=correlation_id,
            token_budget=assignment.token_budget,
        )
        await self._record(provenance)
        return AssessmentEnvelope(
            assessment=assessment,
            decision=decision,
            provenance=provenance,
            correlation_id=correlation_id,
        )

    async def realize(self, action: FrozenAction, *, correlation_id: str) -> RealizationResult:
        assignment, endpoint, secret = await self._resolve(ModelRole.ACTION_REALIZATION)
        text = await self._adapter.stream_realization(
            assignment,
            endpoint,
            secret,
            messages=[
                {
                    "role": "user",
                    "content": str(action.payload.get("prompt", action.payload.get("text", ""))),
                }
            ],
            request_id=action.provider_request_id,
        )
        await self._record(
            ProviderProvenance(
                role=assignment.role,
                endpoint_id=endpoint.endpoint_id,
                model_id=assignment.model_id,
                prompt_version="realization.v1",
                schema_version="realization.v1",
                correlation_id=correlation_id,
                token_budget=assignment.token_budget,
            )
        )
        return RealizationResult(
            action.provider_request_id, {"text": text, "correlation_id": correlation_id}
        )

    async def stream_realize(
        self, action: FrozenAction, *, correlation_id: str
    ) -> AsyncIterator[str]:
        assignment, endpoint, secret = await self._resolve(ModelRole.ACTION_REALIZATION)
        chunks: list[str] = []
        async for chunk in self._adapter.stream_realization_chunks(
            assignment,
            endpoint,
            secret,
            messages=[
                {
                    "role": "user",
                    "content": str(action.payload.get("prompt", action.payload.get("text", ""))),
                }
            ],
            request_id=action.provider_request_id,
        ):
            if not isinstance(chunk, str) or not chunk:
                continue
            chunks.append(chunk)
            yield chunk
        text = "".join(chunks)
        if not text.strip():
            raise RuntimeError("realization Provider completion was empty")
        await self._record(
            ProviderProvenance(
                role=assignment.role,
                endpoint_id=endpoint.endpoint_id,
                model_id=assignment.model_id,
                prompt_version="realization.v1",
                schema_version="realization.v1",
                correlation_id=correlation_id,
                token_budget=assignment.token_budget,
            )
        )

    async def reflect(self, window: ReflectionWindow, *, correlation_id: str) -> ReflectionProposal:
        assignment, endpoint, secret = await self._resolve(ModelRole.REFLECTION)
        payload = await self._adapter.complete_structured(
            assignment,
            endpoint,
            secret,
            messages=[
                {
                    "role": "user",
                    "content": json.dumps(
                        {"from_sequence": window.from_sequence, "to_sequence": window.to_sequence},
                        sort_keys=True,
                    ),
                }
            ],
            schema_version="reflection.v1",
            request_id=f"reflection:{window.fluctlight_id}:{window.to_sequence}",
        )
        provenance = ProviderProvenance(
            role=assignment.role,
            endpoint_id=endpoint.endpoint_id,
            model_id=assignment.model_id,
            prompt_version=str(payload.get("prompt_version", "reflection.v1")),
            schema_version="reflection.v1",
            correlation_id=correlation_id,
            token_budget=assignment.token_budget,
        )
        await self._record(provenance)
        return ReflectionProposal(
            proposal_id=str(payload.get("proposal_id", f"reflection_{uuid4().hex}")),
            fluctlight_id=window.fluctlight_id,
            from_sequence=window.from_sequence,
            to_sequence=window.to_sequence,
            base_state_revision=window.base_state_revision,
            payload=payload,
            evidence_refs=tuple(payload.get("evidence_refs", (f"inbox:{window.to_sequence}",))),
            provenance=provenance,
        )

    async def embed(self, text: str) -> tuple[float, ...]:
        assignment, endpoint, secret = await self._resolve(ModelRole.EMBEDDING)
        request_id = (
            f"embedding:{endpoint.endpoint_id}:{assignment.model_id}:v1:"
            f"{sha256(text.encode('utf-8')).hexdigest()}"
        )
        vector = await self._adapter.embed(
            assignment, endpoint, secret, text=text, request_id=request_id
        )
        await self._record(
            ProviderProvenance(
                role=assignment.role,
                endpoint_id=endpoint.endpoint_id,
                model_id=assignment.model_id,
                prompt_version="embedding.v1",
                schema_version="embedding.v1",
                correlation_id=request_id,
                token_budget=assignment.token_budget,
            )
        )
        return vector

    async def _record(self, provenance: ProviderProvenance) -> None:
        if self._provenance_recorder is not None:
            await self._provenance_recorder(provenance)

    async def _resolve(self, role: ModelRole) -> tuple[RoleAssignment, ProviderEndpoint, Any]:
        async with self._unit_of_work.begin(command_id=f"provider-runtime:{role.value}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.model_roles, schema.provider_endpoints)
                        .join(
                            schema.provider_endpoints,
                            schema.provider_endpoints.c.id
                            == schema.model_roles.c.provider_endpoint_id,
                        )
                        .where(schema.model_roles.c.role == role.value)
                    )
                )
                .mappings()
                .one_or_none()
            )
        if row is None:
            raise RuntimeError(f"Provider role {role.value} is not configured")
        assignment = RoleAssignment(
            role=role,
            endpoint_id=row["provider_endpoint_id"],
            model_id=row["model_id"],
            token_budget=int(row["token_budget"]),
            timeout_seconds=int(row["timeout_seconds"]),
        )
        endpoint = ProviderEndpoint(
            endpoint_id=row["provider_endpoint_id"],
            kind=row["kind"],
            base_url=row["base_url"],
            secret_purpose=row["secret_purpose"],
        )
        return (
            assignment,
            endpoint,
            await self._settings.resolve_provider_secret(endpoint.secret_purpose),
        )
