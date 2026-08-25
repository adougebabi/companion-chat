from pathlib import Path


def test_conversation_contracts_are_framework_free() -> None:
    source = (
        Path(__file__).parents[2] / "src" / "fluctlight_core" / "conversations" / "contracts.py"
    ).read_text()
    lowered = source.lower()
    assert all(
        token not in lowered for token in ("fastapi", "temporalio", "redis", "boto3", "sqlalchemy")
    )


def test_t06_transport_only_owns_ndjson_mapping() -> None:
    source = (
        Path(__file__).parents[2] / "src" / "fluctlight_core" / "transport" / "conversations.py"
    ).read_text()
    assert "NdjsonProducer" in source
    assert "ConversationService" in source
