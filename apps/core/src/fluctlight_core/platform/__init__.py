"""Framework-neutral infrastructure contracts shared by Core entrypoints."""

from .configuration import PlatformSettings
from .persistence import UnitOfWork, UnitOfWorkFactory

__all__ = ["PlatformSettings", "UnitOfWork", "UnitOfWorkFactory"]
