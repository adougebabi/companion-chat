"""Autonomy policy and frozen-action service.

Keep execution dependencies out of the package initializer. Temporal imports a
workflow module through this package inside its deterministic sandbox.
"""

from .service import AutonomyService

__all__ = ["AutonomyService"]
