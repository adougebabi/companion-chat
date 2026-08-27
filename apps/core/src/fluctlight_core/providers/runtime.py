"""Configured Provider execution ports used by cognition and memory workflows."""

from __future__ import annotations

import json
from collections.abc import AsyncIterator, Awaitable, Callable, Mapping
from datetime import date
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
from .adapters import OpenAICompatibleAdapter
from .contracts import ModelRole, ProviderProvenance
from .service import ProviderEndpoint, RoleAssignment

ProviderRecorder = Callable[[ProviderProvenance], Awaitable[None]]


COGNITIVE_ASSESSMENT_SYSTEM_PROMPT = (
    "Return JSON only, matching this shape:\n"
    '{"assessment":{"perception":{"event_kind":"conversation.message",'
    '"observed_intent":null,"sentiment":null,"social_signals":[],'
    '"environment_meaning":null},"appraisal":{"relevance":0.0,'
    '"goal_congruence":0.0,"reward":0.0,"loss":0.0,"social_threat":0.0,'
    '"controllability":0.0,"responsibility":0.0,'
    '"relationship_significance":0.0,"expected_effect":0.0},'
    '"direction":"neutral","strength":0.0,"confidence":0.0},'
    '"decision":{"action_type":"reply","payload":{"response_intent":{}},'
    '"confidence":0.0}}\n'
    "Use only positive, negative, mixed, or neutral for direction. All numbers are 0 to 1. "
    "social_signals is always an array of strings, using [] when empty. For a conversation "
    "message, choose reply, media_request, or no_op. reply and no_op payloads may contain only "
    "response_intent, never visible reply text. "
    "Choose media_request only when the user explicitly requests a visual; its payload must be "
    '{"response_intent":{}} and must not contain visible reply text or final media parameters. '
    "Do not write a visible reply in this JSON."
)

ACTION_REALIZATION_SYSTEM_PROMPT = (
    "Write the visible reply to the user's message. The action type is already frozen by a "
    "separate cognitive decision; never explain implementation limits or invent a body. When "
    "action_type is media_request, acknowledge the requested image concisely while it is being "
    "generated. Return visible reply text only."
)

MEDIA_PROMPT_SYSTEM_PROMPT = (
    'Return JSON only: {"prompt":"final image-generation prompt"}. '
    "Convert the supplied visual request into a concrete image prompt. Do not return prose, "
    "markdown, hidden reasoning, or any key other than prompt."
)

MEDIA_RESPONSE_SYSTEM_PROMPT = (
    "Write a concise visible acknowledgement of the user's request. Then call the provided "
    "request_media tool with every image parameter needed by a generic media prompt optimizer. "
    "Do not write an image prompt, hidden reasoning, or Fluctlight IDs."
)

INITIAL_SCHEDULE_SYSTEM_PROMPT = """Return one JSON object only with items and reschedule_policy.
You are creating a Fluctlight's initial daily Schedule from the supplied identity facts.
Choose the activities and scenes yourself; do not invent unsupported biographical facts.
items must be a non-empty array. Each item must have start_at, end_at, activity, scene,
item_type, status, priority, flexibility, interruption_cost. All numeric values must be
between 0 and 1. Times must use RFC3339 offsets in the requested timezone. The items must
exactly cover the complete requested local date from 00:00:00 to the next 00:00:00 with no
gaps or overlaps. Do not return prose, markdown, hidden reasoning, or a partial schedule."""


def _diagnostic_error_code(exc: Exception, fallback: str) -> str:
    value = str(exc).strip().lower().replace(" ", "_")
    return value[:120] or fallback


class InitializationAnalysisError(RuntimeError):
    def __init__(self, code: str, *, status_code: int) -> None:
        super().__init__(code)
        self.code = code
        self.status_code = status_code


