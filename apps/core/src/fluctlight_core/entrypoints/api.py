"""Start the Core API process. It never polls Temporal task queues."""

from __future__ import annotations

import uvicorn

from fluctlight_core.platform.configuration import PlatformSettings
from fluctlight_core.transport.api import create_app


def main() -> None:
    settings = PlatformSettings.from_environ()
    uvicorn.run(create_app(), host=settings.api_host, port=settings.api_port, log_level="info")
