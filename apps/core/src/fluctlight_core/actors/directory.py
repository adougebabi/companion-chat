"""Owner-organized Actor directory groups without conversation behavior."""

from __future__ import annotations

from dataclasses import dataclass
from uuid import uuid4

from sqlalchemy import delete, insert, select

from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema
from .service import AuthError, AuthService, ResolvedHumanActor


@dataclass(frozen=True, slots=True)
class ActorGroup:
    id: str
    name: str
    actor_ids: tuple[str, ...]


class ActorDirectoryService:
    def __init__(self, unit_of_work: UnitOfWorkFactory, auth: AuthService) -> None:
        self._unit_of_work = unit_of_work
        self._auth = auth

    async def list_groups(self, actor: ResolvedHumanActor) -> list[ActorGroup]:
        await self._require_owner(actor)
        async with self._unit_of_work.begin(command_id=f"actor-groups:{actor.actor_id}") as tx:
            groups = (
                await tx.session.execute(
                    select(schema.actor_groups)
                    .where(schema.actor_groups.c.owner_actor_id == actor.actor_id)
                    .order_by(schema.actor_groups.c.name)
                )
            ).mappings().all()
            members = (
                await tx.session.execute(select(schema.actor_group_members))
            ).mappings().all()
        by_group: dict[str, list[str]] = {}
        for member in members:
            by_group.setdefault(str(member["group_id"]), []).append(str(member["actor_id"]))
        return [
            ActorGroup(
                str(group["id"]),
                str(group["name"]),
                tuple(by_group.get(str(group["id"]), ())),
            )
            for group in groups
        ]

    async def create_group(self, actor: ResolvedHumanActor, *, name: str) -> ActorGroup:
        await self._require_owner(actor)
        normalized = name.strip()
        if not normalized or len(normalized) > 128:
            raise ValueError("group name is required and bounded")
        group = ActorGroup(f"actor_group_{uuid4().hex}", normalized, ())
        async with self._unit_of_work.begin(command_id=f"actor-group-create:{group.id}") as tx:
            await tx.session.execute(
                insert(schema.actor_groups).values(
                    id=group.id, owner_actor_id=actor.actor_id, name=group.name
                )
            )
            await tx.commit()
        return group

    async def assign_member(
        self, actor: ResolvedHumanActor, *, group_id: str, member_actor_id: str
    ) -> None:
        await self._require_owner(actor)
        async with self._unit_of_work.begin(command_id=f"actor-group-assign:{group_id}") as tx:
            group = await tx.session.scalar(
                select(schema.actor_groups.c.id).where(
                    schema.actor_groups.c.id == group_id,
                    schema.actor_groups.c.owner_actor_id == actor.actor_id,
                )
            )
            member_type = await tx.session.scalar(
                select(schema.actors.c.actor_type).where(schema.actors.c.id == member_actor_id)
            )
            if group is None or member_type not in {"human", "fluctlight"}:
                raise KeyError("group or Actor is unavailable")
            existing = await tx.session.scalar(
                select(schema.actor_group_members.c.actor_id).where(
                    schema.actor_group_members.c.group_id == group_id,
                    schema.actor_group_members.c.actor_id == member_actor_id,
                )
            )
            if existing is None:
                await tx.session.execute(
                    insert(schema.actor_group_members).values(
                        group_id=group_id, actor_id=member_actor_id
                    )
                )
                await tx.commit()

    async def remove_member(
        self, actor: ResolvedHumanActor, *, group_id: str, member_actor_id: str
    ) -> None:
        await self._require_owner(actor)
        async with self._unit_of_work.begin(command_id=f"actor-group-remove:{group_id}") as tx:
            group = await tx.session.scalar(
                select(schema.actor_groups.c.id).where(
                    schema.actor_groups.c.id == group_id,
                    schema.actor_groups.c.owner_actor_id == actor.actor_id,
                )
            )
            if group is None:
                raise KeyError(group_id)
            await tx.session.execute(
                delete(schema.actor_group_members).where(
                    schema.actor_group_members.c.group_id == group_id,
                    schema.actor_group_members.c.actor_id == member_actor_id,
                )
            )
            await tx.commit()

    async def _require_owner(self, actor: ResolvedHumanActor) -> None:
        if not await self._auth.is_owner(actor):
            raise AuthError("forbidden", 403)
