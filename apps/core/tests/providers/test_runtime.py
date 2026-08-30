import asyncio
from typing import Any

import pytest
from fluctlight_core.cognition.contracts import (
    ActionType,
    CognitionFact,
    FrozenAction,
    RealizationResult,
)
from fluctlight_core.cognition.service import CognitionService
from fluctlight_core.providers.contracts import ModelRole
from fluctlight_core.providers.runtime import ConfiguredProviderRuntime
from fluctlight_core.providers.service import ProviderEndpoint, RoleAssignment


class InvalidAssessmentAdapter:
    async def complete_structured(self, *_args, **_kwargs) -> dict[str, object]:
        return {"assessment": {}}


class CompoundAssessmentAdapter:
    async def complete_structured(self, *_args, **_kwargs) -> dict[str, object]:
        return {
            "assessment": {
                "perception": {
                    "event_kind": "conversation.message",
                    "observed_intent": "plan",
                    "sentiment": "positive",
                    "social_signals": [],
                    "environment_meaning": None,
                },
                "appraisal": {
                    "relevance": 0.8,
                    "goal_congruence": 0.8,
                    "reward": 0.7,
                    "loss": 0.1,
                    "social_threat": 0.0,
                    "controllability": 0.8,
                    "responsibility": 0.7,
                    "relationship_significance": 0.8,
                    "expected_effect": 0.7,
                },
                "direction": "positive",
                "strength": 0.7,
                "confidence": 0.9,
            },
            "decision": {
                "effects": [
                    {
                        "id": "reply",
                        "action_type": "reply",
                        "payload": {"response_intent": {"tone": "warm"}},
                    },
                    {
                        "id": "reply-image",
                        "action_type": "media_request",
                        "payload": {
                            "media_request": {
                                "scene": "直播间",
                                "action": "准备直播预告",
                                "mood": "期待",
                                "subject": "主播",
                                "capture_details": "半身构图",
                            }
                        },
                    },
                    {
                        "id": "announcement",
                        "action_type": "moment",
                        "payload": {"response_intent": {"purpose": "直播预告"}},
                    },
                ],
                "confidence": 0.9,
            },
        }


class DailyReviewMediaMismatchAdapter:
    def __init__(self, *, needed: bool, include_concept: bool) -> None:
        self.needed = needed
        self.include_concept = include_concept

    async def complete_structured(self, *_args, **_kwargs) -> dict[str, object]:
        payload: dict[str, object] = {
            "assessment": {
                "perception": {
                    "event_kind": "life_world.daily_review",
                    "observed_intent": "review",
                    "sentiment": "neutral",
                    "social_signals": [],
                    "environment_meaning": None,
                },
                "appraisal": {
                    "relevance": 0.5,
                    "goal_congruence": 0.5,
                    "reward": 0.5,
                    "loss": 0.1,
                    "social_threat": 0.0,
                    "controllability": 0.5,
                    "responsibility": 0.5,
                    "relationship_significance": 0.5,
                    "expected_effect": 0.5,
                },
                "direction": "neutral",
                "strength": 0.5,
                "confidence": 0.9,
            },
            "decision": {
                "effects": [
                    {
                        "id": "moment",
                        "action_type": "moment",
                        "payload": {
                            "response_intent": {"purpose": "daily review"},
                            **(
                                {"moment_media_request": {"subject": "sky"}}
                                if self.include_concept
                                else {}
                            ),
                        },
                    }
                ],
                "confidence": 0.9,
                "media_evaluation": {
                    "needed": self.needed,
                    "reason": "fixture",
                },
            },
        }
        return payload


class DiagnosticsRecorder:
    def __init__(self) -> None:
        self.runs: list[Any] = []

    async def emit_model_run(self, run) -> None:
        self.runs.append(run)


