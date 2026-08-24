"""Stable identities for committed intents and external effects."""

from __future__ import annotations

import hashlib
import re


def stable_id(prefix: str, value: str) -> str:
    digest = hashlib.sha256(value.encode("utf-8")).hexdigest()[:24]
    return f"{prefix}_{digest}"


def intent_id(intent_key: str) -> str:
    return stable_id("intent", intent_key)


def workflow_id(committed_intent_id: str) -> str:
    return stable_id("wf", committed_intent_id)


def provider_request_id(committed_intent_id: str) -> str:
    return stable_id("provider", committed_intent_id)


def correlation_id(committed_intent_id: str) -> str:
    return stable_id("corr", committed_intent_id)


def result_id(committed_workflow_id: str) -> str:
    return stable_id("result", committed_workflow_id)


def audit_id(actor: str, action: str, execution_id: str) -> str:
    return stable_id("audit", f"{actor}:{action}:{execution_id}")


def slug(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-") or "unknown"
