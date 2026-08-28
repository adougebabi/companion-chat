from fluctlight_core.media import schema as media_schema
from fluctlight_core.moments import schema as moments_schema
from fluctlight_core.platform.persistence import metadata


def test_t09_tables_share_public_metadata_and_private_object_identity() -> None:
    expected = {
        "moments",
        "moment_comments",
        "moment_reactions",
        "moment_unread_markers",
        "media_intents",
        "media_assets",
        "media_references",
        "media_tombstones",
    }
    assert {f"public.{name}" for name in expected} <= set(metadata.tables)
    assert media_schema.assets.c.object_key is not None
    assert media_schema.intents.c.provider_job_id is not None
    assert moments_schema.reactions.c.actor_id is not None
