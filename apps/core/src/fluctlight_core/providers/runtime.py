"""Configured Provider execution ports used by cognition and memory workflows."""

from __future__ import annotations

import json
import logging
from collections.abc import AsyncIterator, Awaitable, Callable, Mapping
from datetime import date
from hashlib import sha256
from typing import Any
from uuid import uuid4

from sqlalchemy import select

from fluctlight_core.cognition.contracts import (
    ActionType,
    AssessmentEnvelope,
    CognitionFact,
    DecisionEffect,
    DecisionProposal,
    FrozenAction,
    RealizationResult,
    ReflectionProposal,
    ReflectionWindow,
)
from fluctlight_core.diagnostics.contracts import (
    DiagnosticEvent,
    DiagnosticModelRun,
    DiagnosticSeverity,
)
from fluctlight_core.diagnostics.service import DiagnosticsService
from fluctlight_core.inner_state.contracts import (
    AffectDirection,
    Appraisal,
    SemanticAssessment,
    SemanticPerception,
)
from fluctlight_core.platform.persistence import UnitOfWorkFactory
from fluctlight_core.settings.service import SettingsService

from . import schema
from .adapters import OpenAICompatibleAdapter, structured_schema
from .contracts import ModelRole, ProviderProvenance
from .service import ProviderEndpoint, RoleAssignment

ProviderRecorder = Callable[[ProviderProvenance], Awaitable[None]]
logger = logging.getLogger(__name__)


COGNITIVE_ASSESSMENT_SYSTEM_PROMPT = (
    "Return one JSON object matching the semantic.assessment.v1 response schema. Always include "
    "decision.media_evaluation with a concise reason and a boolean needed value. Use only "
    "positive, negative, mixed, or neutral for direction. For every assessment and decision, "
    "cite at least one source reference, normally observation_id. For a conversation "
    "message, return ordered decision.effects. The first effect must be reply or no_op; "
    "later effects "
    "may be media_request or moment. reply and no_op payloads may contain only "
    "response_intent, never visible reply text. "
    "For life_world.daily_review, return ordered decision.effects choosing proactive_message, "
    "moment, or no_op. Its payload may "
    "contain response_intent and, only for a moment that needs an image, moment_media_request. "
    "moment_media_request is a complete model-owned visual concept for the image prompt role; "
    "it must not name an asset, provider, workflow, video, or a visible reply. "
    "Never include visible text, a conversation ID, or a visibility value. "
    "Choose proactive_message only when background_context contains a non-empty conversation_id. "
    "Choose moment when the Fluctlight has a meaningful shared update worth publishing; no_op "
    "is always valid. Every effect needs a unique id. "
    "Decide from the whole situation whether a visual artifact would materially improve the "
    "response; do not require a magic phrase or use a keyword rule. An explicit request is strong "
    "evidence, but a contextual need can also justify media_request. A media_request effect must "
    "carry a non-empty media_request visual concept with scene, action, mood, subject/object and "
    "capture details. If media_evaluation.needed is true, include a media_request effect; "
    "if false, "
    "do not include one. Keep response_intent limited to the visible acknowledgement; never put "
    "the "
    "visual concept there or emit final provider/workflow parameters. "
    "persona_profile, when present in the observation payload, is the authoritative Foundation "
    "context for interpreting this Fluctlight's stable inclinations and expression policy. "
    "conversation_history, when present, is the recent ordered dialogue context; use it to "
    "resolve references and preserve continuity. Do not "
    "write visible text in this JSON."
)

ACTION_REALIZATION_SYSTEM_PROMPT = (
    "Write the visible text for the already-frozen action. The action type is already frozen by "
    "a separate cognitive decision; never explain implementation limits or invent a body. For "
    "proactive_message, write one direct message to the Owner. For moment, write one concise "
    "shared Moment and retain any frozen moment_media_request unchanged. When action_type is "
    "media_request, acknowledge the requested image "
    "concisely while it is being generated. persona_profile is the authoritative, already-frozen "
    "Foundation context: follow "
    "its behavioral_policy for voice, length, punctuation, humor, directness, and emotional "
    "expression. Use personality only as durable inclination. Use conversation_history as the "
    "authoritative recent dialogue context and do not answer as if this were a new conversation. "
    "Return visible reply text only."
)

MEDIA_PROMPT_SYSTEM_PROMPT = (
    "Return one JSON object matching the media.prompt.v1 response schema. Convert the supplied "
    "visual request into a concrete final image-generation prompt. Do not return prose, markdown, "
    "hidden reasoning, or additional semantic decisions."
)

