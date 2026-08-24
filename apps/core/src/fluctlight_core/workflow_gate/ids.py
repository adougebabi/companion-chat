"""Stable identifiers used by every gate record and recovery attempt."""

from __future__ import annotations

import hashlib
import re


def stable_id(prefix: str, value: str) -> str:
    """Return an opaque, deterministic identifier for a committed value."""

    digest = hashlib.sha256(value.encode("utf-8")).hexdigest()[:24]
    return f"{prefix}_{digest}"


def slug(value: str) -> str:
    """Keep log labels readable without allowing caller input into identifiers."""

    return re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-") or "unknown"


def workflow_id(intent_id: str) -> str:
    return stable_id("wf", intent_id)


def provider_request_id(intent_id: str) -> str:
    return stable_id("provider", intent_id)


def correlation_id(intent_id: str) -> str:
    return stable_id("corr", intent_id)
