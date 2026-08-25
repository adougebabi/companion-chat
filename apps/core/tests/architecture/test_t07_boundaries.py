from pathlib import Path


def test_t07_domain_contracts_do_not_import_transport_or_external_runtimes() -> None:
    root = Path(__file__).parents[2] / "src" / "fluctlight_core"
    source = "\n".join(
        (root / module / "contracts.py").read_text() for module in ("memory", "relationships")
    ).lower()
    assert all(
        token not in source for token in ("fastapi", "temporalio", "redis", "boto3", "sqlalchemy")
    )
    assert all(token not in source for token in ("keyword", "regex", "sentiment word"))
