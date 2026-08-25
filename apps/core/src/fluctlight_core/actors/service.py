"""Owner authentication application service backed by the actors module."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from hashlib import sha256
from secrets import token_urlsafe
from uuid import uuid4

from sqlalchemy import insert, select, update

from fluctlight_core.platform.persistence import UnitOfWorkFactory

from . import schema
from .security import hash_password, issue_opaque_token, verify_password


class AuthError(RuntimeError):
    def __init__(self, code: str, status_code: int = 401) -> None:
        super().__init__(code)
        self.code = code
        self.status_code = status_code


@dataclass(frozen=True, slots=True)
class ResolvedHumanActor:
    actor_id: str
    session_id: str


@dataclass(frozen=True, slots=True)
class SessionResult:
    actor_id: str
    token: str
    expires_at: datetime


async def ensure_actor(tx, *, actor_id: str, actor_type: str) -> None:
    """Materialize a typed Actor inside the caller-owned transaction."""

    if actor_type not in {"human", "fluctlight"}:
        raise ValueError("actor_type must be human or fluctlight")
    existing = await tx.session.scalar(
        select(schema.actors.c.actor_type).where(schema.actors.c.id == actor_id)
    )
    if existing is None:
        await tx.session.execute(insert(schema.actors).values(id=actor_id, actor_type=actor_type))
    elif existing != actor_type:
        raise ValueError("actor identity already exists with a different type")


class AuthService:
    def __init__(
        self, unit_of_work: UnitOfWorkFactory, *, session_ttl: timedelta = timedelta(days=14)
    ) -> None:
        self._unit_of_work = unit_of_work
        self._session_ttl = session_ttl

    async def issue_setup_token(self, *, expires_in: timedelta = timedelta(hours=1)) -> str:
        token = token_urlsafe(32)
        now = datetime.now(UTC)
        async with self._unit_of_work.begin(command_id=f"setup-token:{uuid4()}") as tx:
            existing_owner = await tx.session.scalar(select(schema.owner_accounts.c.human_actor_id))
            if existing_owner is not None:
                raise AuthError("owner_already_exists", 409)
            await tx.session.execute(
                insert(schema.owner_setup_tokens).values(
                    id=str(uuid4()),
                    token_hash=sha256(token.encode()).hexdigest(),
                    expires_at=now + expires_in,
                )
            )
            await tx.commit()
        return token

    async def setup(self, *, setup_token: str, password: str) -> SessionResult:
        now = datetime.now(UTC)
        token_hash = sha256(setup_token.encode()).hexdigest()
        async with self._unit_of_work.begin(command_id=f"setup:{uuid4()}") as tx:
            owner = await tx.session.scalar(
                select(schema.owner_accounts.c.human_actor_id).with_for_update()
            )
            row = (
                (
                    await tx.session.execute(
                        select(schema.owner_setup_tokens)
                        .where(schema.owner_setup_tokens.c.token_hash == token_hash)
                        .with_for_update()
                    )
                )
                .mappings()
                .one_or_none()
            )
            if (
                owner is not None
                or row is None
                or row["consumed_at"] is not None
                or row["expires_at"] <= now
            ):
                raise AuthError("setup_unavailable", 403)
            actor_id = f"human_{uuid4().hex}"
            session = issue_opaque_token()
            session_id = f"session_{uuid4().hex}"
            expires_at = now + self._session_ttl
            await tx.session.execute(insert(schema.actors).values(id=actor_id, actor_type="human"))
            await tx.session.execute(
                insert(schema.owner_accounts).values(
                    human_actor_id=actor_id,
                    credential_hash=hash_password(password),
                    parameters="argon2id-default",
                    credential_revision=str(uuid4()),
                    owner_key="owner",
                )
            )
            await tx.session.execute(
                update(schema.owner_setup_tokens)
                .where(schema.owner_setup_tokens.c.id == row["id"])
                .values(consumed_at=now)
            )
            await self._insert_session(tx, session_id, session.digest, actor_id, expires_at, now)
            await self._audit(tx, "setup", actor_id, "ok")
            await tx.commit()
        return SessionResult(actor_id, session.value, expires_at)

    async def login(self, *, password: str) -> SessionResult:
        now = datetime.now(UTC)
        async with self._unit_of_work.begin(command_id=f"login:{uuid4()}") as tx:
            row = (await tx.session.execute(select(schema.owner_accounts))).mappings().one_or_none()
            if row is None or not verify_password(row["credential_hash"], password)[0]:
                raise AuthError("authentication_failed")
            token = issue_opaque_token()
            session_id = f"session_{uuid4().hex}"
            expires_at = now + self._session_ttl
            await self._insert_session(
                tx, session_id, token.digest, row["human_actor_id"], expires_at, now
            )
            await self._audit(tx, "login", row["human_actor_id"], "ok")
            await tx.commit()
        return SessionResult(row["human_actor_id"], token.value, expires_at)

    async def resolve(self, token: str | None) -> ResolvedHumanActor:
        if not token:
            raise AuthError("unauthenticated")
        now = datetime.now(UTC)
        digest = sha256(token.encode()).hexdigest()
        async with self._unit_of_work.begin(command_id=f"resolve:{uuid4()}") as tx:
            row = (
                (
                    await tx.session.execute(
                        select(schema.auth_sessions).where(
                            schema.auth_sessions.c.token_hash == digest
                        )
                    )
                )
                .mappings()
                .one_or_none()
            )
            if row is None or row["revoked_at"] is not None or row["expires_at"] <= now:
                raise AuthError("unauthenticated")
            await tx.session.execute(
                update(schema.auth_sessions)
                .where(schema.auth_sessions.c.id == row["id"])
                .values(last_seen_at=now)
            )
            await tx.commit()
        return ResolvedHumanActor(row["human_actor_id"], row["id"])

    async def revoke_all(self, actor: ResolvedHumanActor) -> None:
        now = datetime.now(UTC)
        async with self._unit_of_work.begin(command_id=f"revoke-all:{uuid4()}") as tx:
            owner = await tx.session.scalar(select(schema.owner_accounts.c.human_actor_id))
            if owner != actor.actor_id:
                raise AuthError("forbidden", 403)
            await tx.session.execute(
                update(schema.auth_sessions)
                .where(schema.auth_sessions.c.human_actor_id == actor.actor_id)
                .values(revoked_at=now)
            )
            await self._audit(tx, "revoke_all", actor.actor_id, "ok")
            await tx.commit()

    async def revoke_current(self, actor: ResolvedHumanActor) -> None:
        now = datetime.now(UTC)
        async with self._unit_of_work.begin(command_id=f"revoke-current:{actor.session_id}") as tx:
            await tx.session.execute(
                update(schema.auth_sessions)
                .where(
                    schema.auth_sessions.c.id == actor.session_id,
                    schema.auth_sessions.c.human_actor_id == actor.actor_id,
                )
                .values(revoked_at=now)
            )
            await self._audit(tx, "revoke_current", actor.actor_id, "ok")
            await tx.commit()

    async def reset_password(self, actor: ResolvedHumanActor, *, password: str) -> None:
        now = datetime.now(UTC)
        async with self._unit_of_work.begin(command_id=f"reset-password:{uuid4()}") as tx:
            owner = await tx.session.scalar(select(schema.owner_accounts.c.human_actor_id))
            if owner != actor.actor_id:
                raise AuthError("forbidden", 403)
            await tx.session.execute(
                update(schema.owner_accounts)
                .where(schema.owner_accounts.c.human_actor_id == actor.actor_id)
                .values(
                    credential_hash=hash_password(password),
                    credential_revision=str(uuid4()),
                    updated_at=now,
                )
            )
            await tx.session.execute(
                update(schema.auth_sessions)
                .where(schema.auth_sessions.c.human_actor_id == actor.actor_id)
                .values(revoked_at=now)
            )
            await self._audit(tx, "reset_password", actor.actor_id, "ok")
            await tx.commit()

    async def is_owner(self, actor: ResolvedHumanActor) -> bool:
        async with self._unit_of_work.begin(command_id=f"owner-check:{uuid4()}") as tx:
            owner = await tx.session.scalar(select(schema.owner_accounts.c.human_actor_id))
        return owner == actor.actor_id

    async def _insert_session(
        self,
        tx,
        session_id: str,
        token_hash: str,
        actor_id: str,
        expires_at: datetime,
        now: datetime,
    ) -> None:
        await tx.session.execute(
            insert(schema.auth_sessions).values(
                id=session_id,
                token_hash=token_hash,
                human_actor_id=actor_id,
                expires_at=expires_at,
                last_seen_at=now,
            )
        )

    async def _audit(self, tx, action: str, actor_id: str | None, result: str) -> None:
        await tx.session.execute(
            insert(schema.auth_audit).values(
                id=f"auth_audit_{uuid4().hex}",
                action=action,
                actor_id=actor_id,
                result=result,
                details="{}",
            )
        )