def test_invalid_cognitive_response_records_a_redacted_failed_model_run() -> None:
    runtime = ConfiguredProviderRuntime.__new__(ConfiguredProviderRuntime)
    diagnostics = DiagnosticsRecorder()
    runtime._adapter = InvalidAssessmentAdapter()  # type: ignore[assignment]
    runtime._diagnostics = diagnostics  # type: ignore[assignment]
    runtime._provenance_recorder = None

    async def resolve(_role):
        return (
            RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "local", "model", 100, 30),
            ProviderEndpoint("local", "openai-compatible", "http://provider/v1", "provider:local"),
            None,
        )

    runtime._resolve = resolve  # type: ignore[method-assign]
    fact = CognitionFact(
        id="turn-1",
        fluctlight_id="fluctlight-1",
        event_type="conversation.message",
        payload={"text": "你好🙂"},
        causation_id="cause-1",
        correlation_id="corr-1",
        idempotency_key="turn-1",
    )

    with pytest.raises(RuntimeError, match="missing decision"):
        asyncio.run(runtime.assess(fact, correlation_id="corr-1"))

    assert len(diagnostics.runs) == 1
    run = diagnostics.runs[0]
    assert run.status == "failed"
    assert run.error_code == "cognitive_provider_response_is_missing_decision"
    messages = run.prompt["messages"]
    assert messages[0]["role"] == "system"
    assert "semantic.assessment.v1" in messages[0]["content"]
    assert "at least one source reference" in messages[0]["content"]
    assert "decision_id" not in messages[0]["content"]
    assert "visible reply text" in messages[0]["content"]
    assert "你好🙂" in messages[1]["content"]
    assert "\\u4f60" not in messages[1]["content"]


def test_cognitive_assessment_parses_ordered_compound_effects() -> None:
    runtime = ConfiguredProviderRuntime.__new__(ConfiguredProviderRuntime)
    runtime._adapter = CompoundAssessmentAdapter()  # type: ignore[assignment]
    runtime._diagnostics = None
    runtime._provenance_recorder = None

    async def resolve(_role):
        return (
            RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "local", "model", 100, 30),
            ProviderEndpoint("local", "openai-compatible", "http://provider/v1", "provider:local"),
            None,
        )

    runtime._resolve = resolve  # type: ignore[method-assign]
    fact = CognitionFact(
        id="turn-effects",
        fluctlight_id="fluctlight-1",
        event_type="conversation.message",
        payload={"text": "今晚直播要不要做预告？"},
        causation_id="cause-effects",
        correlation_id="corr-effects",
        idempotency_key="turn-effects",
    )

    envelope = asyncio.run(runtime.assess(fact, correlation_id="corr-effects"))

    assert [effect.action_type.value for effect in envelope.decision.effects] == [
        "reply",
        "media_request",
        "moment",
    ]
    assert envelope.decision.effects[1].payload["media_request"]["scene"] == "直播间"


def test_daily_review_missing_optional_media_does_not_fail_the_moment() -> None:
    runtime = ConfiguredProviderRuntime.__new__(ConfiguredProviderRuntime)
    runtime._adapter = DailyReviewMediaMismatchAdapter(needed=True, include_concept=False)
    runtime._diagnostics = None
    runtime._provenance_recorder = None

    async def resolve(_role):
        return (
            RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "local", "model", 100, 30),
            ProviderEndpoint("local", "openai-compatible", "http://provider/v1", "provider:local"),
            None,
        )

    runtime._resolve = resolve  # type: ignore[method-assign]
    fact = CognitionFact(
        id="daily-review-media-missing",
        fluctlight_id="fluctlight-1",
        event_type="life_world.daily_review",
        payload={"background_context": {"conversation_id": "conversation-1"}},
        causation_id="schedule-1",
        correlation_id="corr-daily-review",
        idempotency_key="daily-review-media-missing",
    )

    envelope = asyncio.run(runtime.assess(fact, correlation_id="corr-daily-review"))

    assert envelope.decision.action_type is ActionType.MOMENT
    assert "moment_media_request" not in envelope.decision.payload


