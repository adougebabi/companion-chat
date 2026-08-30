"""Dispatch committed PostgreSQL workflow intents to the sole Temporal runtime."""

from __future__ import annotations

import logging
from typing import Any

from sqlalchemy import case, select
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
        started = 0
        batch_size = max(1, limit)
        async with self.unit_of_work.begin(command_id="workflow-dispatch") as tx:
            statement = select(workflow_intents)
            if self._started:
                statement = statement.where(
                    workflow_intents.c.intent_id.not_in(tuple(self._started))
                )
            rows = (
                (
                    await tx.session.execute(
                        statement
                        .order_by(
                            case(
                                (workflow_intents.c.intent_type.like("daily_review.%"), 0),
                                (workflow_intents.c.intent_type.like("schedule.%"), 1),
                                (workflow_intents.c.intent_type.like("autonomy.%"), 2),
                                (workflow_intents.c.intent_type.like("media.%"), 3),
                                (workflow_intents.c.intent_type.like("reflection.%"), 4),
                                else_=5,
                            ),
                            workflow_intents.c.created_at,
                        )
                        .limit(batch_size)
                    )
                )
                .mappings()
                .all()
            )
        # One pass is deliberately bounded. The old offset loop eventually
        # scanned the entire intent table on every tick, starving fresh
        # lifecycle work behind historical reflection/media rows.
        for row in rows:
            intent_id = str(row["intent_id"])
            if intent_id in self._started:
                continue
            task_queue = str(row["task_queue"])
            if task_queue not in TASK_QUEUES:
                logger.error(
                    "workflow.dispatch.unsupported_queue intent_id=%s task_queue=%s",
                    intent_id,
                    task_queue,
                )
                continue
            workflow = self.workflows.get(
                str(row["intent_type"]).split(".", 1)[0], self.workflows["platform"]
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
                # Stable workflow_id is the durable replay guard. A restarted
                # Worker may lose `_started`, but Temporal still rejects the
                # duplicate and the intent is considered dispatched.
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
                started += 1
            self._started.add(intent_id)
        return started
