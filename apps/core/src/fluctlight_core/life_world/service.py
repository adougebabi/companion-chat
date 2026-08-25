"""Life World Event/Schedule persistence and explicit Context authority."""

from __future__ import annotations

from datetime import date, datetime

from sqlalchemy import insert, select, update

from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema
from .contracts import (
    ContextSnapshot,
    ContextSource,
    ScheduleStatus,
    ScheduleValidationError,
    ScheduleVersion,
    WorldEvent,
    timezone_or_error,
)


class LifeWorldService:
    def __init__(self, unit_of_work: UnitOfWorkFactory) -> None:
        self._unit_of_work = unit_of_work

    def propose_schedule(self, proposal: ScheduleVersion) -> ScheduleVersion:
        proposal.validate_full_day()
        return proposal

    async def create_event(self, event: WorldEvent) -> WorldEvent:
        async with self._unit_of_work.begin(command_id=f"life-event:{event.id}") as tx:
            await tx.session.execute(
                insert(schema.events).values(
                    id=event.id,
                    fluctlight_id=event.fluctlight_id,
                    kind=event.kind,
                    start_at=event.start_at,
                    end_at=event.end_at,
                    scene=event.scene,
                    activity=event.activity,
                    location=event.location,
                    status=event.status.value,
                    evidence_refs=list(event.evidence_refs),
                )
            )
            await tx.commit()
        return event

    async def accept_schedule(self, proposal: ScheduleVersion) -> ScheduleVersion:
        proposal.validate_full_day()
        accepted = ScheduleVersion(
            id=proposal.id,
            fluctlight_id=proposal.fluctlight_id,
            local_date=proposal.local_date,
            timezone=proposal.timezone,
            items=proposal.items,
            generated_from=proposal.generated_from,
            evidence_refs=proposal.evidence_refs,
            status=ScheduleStatus.ACCEPTED,
            previous_version_id=proposal.previous_version_id,
            revision=proposal.revision,
            generated_at=proposal.generated_at,
            reschedule_policy=proposal.reschedule_policy,
        )
        async with self._unit_of_work.begin(command_id=f"schedule-accept:{accepted.id}") as tx:
            previous = (
                (
                    await tx.session.execute(
                        select(schema.schedules)
                        .where(
                            schema.schedules.c.fluctlight_id == accepted.fluctlight_id,
                            schema.schedules.c.local_date == accepted.local_date,
                            schema.schedules.c.status == ScheduleStatus.ACCEPTED.value,
                        )
                        .with_for_update()
                    )
                )
                .mappings()
                .all()
            )
            for row in previous:
                await tx.session.execute(
                    update(schema.schedules)
                    .where(schema.schedules.c.id == row["id"])
                    .values(status=ScheduleStatus.SUPERSEDED.value)
                )
            await tx.session.execute(
                insert(schema.schedules).values(
                    id=accepted.id,
                    fluctlight_id=accepted.fluctlight_id,
                    local_date=accepted.local_date,
                    timezone=accepted.timezone,
                    status=accepted.status.value,
                    generated_from=accepted.generated_from,
                    evidence_refs=list(accepted.evidence_refs),
                    previous_version_id=accepted.previous_version_id
                    or (previous[-1]["id"] if previous else None),
                    revision=accepted.revision,
                    generated_at=accepted.generated_at,
                    reschedule_policy=dict(accepted.reschedule_policy),
                )
            )
            for item in accepted.items:
                await tx.session.execute(
                    insert(schema.schedule_items).values(
                        id=item.id,
                        schedule_id=accepted.id,
                        start_at=item.start_at,
                        end_at=item.end_at,
                        activity=item.activity,
                        scene=item.scene,
                        item_type=item.item_type,
                        status=item.status,
                        priority=str(item.priority),
                        flexibility=str(item.flexibility),
                        interruption_cost=str(item.interruption_cost),
                    )
                )
            await tx.commit()
        return accepted

    async def replan(
        self, proposal: ScheduleVersion, *, completed_before: datetime | None = None
    ) -> ScheduleVersion:
        proposal.validate_full_day()
        if completed_before is not None:
            for item in proposal.items:
                if item.end_at <= completed_before:
                    continue
                if item.start_at < completed_before:
                    raise ScheduleValidationError(
                        "replan cannot rewrite a completed schedule segment"
                    )
        return await self.accept_schedule(proposal)

    async def resolve_context(self, fluctlight_id: str, instant: datetime) -> ContextSnapshot:
        async with self._unit_of_work.begin(command_id=f"context:{fluctlight_id}") as tx:
            events = (
                (
                    await tx.session.execute(
                        select(schema.events)
                        .where(
                            schema.events.c.fluctlight_id == fluctlight_id,
                            schema.events.c.start_at <= instant,
                            schema.events.c.end_at > instant,
                            schema.events.c.status == "confirmed",
                        )
                        .order_by(schema.events.c.start_at.desc())
                    )
                )
                .mappings()
                .all()
            )
            if events:
                row = events[0]
                return ContextSnapshot(
                    fluctlight_id,
                    ContextSource.EVENT,
                    instant,
                    event_id=row["id"],
                    scene=row["scene"],
                    activity=row["activity"],
                    location=row["location"],
                )
            schedule_row = (
                (
                    await tx.session.execute(
                        select(schema.schedules)
                        .where(
                            schema.schedules.c.fluctlight_id == fluctlight_id,
                            schema.schedules.c.status == ScheduleStatus.ACCEPTED.value,
                        )
                        .order_by(schema.schedules.c.generated_at.desc())
                    )
                )
                .mappings()
                .all()
            )
            for schedule in schedule_row:
                zone = timezone_or_error(schedule["timezone"])
                if instant.astimezone(zone).date() != schedule["local_date"]:
                    continue
                item = (
                    (
                        await tx.session.execute(
                            select(schema.schedule_items).where(
                                schema.schedule_items.c.schedule_id == schedule["id"],
                                schema.schedule_items.c.start_at <= instant,
                                schema.schedule_items.c.end_at > instant,
                            )
                        )
                    )
                    .mappings()
                    .one_or_none()
                )
                if item is not None:
                    return ContextSnapshot(
                        fluctlight_id,
                        ContextSource.SCHEDULE,
                        instant,
                        schedule_id=schedule["id"],
                        scene=item["scene"],
                        activity=item["activity"],
                    )
                break
        return ContextSnapshot(fluctlight_id, ContextSource.PENDING, instant)

    async def change_timezone(
        self, fluctlight_id: str, timezone: str, *, effective_date: date
    ) -> int:
        timezone_or_error(timezone)
        async with self._unit_of_work.begin(
            command_id=f"timezone:{fluctlight_id}:{timezone}"
        ) as tx:
            result = await tx.session.execute(
                update(schema.schedules)
                .where(
                    schema.schedules.c.fluctlight_id == fluctlight_id,
                    schema.schedules.c.local_date >= effective_date,
                    schema.schedules.c.status == ScheduleStatus.ACCEPTED.value,
                )
                .values(status=ScheduleStatus.SUPERSEDED.value)
            )
            await tx.commit()
        return int(result.rowcount or 0)
