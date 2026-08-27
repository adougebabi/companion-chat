from __future__ import annotations

import importlib.util
from pathlib import Path


def test_0018_declares_versioned_life_profile_and_provenance_columns() -> None:
    path = (
        Path(__file__).parents[2] / "migrations" / "versions" / "0018_foundation_v2_life_profile.py"
    )
    specification = importlib.util.spec_from_file_location("migration_0018", path)
    assert specification and specification.loader
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)

    assert module.revision == "0018_foundation_v2_life_profile"
    assert module.down_revision == "0017_media_intent_moment"
