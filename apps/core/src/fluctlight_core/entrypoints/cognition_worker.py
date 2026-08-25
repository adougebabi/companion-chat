"""Small adapter used by the Worker process to drain one cognition writer."""

from __future__ import annotations

from fluctlight_core.cognition.service import CognitionService


async def process_fluctlight_once(service: CognitionService, fluctlight_id: str, *, worker_id: str):
    """Process at most one ordered inbox item; callers own the polling policy."""

    return await service.process_next(fluctlight_id, worker_id=worker_id)