INITIAL_SCHEDULE_SYSTEM_PROMPT = """Return one JSON object matching the life.schedule.initial.v1
response schema. Create a complete initial daily Schedule from the supplied identity facts and
life context. Choose activities and scenes yourself, but never invent unsupported biographical
facts. Times must be RFC3339 values in the requested IANA timezone and must cover the complete
requested local date from 00:00:00 to the next 00:00:00 without gaps or overlaps. Do not return
prose, markdown, hidden reasoning, or a partial schedule."""


def _diagnostic_error_code(exc: Exception, fallback: str) -> str:
    value = str(exc).strip().lower().replace(" ", "_")
    return value[:120] or fallback


def _structured_prompt(messages: list[dict[str, Any]], schema_version: str) -> dict[str, Any]:
    """Persist the complete structured request contract for diagnostics."""
    return {
        "messages": messages,
        "schema_version": schema_version,
        "response_format": {
            "type": "json_schema",
            "json_schema": {
                "name": schema_version.replace(".", "_"),
                "strict": True,
                "schema": structured_schema(schema_version),
            },
        },
    }


class InitializationAnalysisError(RuntimeError):
    def __init__(
        self,
        code: str,
        *,
        status_code: int,
        details: dict[str, Any] | None = None,
    ) -> None:
        super().__init__(code)
        self.code = code
        self.status_code = status_code
        self.details = details or {}


INITIALIZATION_SYSTEM_PROMPT = """Return one JSON object matching the
fluctlight.initialization.v1 response schema. Do not return markdown, prose, hidden reasoning,
or keys outside that schema.

Generate a coherent Foundation from the description. Keep stable biography and values in
identity, durable inclinations in personality, and all tone/voice/cadence/wording/relationship
expression rules in behavioral_policy. Keep appearance, social context, enduring preferences,
repeated habits, real commitments, relationship seeds, and long-lived constraints in life_profile.
Do not put behavioral traits in identity.notes.

Use a real IANA timezone such as Asia/Shanghai, never an offset label such as UTC+8. For every
semantic field, preserve explicit user facts and mark provenance.field_sources as user_explicit;
mark direct consequences as user_inferred; otherwise generate a coherent value and mark it
model_generated. Do not leave semantic fields blank or use empty objects, empty arrays, neutral
placeholders, or identity.notes to avoid making a decision. Keep initial_goals concrete (one to
three) and provide at least one concrete initial_intentions action for every goal; goal_index is
zero-based. The server owns identifiers and personality.update_policy; do not return either.
The four life_profile collection fields must remain arrays of objects, including when there is one
entry. Return every life_profile field with its intended meaning: appearance for visual continuity,
social_background for support context, preferences for enduring interests, life_habits for repeated
practices, recurring_commitments for schedule constraints, and relationship_seeds for initial social
contexts. character_constraints contain long-lived boundaries. The generated Foundation must be
usable as an initial state, not a generic template."""


