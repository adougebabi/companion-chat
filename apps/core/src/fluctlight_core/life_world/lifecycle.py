"""Durable registration for each Fluctlight's local-day Schedule lifecycle."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fluctlight_core.fluctlights.contracts import FluctlightSnapshot
from fluctlight_core.platform.outbox import CommittedWorkflowIntent, commit_workflow_intent
from fluctlight_core.platform.persistence import UnitOfWork, UnitOfWorkFactory


class ScheduleLifecycleRegistrar:
    """Commit one stable Temporal lifecycle request per Fluctlight."""

    def __init__(self, unit_of_work: UnitOfWorkFactory) -> None:
        self._unit_of_work = unit_of_work

    @asynccontextmanager
    async def _transaction(
        self, tx: UnitOfWork | None, command_id: str
    ) -> AsyncIterator[UnitOfWork]:
        if tx is not None:
            yield tx
            return
        async with self._unit_of_work.begin(command_id=command_id) as owned:
            yield owned
            await owned.commit()

    async def register(
        self, fluctlight_id: str, *, tx: UnitOfWork | None = None
    ) -> CommittedWorkflowIntent:
        intent = schedule_lifecycle_intent(fluctlight_id)
        async with self._transaction(
            tx, f"schedule-lifecycle:{intent.payload['fluctlight_id']}"
        ) as current:
            return await commit_workflow_intent(current.session, intent)

    async def register_active(self, fluctlights: list[FluctlightSnapshot]) -> int:
        registered = 0
        for fluctlight in fluctlights:
            await self.register(fluctlight.id)
            registered += 1
        return registered


def schedule_lifecycle_intent(fluctlight_id: str) -> CommittedWorkflowIntent:
    if not isinstance(fluctlight_id, str) or not fluctlight_id.strip():
        raise ValueError("schedule lifecycle requires fluctlight_id")
    resolved_id = fluctlight_id.strip()
    return CommittedWorkflowIntent(
        intent_id=f"schedule_intent:{resolved_id}",
        workflow_id=f"schedule:{resolved_id}",
        task_queue="lifecycle",
        intent_type="schedule.current_day",
        payload={"fluctlight_id": resolved_id},
    )
