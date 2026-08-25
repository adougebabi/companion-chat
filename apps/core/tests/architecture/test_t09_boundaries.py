from pathlib import Path


def test_t09_domain_contracts_do_not_import_storage_or_transport_runtimes() -> None:
    root = Path(__file__).parents[2] / "src" / "fluctlight_core"
    source = "\n".join(
        (root / module / "contracts.py").read_text() for module in ("moments", "media")
    ).lower()
    assert all(
        token not in source for token in ("fastapi", "temporalio", "redis", "boto3", "sqlalchemy")
    )
    assert "absolute path" not in source
