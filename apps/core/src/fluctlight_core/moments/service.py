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
    ReactionKind,
)


class MomentsService:
    def __init__(self, unit_of_work: UnitOfWorkFactory) -> None:
        self._unit_of_work = unit_of_work

    async def create(self, moment: Moment) -> Moment:
        async with self._unit_of_work.begin(command_id=f"moment-create:{moment.id}") as tx:
            existing = (
                (
                    await tx.session.execute(
                        select(schema.moments).where(schema.moments.c.id == moment.id)
                    )
                )
                .mappings()
                .one_or_none()
            )
            if existing is not None:
                persisted = self._from_row(existing)
                if (
                    persisted.owner_fluctlight_id != moment.owner_fluctlight_id
                    or persisted.author_actor_id != moment.author_actor_id
                    or persisted.text != moment.text
                ):
                    raise ValueError("moment ID was reused with different authoritative content")
                return persisted
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
        self,
        *,
        owner_fluctlight_id: str,
        actor_id: str,
        limit: int = 50,
        include_hidden: bool = False,
    ) -> list[Moment]:
        async with self._unit_of_work.begin(
            command_id=f"moment-feed:{owner_fluctlight_id}:{actor_id}"
        ) as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.moments)
                        .where(schema.moments.c.owner_fluctlight_id == owner_fluctlight_id)
                        .where(
                            schema.moments.c.visibility.in_(
                                [MomentVisibility.OWNER.value, MomentVisibility.PARTICIPANTS.value]
                            )
                        )
                        .order_by(schema.moments.c.created_at.desc())
                        .limit(min(max(limit, 1), 200))
                    )
                )
                .mappings()
                .all()
            )
            if not include_hidden:
                rows = [row for row in rows if row["status"] == MomentStatus.VISIBLE.value]
        # Core has already authorized the Owner for this Fluctlight. Visibility
        # governs what the feed projects, not whether its own Owner may read it.
        return [self._from_row(row) for row in rows]

    async def global_feed(
        self,
        *,
        owner_fluctlight_ids: tuple[str, ...],
        limit: int = 50,
        include_hidden: bool = False,
    ) -> list[Moment]:
        """Read a single feed across the caller's already-authorized instances."""

        if not owner_fluctlight_ids:
            return []
        async with self._unit_of_work.begin(command_id="moment-global-feed") as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.moments)
                        .where(schema.moments.c.owner_fluctlight_id.in_(owner_fluctlight_ids))
                        .where(
                            schema.moments.c.visibility.in_(
                                [MomentVisibility.OWNER.value, MomentVisibility.PARTICIPANTS.value]
                            )
                        )
                        .order_by(schema.moments.c.created_at.desc())
                        .limit(min(max(limit, 1), 200))
                    )
                )
                .mappings()
                .all()
            )
            if not include_hidden:
                rows = [row for row in rows if row["status"] == MomentStatus.VISIBLE.value]
        return [self._from_row(row) for row in rows]

    async def unread_counts(
        self, *, owner_fluctlight_ids: tuple[str, ...], actor_id: str
    ) -> dict[str, int]:
        """Count visible Moments after each persisted per-instance marker."""

        if not owner_fluctlight_ids:
            return {}
        async with self._unit_of_work.begin(command_id=f"moment-unread:{actor_id}") as tx:
            markers = {
                str(row["owner_fluctlight_id"]): row["last_seen_at"]
                for row in (
                    await tx.session.execute(
                        select(schema.unread_markers).where(
                            schema.unread_markers.c.actor_id == actor_id,
                            schema.unread_markers.c.owner_fluctlight_id.in_(owner_fluctlight_ids),
                        )
                    )
                ).mappings()
            }
            counts: dict[str, int] = {}
            for fluctlight_id in owner_fluctlight_ids:
                statement = select(schema.moments.c.id).where(
                    schema.moments.c.owner_fluctlight_id == fluctlight_id,
                    schema.moments.c.status == MomentStatus.VISIBLE.value,
                )
                if marker := markers.get(fluctlight_id):
                    statement = statement.where(schema.moments.c.created_at > marker)
                counts[fluctlight_id] = len((await tx.session.execute(statement)).all())
        return counts

    async def comments(self, moment_id: str) -> list[MomentComment]:
        async with self._unit_of_work.begin(command_id=f"moment-comments:{moment_id}") as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.comments)
                        .where(schema.comments.c.moment_id == moment_id)
                        .order_by(schema.comments.c.created_at)
                    )
                )
                .mappings()
                .all()
            )
        return [
            MomentComment(
                id=row["id"],
                moment_id=row["moment_id"],
                author_actor_id=row["author_actor_id"],
                text=row["text"],
                created_at=row["created_at"],
            )
            for row in rows
        ]

    async def reaction_summary(
        self, moment_id: str, actor_id: str
    ) -> tuple[int, ReactionKind | None]:
        async with self._unit_of_work.begin(command_id=f"moment-reactions:{moment_id}") as tx:
            rows = (
                (
                    await tx.session.execute(
                        select(schema.reactions).where(schema.reactions.c.moment_id == moment_id)
                    )
                )
                .mappings()
                .all()
            )
        own = next((row for row in rows if row["actor_id"] == actor_id), None)
        return len(rows), ReactionKind(own["kind"]) if own is not None else None

    async def set_status(self, moment_id: str, status: MomentStatus) -> None:
        async with self._unit_of_work.begin(command_id=f"moment-status:{moment_id}:{status}") as tx:
            result = await tx.session.execute(
                update(schema.moments)
                .where(schema.moments.c.id == moment_id)
                .values(status=status.value)
            )
            if result.rowcount != 1:
                raise KeyError(moment_id)
            await tx.commit()

    async def owner_fluctlight_id(self, moment_id: str) -> str:
        async with self._unit_of_work.begin(command_id=f"moment-owner:{moment_id}") as tx:
            owner = await tx.session.scalar(
                select(schema.moments.c.owner_fluctlight_id).where(schema.moments.c.id == moment_id)
            )
        if owner is None:
            raise KeyError(moment_id)
        return str(owner)

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
