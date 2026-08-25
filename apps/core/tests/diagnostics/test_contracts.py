from fluctlight_core.diagnostics.contracts import (
    DiagnosticEvent,
    DiagnosticModelRun,
    DiagnosticSeverity,
    redact,
)


def test_redaction_removes_secrets_and_hidden_reasoning_recursively() -> None:
    value = redact(
        {
            "prompt": "visible",
            "api_key": "secret",
            "nested": {"authorization": "Bearer x", "hidden_reasoning": "private"},
            "items": [{"password": "p", "text": "ok"}],
        }
    )
    assert value == {
        "prompt": "visible",
        "api_key": "[REDACTED]",
        "nested": {"authorization": "[REDACTED]"},
        "items": [{"password": "[REDACTED]", "text": "ok"}],
    }


def test_diagnostic_models_redact_at_construction() -> None:
    event = DiagnosticEvent(
        event_type="cognition.assessment",
        payload={"token": "secret", "message": "visible"},
        correlation_id="corr-1",
        severity=DiagnosticSeverity.INFO,
    )
    run = DiagnosticModelRun(
        role="cognitive_assessment",
        model_id="model-1",
        prompt={"text": "visible", "chain_of_thought": "hidden"},
        response={"output": "ok"},
        correlation_id="corr-1",
        status="completed",
    )
    assert event.payload["token"] == "[REDACTED]"
    assert "chain_of_thought" not in run.prompt
