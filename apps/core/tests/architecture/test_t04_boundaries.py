from pathlib import Path


def _source() -> str:
    root = Path(__file__).parents[2] / "src" / "fluctlight_core"
    paths = [
        root / "fluctlights" / "contracts.py",
        root / "fluctlights" / "policy.py",
        root / "inner_state" / "contracts.py",
        root / "inner_state" / "policy.py",
    ]
    return "\n".join(path.read_text() for path in paths)


def test_t04_domain_modules_do_not_import_transport_or_external_runtimes() -> None:
    source = _source().lower()
    forbidden = ("fastapi", "temporalio", "redis", "boto3", "sqlalchemy")
    assert all(token not in source for token in forbidden)


def test_t04_application_services_do_not_import_external_runtime_adapters() -> None:
    root = Path(__file__).parents[2] / "src" / "fluctlight_core"
    source = "\n".join(
        path.read_text()
        for path in (
            root / "fluctlights" / "service.py",
            root / "fluctlights" / "schema.py",
            root / "inner_state" / "service.py",
            root / "inner_state" / "schema.py",
        )
    ).lower()
    assert all(token not in source for token in ("fastapi", "temporalio", "redis", "boto3"))


def test_t04_domain_modules_do_not_add_natural_language_heuristic_paths() -> None:
    source = _source().lower()
    forbidden = ("regex", "keyword", "substring", "sentiment word", "fallback_appraisal")
    assert all(token not in source for token in forbidden)
