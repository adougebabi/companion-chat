"""PostgreSQL metadata, explicit migration checks, and application Unit of Work."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Final

from sqlalchemy import MetaData, text
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

NAMING_CONVENTION: Final = {
    "ix": "ix_%(column_0_label)s",
    "uq": "uq_%(table_name)s_%(column_0_name)s",
    "ck": "ck_%(table_name)s_%(constraint_name)s",
    "fk": "fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s",
    "pk": "pk_%(table_name)s",
}
metadata = MetaData(schema="public", naming_convention=NAMING_CONVENTION)


class MigrationRevisionError(RuntimeError):
    """The database is not at the revision deployed with this process."""


class UnitOfWork:
    """One short application transaction with one explicit commit owner."""

    def __init__(self, session: AsyncSession, command_id: str) -> None:
        self.session = session
        self.command_id = command_id
        self._committed = False

    async def commit(self) -> None:
        if self._committed:
            raise RuntimeError("UnitOfWork.commit may be called only once")
        await self.session.commit()
        self._committed = True

    async def rollback(self) -> None:
        if not self._committed:
            await self.session.rollback()


class UnitOfWorkFactory:
    """Creates one AsyncSession per command; sessions never cross task boundaries."""

    def __init__(self, engine: AsyncEngine) -> None:
        self.engine = engine
        self._sessions = async_sessionmaker(engine, expire_on_commit=False)

    @asynccontextmanager
    async def begin(self, *, command_id: str) -> AsyncIterator[UnitOfWork]:
        async with self._sessions() as session:
            transaction = UnitOfWork(session, command_id)
            try:
                yield transaction
                if not transaction._committed:
                    await transaction.rollback()
            except BaseException:
                await transaction.rollback()
                raise


def create_engine(database_url: str) -> AsyncEngine:
    normalized = database_url.replace("postgresql://", "postgresql+psycopg://", 1)
    return create_async_engine(normalized, pool_pre_ping=True)


async def verify_revision(engine: AsyncEngine, expected_revision: str) -> None:
    async with engine.connect() as connection:
        try:
            revision = await connection.scalar(text("SELECT version_num FROM alembic_version"))
        except Exception as exc:
            raise MigrationRevisionError("database migration revision is unavailable") from exc
    if revision != expected_revision:
        raise MigrationRevisionError(
            f"database revision is {revision or 'none'}; expected {expected_revision}"
        )
