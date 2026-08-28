"""Dispatch committed PostgreSQL workflow intents to the sole Temporal runtime."""

from __future__ import annotations

import logging
from typing import Any

from sqlalchemy import select
from temporalio.exceptions import WorkflowAlreadyStartedError

from .schema import workflow_intents
from .temporal import TASK_QUEUES

logger = logging.getLogger(__name__)

class CommittedIntentDispatcher:
    """At-least-once dispatcher; stable Workflow IDs make retries idempotent."""

    def __init__(self, client: Any, unit_of_work: Any, workflows: dict[str, Any]) -> None:
        self.client = client
        self.unit_of_work = unit_of_work
        self.workflows = workflows
        self._started: set[str] = set()

    async def dispatch_once(self, *, limit: int = 20) -> int:
        async with self.unit_of_work.begin(command_id="workflow-dispatch") as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(workflow_intents)
                        .order_by(workflow_intents.c.created_at)
                        .limit(limit)
                    )
                )
                .mappings()
                .all()
            )
        started = 0
        for row in rows:
            intent_id = row["intent_id"]
            if intent_id in self._started:
                continue
            task_queue = row["task_queue"]
            if task_queue not in TASK_QUEUES:
                continue
            workflow = self.workflows.get(
                row["intent_type"].split(".", 1)[0], self.workflows["platform"]
            )
            payload = dict(row["payload"])
            payload.setdefault("intent_id", intent_id)
            logger.warning(
                "workflow.dispatch.start intent_id=%s workflow_id=%s intent_type=%s "
                "task_queue=%s",
                intent_id,
                row["workflow_id"],
                row["intent_type"],
                task_queue,
            )
            try:
                await self.client.start_workflow(
                    workflow,
                    payload,
                    id=row["workflow_id"],
                    task_queue=task_queue,
                )
            except WorkflowAlreadyStartedError:
                logger.warning(
                    "workflow.dispatch.already_started intent_id=%s workflow_id=%s",
                    intent_id,
                    row["workflow_id"],
                )
            except Exception:
                logger.exception(
                    "workflow.dispatch.failed intent_id=%s workflow_id=%s intent_type=%s",
                    intent_id,
                    row["workflow_id"],
                    row["intent_type"],
                )
                continue
            else:
                logger.warning(
                    "workflow.dispatch.started intent_id=%s workflow_id=%s intent_type=%s",
                    intent_id,
                    row["workflow_id"],
                    row["intent_type"],
                )
            self._started.add(intent_id)
            started += 1
        return started
