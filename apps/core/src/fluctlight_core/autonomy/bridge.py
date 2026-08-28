"""Bridge committed cognitive decisions into the autonomous-action lifecycle."""

from __future__ import annotations

import logging
from datetime import UTC, datetime
from typing import Any

from fluctlight_core.cognition.contracts import ActionType, FrozenAction
from fluctlight_core.diagnostics.contracts import DiagnosticEvent, DiagnosticSeverity
from fluctlight_core.diagnostics.service import DiagnosticsService
from fluctlight_core.life_world.contracts import AutonomousActionRequest, AutonomyPolicy
from fluctlight_core.settings.service import SettingsService

from .service import AutonomyService

logger = logging.getLogger(__name__)


class CognitionAutonomyBridge:
    """Route only explicit decision types; never infer an action from text."""

    _ACTION_TYPES = frozenset(
        {
            ActionType.PROACTIVE_MESSAGE,
            ActionType.MOMENT,
            ActionType.MEMORY_CANDIDATE,
            ActionType.RELATIONSHIP_CANDIDATE,
            ActionType.MEDIA_REQUEST,
            ActionType.SCHEDULE_PROPOSAL,
        }
    )

    def __init__(
        self,
        autonomy: AutonomyService,
        settings: SettingsService,
        diagnostics: DiagnosticsService | None = None,
    ) -> None:
        self._autonomy = autonomy
        self._settings = settings
        self._diagnostics = diagnostics

    async def __call__(self, action: FrozenAction) -> None:
        if action.action_type not in self._ACTION_TYPES:
            return
        decision = await self._autonomy.freeze_action(
            AutonomousActionRequest(
                fluctlight_id=action.fluctlight_id,
                action_id=f"autonomy_{action.action_id}",
                action_type=action.action_type.value,
                payload=dict(action.payload),
                policy=await self._policy(),
                expected_revisions={"cognition": action.state_revision},
                cost=self._cost(action.payload),
                requested_at=datetime.now(UTC),
            )
        )
        if decision is not None:
            logger.warning(
                "cognition.autonomy_action.%s action_id=%s action_type=%s reason=%s",
                "accepted" if decision.accepted else "rejected",
                f"autonomy_{action.action_id}",
                action.action_type.value,
                decision.reason_code,
            )
        if self._diagnostics is not None and decision is not None:
            await self._diagnostics.emit_event(
                DiagnosticEvent(
                    event_type=(
                        "cognition.autonomy_action_frozen"
                        if decision.accepted
                        else "cognition.autonomy_action_rejected"
                    ),
                    severity=(
                        DiagnosticSeverity.INFO
                        if decision.accepted
                        else DiagnosticSeverity.WARNING
                    ),
                    fluctlight_id=action.fluctlight_id,
                    correlation_id=action.inbox_id,
                    causation_id=action.inbox_id,
                    payload={
                        "action_id": f"autonomy_{action.action_id}",
                        "action_type": action.action_type.value,
                        "accepted": decision.accepted,
                        "reason_code": decision.reason_code,
                    },
                )
            )

    async def _policy(self) -> AutonomyPolicy:
        value = await self._settings.runtime_value("product.autonomy")
        if value is None:
            return AutonomyPolicy()
        if not isinstance(value, dict):
            raise ValueError("product.autonomy must be an object")
        allowed_actions = value.get("allowed_actions")
        if allowed_actions is not None and (
            not isinstance(allowed_actions, list)
            or not all(isinstance(item, str) for item in allowed_actions)
        ):
            raise ValueError("product.autonomy.allowed_actions must be text")
        budget_remaining = value.get("budget_remaining", 1.0)
        if not isinstance(budget_remaining, int | float):
            raise ValueError("product.autonomy.budget_remaining must be numeric")
        return AutonomyPolicy(
            mode=str(value.get("mode", "active")),
            allowed_actions=(
                frozenset(allowed_actions)
                if allowed_actions is not None
                else AutonomyPolicy().allowed_actions
            ),
            budget_remaining=float(budget_remaining),
        )

    @staticmethod
    def _cost(payload: dict[str, Any] | Any) -> float:
        value = payload.get("cost", 0.0) if isinstance(payload, dict) else 0.0
        if not isinstance(value, int | float):
            raise ValueError("autonomous action cost must be numeric")
        return float(value)