def test_daily_review_contradictory_media_is_dropped_without_failing_the_moment() -> None:
    runtime = ConfiguredProviderRuntime.__new__(ConfiguredProviderRuntime)
    runtime._adapter = DailyReviewMediaMismatchAdapter(needed=False, include_concept=True)
    runtime._diagnostics = None
    runtime._provenance_recorder = None

    async def resolve(_role):
        return (
            RoleAssignment(ModelRole.COGNITIVE_ASSESSMENT, "local", "model", 100, 30),
            ProviderEndpoint("local", "openai-compatible", "http://provider/v1", "provider:local"),
            None,
        )

    runtime._resolve = resolve  # type: ignore[method-assign]
    fact = CognitionFact(
        id="daily-review-media-contradictory",
        fluctlight_id="fluctlight-1",
        event_type="life_world.daily_review",
        payload={"background_context": {"conversation_id": "conversation-1"}},
        causation_id="schedule-1",
        correlation_id="corr-daily-review-2",
        idempotency_key="daily-review-media-contradictory",
    )

    envelope = asyncio.run(runtime.assess(fact, correlation_id="corr-daily-review-2"))

    assert envelope.decision.action_type is ActionType.MOMENT
    assert "moment_media_request" not in envelope.decision.payload


def test_realization_uses_the_factual_source_message_not_cognitive_payload_text() -> None:
    action = FrozenAction(
        action_id="action-1",
        decision_id="decision-1",
        inbox_id="inbox-1",
        fluctlight_id="fluctlight-1",
        action_type=ActionType.REPLY,
        payload={
            "source_text": "请告诉我现在的状态",
            "text": "this must never become the realization input",
        },
        state_revision=1,
        provider_request_id="request-1",
    )

    messages = ConfiguredProviderRuntime._realization_messages(action)

    content = str(messages[1]["content"])
    assert "请告诉我现在的状态" in content
    assert "this must never become the realization input" not in content


def test_realization_receives_the_frozen_persona_expression_profile() -> None:
    action = FrozenAction(
        action_id="action-1",
        decision_id="decision-1",
        inbox_id="inbox-1",
        fluctlight_id="fluctlight-1",
        action_type=ActionType.REPLY,
        payload={
            "source_text": "今天过得怎么样？",
            "persona_profile": {
                "personality": {"humor": 0.7, "empathy": 0.8},
                "behavioral_policy": {
                    "response_style": "简洁、温和",
                    "punctuation_style": "自然，不滥用感叹号",
                    "emoji_frequency": 0.1,
                },
            },
        },
        state_revision=1,
        provider_request_id="request-1",
    )

    messages = ConfiguredProviderRuntime._realization_messages(action)

    content = str(messages[1]["content"])
    assert '"persona_profile"' in content
    assert "简洁、温和" in content
    assert '"humor": 0.7' in content


def test_background_realization_uses_frozen_daily_context_without_a_user_message() -> None:
    action = FrozenAction(
        action_id="action-background",
        decision_id="decision-background",
        inbox_id="background-1",
        fluctlight_id="fluctlight-1",
        action_type=ActionType.MOMENT,
        payload={
            "background_context": {"kind": "daily_schedule_ready", "local_date": "2026-08-27"},
            "persona_profile": {"behavioral_policy": {"response_style": "简洁"}},
        },
        state_revision=1,
        provider_request_id="request-background",
    )

    messages = ConfiguredProviderRuntime._realization_messages(action)

    content = str(messages[1]["content"])
    assert '"background_context"' in content
    assert "daily_schedule_ready" in content


def test_moment_realization_keeps_the_frozen_image_concept_for_later_media_execution() -> None:
    action = FrozenAction(
        action_id="action-moment",
        decision_id="decision-moment",
        inbox_id="background-1",
        fluctlight_id="fluctlight-1",
        action_type=ActionType.MOMENT,
        payload={
            "background_context": {"kind": "daily_schedule_ready"},
            "moment_media_request": {"subject": "窗边的照片和咖啡"},
        },
        state_revision=1,
        provider_request_id="request-moment",
    )
    result = RealizationResult(action.provider_request_id, {"text": "今天想留下一点光。"})

    finalized = CognitionService._action_after_realization(action, result)

    assert finalized.payload["text"] == "今天想留下一点光。"
    assert finalized.payload["moment_media_request"] == {"subject": "窗边的照片和咖啡"}
