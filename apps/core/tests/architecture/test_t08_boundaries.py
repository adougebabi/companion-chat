from pathlib import Path


def test_t08_contracts_do_not_import_external_runtimes_or_heuristics() -> None:
    root = Path(__file__).parents[2] / "src" / "fluctlight_core"
    source = "\n".join(
        (root / module / "contracts.py").read_text() for module in ("life_world",)
    ).lower()
    assert all(
        token not in source for token in ("fastapi", "temporalio", "redis", "boto3", "sqlalchemy")
    )
    assert all(token not in source for token in ("occupation ==", "weekday", "keyword", "fallback"))
