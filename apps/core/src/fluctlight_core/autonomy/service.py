"""Python-owned autonomy freeze, governance and reconciliation."""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import replace
from datetime import UTC, datetime
from typing import Any

from sqlalchemy import insert, select, update

from fluctlight_core.life_world.contracts import (
    ActionStatus,
    AutonomousActionRequest,
    AutonomyDecision,
    FrozenAutonomousAction,
)
from fluctlight_core.platform.outbox import CommittedWorkflowIntent, commit_workflow_intent
from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema


class AutonomyService:
    def __init__(
        self,
        unit_of_work: UnitOfWorkFactory,
        *,
        status_resolver: Callable[[str], Awaitable[str]] | None = None,
    ) -> None:
        self._unit_of_work = unit_of_work
        self._status_resolver = status_resolver

    async def _activity_allowed(self, fluctlight_id: str) -> tuple[bool, str]:
        if self._status_resolver is None:
            return True, "active"
        status = await self._status_resolver(fluctlight_id)
        if status == "active":
            return True, status
        return False, f"fluctlight_{status}"

    async def freeze_action(self, request: AutonomousActionRequest) -> AutonomyDecision:
        active, status_reason = await self._activity_allowed(request.fluctlight_id)
        if not active:
            return AutonomyDecision(False, status_reason)
        allowed, reason = request.policy.allows(
            request.action_type, request.requested_at, request.cost
        )
        if not allowed:
            return AutonomyDecision(False, reason)
        action = FrozenAutonomousAction(
            id=request.action_id,
            fluctlight_id=request.fluctlight_id,
            action_type=request.action_type,
            payload=dict(request.payload),
            policy_snapshot={
                "mode": request.policy.mode,
                "allowed_actions": sorted(request.policy.allowed_actions),
                "budget_remaining": request.policy.budget_remaining,
                "cost": request.cost,
            },
            expected_revisions=dict(request.expected_revisions),
        )
        async with self._unit_of_work.begin(command_id=f"autonomy-freeze:{action.id}") as tx:
            existing = (
                (
                    await tx.session.execute(
                        select(schema.actions).where(schema.actions.c.id == action.id)
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                return AutonomyDecision(True, "idempotent_replay", self._from_row(existing))
            await tx.session.execute(
                insert(schema.actions).values(
                    id=action.id,
                    fluctlight_id=action.fluctlight_id,
                    action_type=action.action_type,
                    payload=dict(action.payload),
                    policy_snapshot=dict(action.policy_snapshot),
                    expected_revisions=dict(action.expected_revisions),
                    status=action.status.value,
                    workflow_id=action.workflow_id,
                    provider_request_id=action.provider_request_id,
                    created_at=action.created_at,
                )
            )
            await commit_workflow_intent(
                tx.session,
                CommittedWorkflowIntent(
                    intent_id=f"autonomy_intent:{action.id}",
                    workflow_id=action.workflow_id,
                    task_queue="lifecycle",
                    intent_type="autonomy.action",
                    payload={"action_id": action.id, "action_type": action.action_type},
                ),
            )
            await tx.commit()
        return AutonomyDecision(True, "frozen", action)

    async def get_action(self, action_id: str) -> FrozenAutonomousAction:
        async with self._unit_of_work.begin(command_id=f"autonomy-read:{action_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.actions).where(schema.actions.c.id == action_id)
                    )
                )
                .mappings()
                .one_or_none()
            )
        if row is None:
            raise KeyError(action_id)
        return self._from_row(row)

    async def list_for_fluctlight(
        self, fluctlight_id: str, *, limit: int = 100
    ) -> list[FrozenAutonomousAction]:
        async with self._unit_of_work.begin(command_id=f"autonomy-list:{fluctlight_id}") as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.actions)
                        .where(schema.actions.c.fluctlight_id == fluctlight_id)
                        .order_by(schema.actions.c.created_at.desc())
                        .limit(min(max(limit, 1), 200))
                    )
                )
                .mappings()
                .all()
            )
        return [self._from_row(row) for row in rows]

    async def execute(self, action_id: str, executor: Any) -> FrozenAutonomousAction:
        action = await self.get_action(action_id)
        if action.status not in {ActionStatus.FROZEN, ActionStatus.DEFERRED}:
            return action
        active, status_reason = await self._activity_allowed(action.fluctlight_id)
        if not active:
            return await self.govern(
                action.id,
                to_status=ActionStatus.DEFERRED,
                actor_id=action.fluctlight_id,
                reason=status_reason,
            )
        try:
            result = await executor.execute(action)
        except Exception as exc:
            return await self.govern(
                action.id,
                to_status=ActionStatus.FAILED,
                actor_id=action.fluctlight_id,
                reason=self._executor_failure_reason(exc),
            )
        return await self.govern(
            action.id,
            to_status=result.status,
            actor_id=action.fluctlight_id,
            reason=result.reason,
        )

    async def govern(
        self, action_id: str, *, to_status: ActionStatus, actor_id: str, reason: str
    ) -> FrozenAutonomousAction:
        if not reason.strip():
            raise ValueError("governance requires a reason")
        now = datetime.now(UTC)
        async with self._unit_of_work.begin(command_id=f"autonomy-govern:{action_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.actions)
                        .where(schema.actions.c.id == action_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None:
                raise KeyError(action_id)
            current = self._from_row(row)
            if current.status is ActionStatus.COMPLETED and to_status is not ActionStatus.COMPLETED:
                raise ValueError("completed action cannot be governed backwards")
            await tx.session.execute(
                update(schema.actions)
                .where(schema.actions.c.id == action_id)
                .values(
                    status=to_status.value,
                    settled_at=now
                    if to_status
                    in {ActionStatus.CANCELLED, ActionStatus.COMPLETED, ActionStatus.FAILED}
                    else None,
                )
            )
            await tx.session.execute(
                insert(schema.governance).values(
                    id=f"autonomy_governance_{action_id}_{now.timestamp()}",
                    fluctlight_id=current.fluctlight_id,
                    action_id=action_id,
                    from_status=current.status.value,
                    to_status=to_status.value,
                    actor_id=actor_id,
                    reason=reason,
                    created_at=now,
                )
            )
            await tx.commit()
        return replace(current, status=to_status)

    async def reconcile(self, *, fluctlight_id: str, policy: Any) -> list[FrozenAutonomousAction]:
        async with self._unit_of_work.begin(command_id=f"autonomy-reconcile:{fluctlight_id}") as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.actions).where(
                            schema.actions.c.fluctlight_id == fluctlight_id,
                            schema.actions.c.status.in_(
                                [ActionStatus.FROZEN.value, ActionStatus.DEFERRED.value]
                            ),
                        )
                    )
                )
                .mappings()
                .all()
            )
        return [self._from_row(row) for row in rows]

    @staticmethod
    def _executor_failure_reason(exc: Exception) -> str:
        detail = str(exc).strip().replace("\n", " ")
        if len(detail) > 240:
            detail = detail[:240] + "..."
        return (
            f"executor_{type(exc).__name__}:{detail}"
            if detail
            else f"executor_{type(exc).__name__}"
        )

    @staticmethod
    def _from_row(row: Any) -> FrozenAutonomousAction:
        return FrozenAutonomousAction(
            id=row["id"],
            fluctlight_id=row["fluctlight_id"],
            action_type=row["action_type"],
            payload=dict(row["payload"]),
            policy_snapshot=dict(row["policy_snapshot"]),
            expected_revisions=dict(row["expected_revisions"]),
            status=ActionStatus(row["status"]),
            workflow_id=row["workflow_id"],
            provider_request_id=row["provider_request_id"],
            created_at=row["created_at"],
        )
