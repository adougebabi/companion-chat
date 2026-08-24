"""Small, reproducible Temporal suitability gate.

The package deliberately contains only runtime fixtures. Domain facts remain
owned by the application schema introduced by later child tasks.
"""

from .ids import correlation_id, intent_id, provider_request_id, workflow_id
from .models import GateInput, GateResult, WorkflowRecord, WorkflowStatus

__all__ = [
    "GateInput",
    "GateResult",
    "WorkflowRecord",
    "WorkflowStatus",
    "correlation_id",
    "intent_id",
    "provider_request_id",
    "workflow_id",
]
