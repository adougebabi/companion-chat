"""Explicit Alembic migration command; API and Worker never auto-upgrade."""

from __future__ import annotations

import os
from pathlib import Path

from alembic import command
from alembic.config import Config


def main() -> None:
    root = Path(__file__).resolve().parents[3]
    config = Config(str(root / "alembic.ini"))
    database_url = os.environ["DATABASE_URL"].replace("postgresql://", "postgresql+psycopg://", 1)
    config.set_main_option("sqlalchemy.url", database_url)
    config.set_main_option("script_location", str(root / "migrations"))
    command.upgrade(config, "head")
