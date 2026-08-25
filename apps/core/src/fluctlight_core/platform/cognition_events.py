"""Stable event names emitted by the cognition module."""

from __future__ import annotations

COGNITION_INBOX_ACCEPTED = "cognition.inbox.accepted"
COGNITION_ACTION_FROZEN = "cognition.action.frozen"
COGNITION_ACTION_COMPLETED = "cognition.action.completed"
COGNITION_ACTION_FAILED = "cognition.action.failed"
COGNITION_REFLECTION_PROPOSED = "cognition.reflection.proposed"

__all__ = [
    "COGNITION_ACTION_COMPLETED",
    "COGNITION_ACTION_FAILED",
    "COGNITION_ACTION_FROZEN",
    "COGNITION_INBOX_ACCEPTED",
    "COGNITION_REFLECTION_PROPOSED",
]
