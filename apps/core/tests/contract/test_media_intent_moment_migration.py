from __future__ import annotations

import importlib.util
from pathlib import Path
from types import ModuleType


def _migration_module() -> ModuleType:
    path = Path(__file__).parents[2] / "migrations" / "versions" / "0017_media_intent_moment.py"
    specification = importlib.util.spec_from_file_location("migration_0017", path)
    assert specification and specification.loader
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    return module


class _Inspector:
    def __init__(self, *, columns: list[str], foreign_keys: list[dict[str, object]]) -> None:
        self._columns = columns
        self._foreign_keys = foreign_keys

    def get_columns(self, _table_name: str, *, schema: str) -> list[dict[str, str]]:
        assert schema == "public"
        return [{"name": column} for column in self._columns]

    def get_foreign_keys(self, _table_name: str, *, schema: str) -> list[dict[str, object]]:
        assert schema == "public"
        return self._foreign_keys


class _Operations:
    def __init__(self) -> None:
        self.calls: list[str] = []

    def get_bind(self) -> object:
        return object()

    def add_column(self, *_args: object, **_kwargs: object) -> None:
        self.calls.append("add_column")

    def create_foreign_key(self, *_args: object, **_kwargs: object) -> None:
        self.calls.append("create_foreign_key")

    def execute(self, *_args: object, **_kwargs: object) -> None:
        self.calls.append("upgrade_existing_visibility")

    def alter_column(self, *_args: object, **_kwargs: object) -> None:
        self.calls.append("alter_visibility_default")


def test_0017_creates_moment_target_column_and_foreign_key(monkeypatch) -> None:
    module = _migration_module()
    operations = _Operations()
    monkeypatch.setattr(module.op, "get_bind", operations.get_bind)
    monkeypatch.setattr(module.op, "add_column", operations.add_column)
    monkeypatch.setattr(module.op, "create_foreign_key", operations.create_foreign_key)
    monkeypatch.setattr(module.op, "execute", operations.execute)
    monkeypatch.setattr(module.op, "alter_column", operations.alter_column)
    monkeypatch.setattr(
        module.sa,
        "inspect",
        lambda _bind: _Inspector(columns=[], foreign_keys=[]),
    )

    module.upgrade()

    assert operations.calls == [
        "upgrade_existing_visibility",
        "alter_visibility_default",
        "add_column",
        "create_foreign_key",
    ]
