from pathlib import Path


def test_t05_domain_contracts_do_not_import_external_runtimes_or_heuristics() -> None:
    root = Path(__file__).parents[2] / "src" / "fluctlight_core" / "cognition"
    source = "\n".join((root / "contracts.py", root / "__init__.py")[0].read_text() for _ in [0])
    lowered = source.lower()
    assert all(
        token not in lowered for token in ("fastapi", "temporalio", "redis", "boto3", "sqlalchemy")
    )
    assert all(
        token not in lowered
        for token in ("keyword", "regex", "sentiment word", "fallback_appraisal")
    )


def test_t05_diagnostics_contract_does_not_persist_hidden_reasoning() -> None:
    source = (
        Path(__file__).parents[2] / "src" / "fluctlight_core" / "diagnostics" / "contracts.py"
    ).read_text()
    assert "hidden_reasoning" in source
    assert "redact" in source
