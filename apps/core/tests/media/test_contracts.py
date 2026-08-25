import pytest
from fluctlight_core.media.contracts import MediaAsset, MediaIntent, MediaKind, MediaStatus


def test_media_identity_has_stable_provider_and_object_fields() -> None:
    intent = MediaIntent(
        "intent-1", "fl-1", MediaKind.IMAGE, "image/png", "a prompt", "provider-1", "workflow-1"
    )
    asset = MediaAsset(
        "asset-1",
        "fl-1",
        "v1",
        MediaKind.IMAGE,
        "image/png",
        3,
        "a" * 64,
        "private",
        "media/asset-1/v1",
        "v1",
        "etag",
        "provider-1",
        "workflow-1",
        MediaStatus.READY,
    )
    assert intent.provider_request_id == asset.provider_request_id
    assert asset.object_key.startswith("media/")
    with pytest.raises(ValueError):
        MediaAsset(
            "asset-1",
            "fl-1",
            "v1",
            MediaKind.IMAGE,
            "image/png",
            -1,
            "sha",
            "private",
            "key",
            None,
            None,
            "provider",
            "workflow",
        )
