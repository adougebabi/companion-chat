"""DBOS Worker process entrypoint."""

from __future__ import annotations

import signal
import threading

from .dbos_runtime import launch_worker


def main() -> None:
    dbos = launch_worker()
    stopped = threading.Event()
    signal.signal(signal.SIGTERM, lambda *_: stopped.set())
    signal.signal(signal.SIGINT, lambda *_: stopped.set())
    stopped.wait()
    destroy = getattr(dbos, "destroy", None)
    if callable(destroy):
        destroy()


if __name__ == "__main__":
    main()