class ConfiguredProviderRuntime:
    """Resolve explicit role assignments and execute only the requested role."""

    def __init__(
        self,
        unit_of_work: UnitOfWorkFactory,
        settings: SettingsService,
        adapter: OpenAICompatibleAdapter | None = None,
        provenance_recorder: ProviderRecorder | None = None,
        diagnostics: DiagnosticsService | None = None,
    ) -> None:
        self._unit_of_work = unit_of_work
        self._settings = settings
        self._adapter = adapter or OpenAICompatibleAdapter()
        self._provenance_recorder = provenance_recorder
        self._diagnostics = diagnostics

    async def _record_model_run(
        self,
        *,
        assignment: RoleAssignment,
        endpoint: ProviderEndpoint,
        prompt: dict[str, Any],
        response: dict[str, Any] | None,
        correlation_id: str,
        status: str = "completed",
        error_code: str | None = None,
    ) -> None:
        if self._diagnostics is None:
            return
        await self._diagnostics.emit_model_run(
            DiagnosticModelRun(
                role=assignment.role.value,
                endpoint_id=endpoint.endpoint_id,
                model_id=assignment.model_id,
                prompt=prompt,
                response=response,
                correlation_id=correlation_id,
                status=status,
                error_code=error_code,
            )
        )

    async def assess(self, fact: CognitionFact, *, correlation_id: str) -> AssessmentEnvelope:
        assignment, endpoint, secret = await self._resolve(ModelRole.COGNITIVE_ASSESSMENT)
        messages: list[dict[str, Any]] = [
            {"role": "system", "content": COGNITIVE_ASSESSMENT_SYSTEM_PROMPT},
            {
                "role": "user",
                "content": json.dumps(
                    {
                        "observation_id": fact.id,
                        "event_type": fact.event_type,
                        "payload": fact.payload,
                    },
                    ensure_ascii=False,
                    separators=(",", ":"),
                    sort_keys=True,
                ),
            },
        ]
        try:
            payload = await self._adapter.complete_structured(
                assignment,
                endpoint,
                secret,
                messages=messages,
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
            raw_effects = decision_payload.get("effects")
            if not isinstance(raw_effects, list) or not raw_effects:
                raise RuntimeError("cognitive Provider response is missing decision effects")
            effects = tuple(
                DecisionEffect(
                    effect_id=str(effect["id"]),
                    action_type=effect["action_type"],
                    payload=dict(effect.get("payload", {})),
                )
                for effect in raw_effects
                if isinstance(effect, dict)
            )
            if len(effects) != len(raw_effects):
                raise RuntimeError("cognitive Provider decision effects must be objects")
            for effect in effects:
                if effect.action_type is ActionType.MEDIA_REQUEST:
                    concept = effect.payload.get("media_request")
                    if not isinstance(concept, dict) or not concept:
                        raise RuntimeError(
                            "cognitive media_request effect is missing a visual concept"
                        )
                    required_concept_fields = (
                        "scene",
                        "action",
                        "mood",
                        "subject",
                        "capture_details",
                    )
                    missing_concept_fields = [
                        field
                        for field in required_concept_fields
                        if not isinstance(concept.get(field), str)
                        or not concept[field].strip()
                    ]
                    if missing_concept_fields:
                        raise RuntimeError(
                            "cognitive media_request visual concept has empty fields: "
                            + ", ".join(missing_concept_fields)
                        )
            media_evaluation = decision_payload.get("media_evaluation")
            media_effects = [
                effect for effect in effects if effect.action_type is ActionType.MEDIA_REQUEST
            ]
            if isinstance(media_evaluation, dict):
                needed = media_evaluation.get("needed")
                if not isinstance(needed, bool):
                    raise RuntimeError("cognitive media evaluation has no boolean needed value")
                if needed and not media_effects:
                    raise RuntimeError(
                        "cognitive media evaluation requires a media_request effect"
                    )
                if not needed and media_effects:
                    raise RuntimeError(
                        "cognitive media_request effect contradicts media evaluation"
                    )
            logger.warning(
                "cognition.assessment.response fact_id=%s correlation_id=%s effect_count=%d "
                "effect_types=%s media_needed=%s media_effects=%d media_fields=%s",
                fact.id,
                correlation_id,
                len(effects),
                ",".join(effect.action_type.value for effect in effects),
                media_evaluation.get("needed") if isinstance(media_evaluation, dict) else None,
                len(media_effects),
                ",".join(
                    sorted(
                        {
                            field
                            for effect in media_effects
                            for field in (
                                effect.payload.get("media_request", {})
                                if isinstance(effect.payload.get("media_request"), dict)
                                else {}
                            )
                        }
                    )
                ),
            )
            decision = DecisionProposal(
                action_type=effects[0].action_type,
                payload=effects[0].payload,
                confidence=float(decision_payload["confidence"]),
                evidence_refs=tuple(decision_payload.get("evidence_refs", (fact.id,))),
                decision_id=str(decision_payload.get("decision_id", f"decision_{fact.id}")),
                effects=effects,
            )
        except Exception as exc:
            logger.error(
                "cognition.assessment.rejected fact_id=%s correlation_id=%s error_code=%s "
                "error_type=%s",
                fact.id,
                correlation_id,
                _diagnostic_error_code(exc, "assessment_response_invalid"),
                type(exc).__name__,
            )
            await self._record_model_run(
                assignment=assignment,
                endpoint=endpoint,
                prompt=_structured_prompt(messages, "semantic.assessment.v1"),
                response=None,
                correlation_id=correlation_id,
                status="failed",
                error_code=_diagnostic_error_code(exc, "assessment_response_invalid"),
            )
            raise
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
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt=_structured_prompt(messages, "semantic.assessment.v1"),
            response=payload,
            correlation_id=correlation_id,
        )
        return AssessmentEnvelope(
            assessment=assessment,
            decision=decision,
            provenance=provenance,
            correlation_id=correlation_id,
        )

    async def analyze_initialization(self, description: str) -> dict[str, Any]:
        request_id = f"initialization:{sha256(description.encode()).hexdigest()}"
        try:
            assignment, endpoint, secret = await self._resolve(ModelRole.INITIALIZATION)
        except Exception:
            await self._record_initialization_event(
                request_id,
                "initialization_role_unconfigured",
            )
            raise InitializationAnalysisError(
                "initialization_role_unconfigured",
                status_code=422,
                details={"correlation_id": request_id},
            ) from None
        messages: list[dict[str, Any]] = [
            {"role": "system", "content": INITIALIZATION_SYSTEM_PROMPT},
            {"role": "user", "content": description},
        ]
        try:
            payload = await self._adapter.complete_structured(
                assignment,
                endpoint,
                secret,
                messages=messages,
                schema_version="fluctlight.initialization.v1",
                request_id=request_id,
            )
        except Exception as exc:
            code = self._initialization_error_code(exc)
            await self._record_model_run(
                assignment=assignment,
                endpoint=endpoint,
                prompt=_structured_prompt(messages, "fluctlight.initialization.v1"),
                response=None,
                correlation_id=request_id,
                status="failed",
                error_code=code,
            )
            raise InitializationAnalysisError(
                code,
                status_code=503,
                details={"correlation_id": request_id},
            ) from exc
        payload["provenance"] = {
            "role": ModelRole.INITIALIZATION.value,
            "endpoint_id": endpoint.endpoint_id,
            "model_id": assignment.model_id,
            "prompt_version": "fluctlight.initialization.v1",
            "schema_version": "fluctlight.initialization.v1",
            "correlation_id": request_id,
        }
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt=_structured_prompt(messages, "fluctlight.initialization.v1"),
            response=payload,
            correlation_id=request_id,
        )
        return payload

    async def _record_initialization_event(self, correlation_id: str, error_code: str) -> None:
        if self._diagnostics is None:
            return
        await self._diagnostics.emit_event(
            DiagnosticEvent(
                event_type="fluctlight.initialization.failed",
                severity=DiagnosticSeverity.ERROR,
                correlation_id=correlation_id,
                payload={"error_code": error_code},
            )
        )

    @staticmethod
    def _initialization_error_code(exc: Exception) -> str:
        message = str(exc).lower()
        if "not valid json" in message:
            return "initialization_response_invalid_json"
        if "no json content" in message or "must be an object" in message:
            return "initialization_response_invalid"
        return "initialization_provider_unavailable"

    async def generate_initial_schedule(
        self,
        *,
        fluctlight_id: str,
        identity: Mapping[str, Any],
        life_profile: Mapping[str, Any],
        local_date: date,
        timezone: str,
    ) -> dict[str, Any]:
        assignment, endpoint, secret = await self._resolve(ModelRole.COGNITIVE_ASSESSMENT)
        correlation_id = f"schedule-initialization:{fluctlight_id}:{local_date.isoformat()}"
        messages: list[dict[str, Any]] = [
            {"role": "system", "content": INITIAL_SCHEDULE_SYSTEM_PROMPT},
            {
                "role": "user",
                "content": json.dumps(
                    {
                        "fluctlight_id": fluctlight_id,
                        "identity": identity,
                        "life_profile": life_profile,
                        "local_date": local_date.isoformat(),
                        "timezone": timezone,
                    },
                    ensure_ascii=False,
                    separators=(",", ":"),
                    sort_keys=True,
                ),
            },
        ]
        try:
            payload = await self._adapter.complete_structured(
                assignment,
                endpoint,
                secret,
                messages=messages,
                schema_version="life.schedule.initial.v1",
                request_id=correlation_id,
            )
            if not isinstance(payload.get("items"), list):
                raise RuntimeError("initial schedule response is missing items")
        except Exception as exc:
            await self._record_model_run(
                assignment=assignment,
                endpoint=endpoint,
                prompt=_structured_prompt(messages, "life.schedule.initial.v1"),
                response=None,
                correlation_id=correlation_id,
                status="failed",
                error_code=_diagnostic_error_code(exc, "initial_schedule_response_invalid"),
            )
            raise
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt=_structured_prompt(messages, "life.schedule.initial.v1"),
            response=payload,
            correlation_id=correlation_id,
        )
        return payload

    async def generate_media_prompt(
        self, *, media_request: Mapping[str, Any], correlation_id: str
    ) -> str:
        assignment, endpoint, secret = await self._resolve(ModelRole.MEDIA_PROMPT)
        messages: list[dict[str, Any]] = [
            {"role": "system", "content": MEDIA_PROMPT_SYSTEM_PROMPT},
            {
                "role": "user",
                "content": json.dumps(
                    {"media_request": media_request},
                    ensure_ascii=False,
                    separators=(",", ":"),
                    sort_keys=True,
                ),
            },
        ]
        payload = await self._adapter.complete_structured(
            assignment,
            endpoint,
            secret,
            messages=messages,
            schema_version="media.prompt.v1",
            request_id=correlation_id,
        )
        prompt = payload.get("prompt")
        if not isinstance(prompt, str) or not prompt.strip():
            raise RuntimeError("media prompt response is missing prompt")
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt=_structured_prompt(messages, "media.prompt.v1"),
            response=payload,
            correlation_id=correlation_id,
        )
        return prompt.strip()

    @staticmethod
    def _realization_messages(action: FrozenAction) -> list[dict[str, Any]]:
        source_text = action.payload.get("source_text")
        background_context = action.payload.get("background_context")
        if not isinstance(source_text, str) or not source_text.strip():
            if not isinstance(background_context, dict):
                raise RuntimeError("frozen action has no source message or background context")
            source_text = ""
        return [
            {"role": "system", "content": ACTION_REALIZATION_SYSTEM_PROMPT},
            {
                "role": "user",
                "content": json.dumps(
                    {
                        "user_message": source_text,
                        "action_type": action.action_type.value,
                        "response_intent": action.payload.get("response_intent", {}),
                        "persona_profile": action.payload.get("persona_profile", {}),
                        "conversation_history": action.payload.get("conversation_history", []),
                        "background_context": background_context or {},
                    },
                    sort_keys=True,
                    ensure_ascii=False,
                ),
            },
        ]

    async def realize(self, action: FrozenAction, *, correlation_id: str) -> RealizationResult:
        assignment, endpoint, secret = await self._resolve(ModelRole.ACTION_REALIZATION)
        messages = self._realization_messages(action)
        if action.action_type.value == "media_request":
            concept = action.payload.get("media_request")
            if not isinstance(concept, dict) or not concept:
                raise RuntimeError("media action has no frozen visual concept")
            text = "我来准备一张给你。"
            await self._record_model_run(
                assignment=assignment,
                endpoint=endpoint,
                prompt={"messages": messages, "schema_version": "realization.v1"},
                response={"text": text, "media_request": {"frozen": True}},
                correlation_id=correlation_id,
            )
            return RealizationResult(
                action.provider_request_id,
                {"text": text, "media_request": concept},
                status="media_requested",
            )
        text = await self._adapter.stream_realization(
            assignment,
            endpoint,
            secret,
            messages=messages,
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
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt={"messages": messages, "schema_version": "realization.v1"},
            response={"text": text},
            correlation_id=correlation_id,
        )
        return RealizationResult(
            action.provider_request_id, {"text": text, "correlation_id": correlation_id}
        )

    async def stream_realize(
        self, action: FrozenAction, *, correlation_id: str
    ) -> AsyncIterator[str]:
        assignment, endpoint, secret = await self._resolve(ModelRole.ACTION_REALIZATION)
        chunks: list[str] = []
        messages = self._realization_messages(action)
        async for chunk in self._adapter.stream_realization_chunks(
            assignment,
            endpoint,
            secret,
            messages=messages,
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
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt={"messages": messages, "schema_version": "realization.v1"},
            response={"text": text},
            correlation_id=correlation_id,
        )

    async def reflect(self, window: ReflectionWindow, *, correlation_id: str) -> ReflectionProposal:
        assignment, endpoint, secret = await self._resolve(ModelRole.REFLECTION)
        messages: list[dict[str, Any]] = [
            {
                "role": "user",
                "content": json.dumps(
                    {"from_sequence": window.from_sequence, "to_sequence": window.to_sequence},
                    ensure_ascii=False,
                    separators=(",", ":"),
                    sort_keys=True,
                ),
            }
        ]
        payload = await self._adapter.complete_structured(
            assignment,
            endpoint,
            secret,
            messages=messages,
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
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt=_structured_prompt(messages, "reflection.v1"),
            response=payload,
            correlation_id=correlation_id,
        )
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
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt={"text": text, "schema_version": "embedding.v1"},
            response={"dimensions": len(vector)},
            correlation_id=request_id,
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
                        .where(schema.provider_endpoints.c.capability_status == "available")
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
            await self._settings.resolve_optional_provider_secret(endpoint.secret_purpose),
        )
