"""Moments persistence with explicit visibility and Actor authorization."""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

from sqlalchemy import delete, insert, select, update

from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema
from .contracts import (
    Moment,
    MomentComment,
    MomentReaction,
    MomentStatus,
    MomentVisibility,
)


class MomentsService:
    def __init__(self, unit_of_work: UnitOfWorkFactory) -> None:
        self._unit_of_work = unit_of_work

    async def create(self, moment: Moment) -> Moment:
        async with self._unit_of_work.begin(command_id=f"moment-create:{moment.id}") as tx:
            await tx.session.execute(
                insert(schema.moments).values(
                    id=moment.id,
                    owner_fluctlight_id=moment.owner_fluctlight_id,
                    author_actor_id=moment.author_actor_id,
                    text=moment.text,
                    visibility=moment.visibility.value,
                    status=moment.status.value,
                    media_asset_ids=list(moment.media_asset_ids),
                    created_at=moment.created_at,
                )
            )
            await tx.commit()
        return moment

    async def feed(
        self, *, owner_fluctlight_id: str, actor_id: str, limit: int = 50
    ) -> list[Moment]:
        async with self._unit_of_work.begin(
            command_id=f"moment-feed:{owner_fluctlight_id}:{actor_id}"
        ) as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.moments)
                        .where(
                            schema.moments.c.owner_fluctlight_id == owner_fluctlight_id,
                            schema.moments.c.status == MomentStatus.VISIBLE.value,
                            schema.moments.c.visibility.in_(
                                [MomentVisibility.OWNER.value, MomentVisibility.PARTICIPANTS.value]
                            ),
                        )
                        .order_by(schema.moments.c.created_at.desc())
                        .limit(min(max(limit, 1), 200))
                    )
                )
                .mappings()
                .all()
            )
        return [
            self._from_row(row)
            for row in rows
            if row["author_actor_id"] == actor_id
            or row["visibility"] == MomentVisibility.PARTICIPANTS.value
        ]

    async def comment(self, comment: MomentComment, *, actor_id: str) -> MomentComment:
        if comment.author_actor_id != actor_id:
            raise PermissionError("comment author must match resolved Actor")
        async with self._unit_of_work.begin(command_id=f"moment-comment:{comment.id}") as tx:
            moment = await tx.session.scalar(
                select(schema.moments.c.id).where(
                    schema.moments.c.id == comment.moment_id,
                    schema.moments.c.status == MomentStatus.VISIBLE.value,
                )
            )
            if moment is None:
                raise KeyError(comment.moment_id)
            await tx.session.execute(
                insert(schema.comments).values(
                    id=comment.id,
                    moment_id=comment.moment_id,
                    author_actor_id=comment.author_actor_id,
                    text=comment.text,
                    created_at=comment.created_at,
                )
            )
            await tx.commit()
        return comment

    async def react(self, reaction: MomentReaction, *, actor_id: str) -> MomentReaction:
        if reaction.actor_id != actor_id:
            raise PermissionError("reaction Actor mismatch")
        async with self._unit_of_work.begin(
            command_id=f"moment-reaction:{reaction.moment_id}:{actor_id}"
        ) as tx:
            await tx.session.execute(
                delete(schema.reactions).where(
                    schema.reactions.c.moment_id == reaction.moment_id,
                    schema.reactions.c.actor_id == actor_id,
                )
            )
            await tx.session.execute(
                insert(schema.reactions).values(
                    moment_id=reaction.moment_id,
                    actor_id=actor_id,
                    kind=reaction.kind.value,
                    created_at=reaction.created_at,
                )
            )
            await tx.commit()
        return reaction

    async def hide(self, moment_id: str, *, actor_id: str) -> None:
        async with self._unit_of_work.begin(command_id=f"moment-hide:{moment_id}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.moments)
                        .where(schema.moments.c.id == moment_id)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None or row["author_actor_id"] != actor_id:
                raise PermissionError("only the Moment author may hide it")
            await tx.session.execute(
                update(schema.moments)
                .where(schema.moments.c.id == moment_id)
                .values(status=MomentStatus.HIDDEN.value)
            )
            await tx.commit()

    async def mark_read(
        self, owner_fluctlight_id: str, actor_id: str, *, seen_at: datetime | None = None
    ) -> None:
        now = seen_at or datetime.now(UTC)
        async with self._unit_of_work.begin(
            command_id=f"moment-read:{owner_fluctlight_id}:{actor_id}"
        ) as tx:
            existing = (
                (
                    await tx.session.execute(
                        select(schema.unread_markers).where(
                            schema.unread_markers.c.owner_fluctlight_id == owner_fluctlight_id,
                            schema.unread_markers.c.actor_id == actor_id,
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is None:
                await tx.session.execute(
                    insert(schema.unread_markers).values(
                        owner_fluctlight_id=owner_fluctlight_id, actor_id=actor_id, last_seen_at=now
                    )
                )
            else:
                await tx.session.execute(
                    update(schema.unread_markers)
                    .where(
                        schema.unread_markers.c.owner_fluctlight_id == owner_fluctlight_id,
                        schema.unread_markers.c.actor_id == actor_id,
                    )
                    .values(last_seen_at=now)
                )
            await tx.commit()

    @staticmethod
    def _from_row(row: Any) -> Moment:
        return Moment(
            row["id"],
            row["owner_fluctlight_id"],
            row["author_actor_id"],
            row["text"],
            row["visibility"],
            row["status"],
            tuple(row["media_asset_ids"] or ()),
            row["created_at"],
        )