INITIALIZATION_SYSTEM_PROMPT = """Return JSON only. Return exactly this object shape and no
markdown or prose:
{
  "foundation": {
    "identity": {
      "name": null, "age": null, "gender": null, "occupation": null,
      "residence": null, "timezone": null, "birthday": null,
      "background": null, "biography": null, "core_values": [],
      "worldview": null, "notes": null
    },
    "personality": {
      "openness": 0.5, "conscientiousness": 0.5, "extraversion": 0.5,
      "agreeableness": 0.5, "neuroticism": 0.5, "curiosity": 0.5,
      "independence": 0.5, "patience": 0.5, "empathy": 0.5,
      "assertiveness": 0.5, "humor": 0.5, "sociability": 0.5,
      "risk_tolerance": 0.5
    },
    "behavioral_policy": {
      "response_style": null, "message_length": null, "emoji_frequency": 0.0,
      "punctuation_style": null, "humor_style": null, "sarcasm_tendency": 0.0,
      "directness": 0.5, "initiative": 0.5, "topic_initiation": 0.5,
      "silence_tolerance": 0.5, "response_delay": 0.0,
      "emotional_expression": 0.5, "conflict_style": null,
      "refusal_style": null, "intimacy_expression": null
    }
  }
}
Use null for omitted identity and behavioral text facts. timezone must be an IANA timezone such as
Asia/Shanghai, never an offset label such as UTC+8. core_values must be an array of text.
Every personality value and bounded behavioral-policy value must be a finite number from 0 to 1.
response_delay must be a finite number greater than or equal to 0. Do not include identity.id,
personality.update_policy, provenance, hidden reasoning, or extra keys."""


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
            decision = DecisionProposal(
                action_type=decision_payload["action_type"],
                payload=dict(decision_payload.get("payload", {})),
                confidence=float(decision_payload["confidence"]),
                evidence_refs=tuple(decision_payload.get("evidence_refs", (fact.id,))),
                decision_id=str(decision_payload.get("decision_id", f"decision_{fact.id}")),
            )
        except Exception as exc:
            await self._record_model_run(
                assignment=assignment,
                endpoint=endpoint,
                prompt={"messages": messages, "schema_version": "semantic.assessment.v1"},
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
            prompt={"messages": messages, "schema_version": "semantic.assessment.v1"},
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
                prompt={"messages": messages, "schema_version": "fluctlight.initialization.v1"},
                response=None,
                correlation_id=request_id,
                status="failed",
                error_code=code,
            )
            raise InitializationAnalysisError(code, status_code=503) from exc
        payload["provenance"] = {
            "role": ModelRole.INITIALIZATION.value,
            "endpoint_id": endpoint.endpoint_id,
            "model_id": assignment.model_id,
            "prompt_version": "fluctlight.initialization.v1",
            "schema_version": "fluctlight.initialization.v1",
        }
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt={"messages": messages, "schema_version": "fluctlight.initialization.v1"},
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
                        "local_date": local_date.isoformat(),
                        "timezone": timezone,
                    },
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
                prompt={"messages": messages, "schema_version": "life.schedule.initial.v1"},
                response=None,
                correlation_id=correlation_id,
                status="failed",
                error_code=_diagnostic_error_code(exc, "initial_schedule_response_invalid"),
            )
            raise
        await self._record_model_run(
            assignment=assignment,
            endpoint=endpoint,
            prompt={"messages": messages, "schema_version": "life.schedule.initial.v1"},
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
            prompt={"messages": messages, "schema_version": "media.prompt.v1"},
            response=payload,
            correlation_id=correlation_id,
        )
        return prompt.strip()

    @staticmethod
    def _realization_messages(action: FrozenAction) -> list[dict[str, Any]]:
        source_text = action.payload.get("source_text")
        if not isinstance(source_text, str) or not source_text.strip():
            raise RuntimeError("frozen action has no source message")
        return [
            {"role": "system", "content": ACTION_REALIZATION_SYSTEM_PROMPT},
            {
                "role": "user",
                "content": json.dumps(
                    {
                        "user_message": source_text,
                        "action_type": action.action_type.value,
                        "response_intent": action.payload.get("response_intent", {}),
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
            media_messages: list[dict[str, Any]] = [
                {"role": "system", "content": MEDIA_RESPONSE_SYSTEM_PROMPT},
                messages[1],
            ]
            try:
                tool_result = await self._adapter.stream_media_tool_call(
                    assignment,
                    endpoint,
                    secret,
                    messages=media_messages,
                    request_id=action.provider_request_id,
                )
                if not tool_result.arguments:
                    raise RuntimeError("media response is missing media request")
            except Exception as exc:
                await self._record_model_run(
                    assignment=assignment,
                    endpoint=endpoint,
                    prompt={
                        "messages": media_messages,
                        "schema_version": "action.realization.media.v1",
                    },
                    response={"tool_call_failed": True},
                    correlation_id=correlation_id,
                    status="failed",
                    error_code=_diagnostic_error_code(exc, "media_response_invalid"),
                )
                raise
            await self._record_model_run(
                assignment=assignment,
                endpoint=endpoint,
                prompt={
                    "messages": media_messages,
                    "schema_version": "action.realization.media.v1",
                },
                response={
                    "tool_call": "request_media",
                    "argument_keys": sorted(tool_result.arguments),
                },
                correlation_id=correlation_id,
            )
            return RealizationResult(
                action.provider_request_id,
                {
                    "text": tool_result.text.strip() or "我来准备一张给你。",
                    "media_request": tool_result.arguments,
                },
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
            prompt={"messages": messages, "schema_version": "reflection.v1"},
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
