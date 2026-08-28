"""Start the Core API process. It never polls Temporal task queues."""

from __future__ import annotations

import os

import uvicorn

from fluctlight_core.platform.configuration import PlatformSettings
from fluctlight_core.transport.api import create_app


def main() -> None:
    settings = PlatformSettings.from_environ()
    log_level = os.environ.get("FLUCTLIGHT_LOG_LEVEL", "info").lower()
    uvicorn.run(
        create_app(),
        host=settings.api_host,
        port=settings.api_port,
        log_level=log_level,
    )
