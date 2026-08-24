from __future__ import annotations

from pathlib import Path


def test_platform_does_not_depend_on_a_second_workflow_runtime() -> None:
    source_root = Path(__file__).parents[2] / "src" / "fluctlight_core" / "platform"
    source = "\n".join(path.read_text() for path in source_root.glob("*.py"))
    assert "celery" not in source.lower()
    assert "dbos" not in source.lower()
