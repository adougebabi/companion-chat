import pytest
from fluctlight_core.media.contracts import MediaAsset, MediaIntent, MediaKind, MediaStatus
from fluctlight_core.platform.object_storage import ObjectDescriptor, S3ObjectStorage


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
    long_prompt = MediaIntent(
        "intent-long", "fl-1", MediaKind.IMAGE, "image/png", "p" * 2_000, "provider-2", "workflow-2"
    )
    assert len(long_prompt.prompt) == 2_000
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


class _ObjectClient:
    def __init__(self) -> None:
        self.request: dict[str, object] | None = None

    def get_object(self, **request: object) -> dict[str, object]:
        self.request = request
        return {"Body": _Body(b"media"), "ETag": "etag"}


class _Body:
    def __init__(self, value: bytes) -> None:
        self._value = value

    def read(self) -> bytes:
        return self._value


def test_object_storage_normalizes_browser_open_ended_and_suffix_ranges() -> None:
    client = _ObjectClient()
    storage = S3ObjectStorage(client, "private")
    descriptor = ObjectDescriptor("private", "media/a/v1", None, None, "video/mp4", 10, "a" * 64)

    open_ended = storage.grant_read(descriptor, allowed_range="bytes=0-")
    suffix = storage.grant_read(descriptor, allowed_range="bytes=-4")

    assert open_ended.allowed_range == "bytes=0-9"
    assert suffix.allowed_range == "bytes=6-9"
    storage.read(open_ended)
    assert client.request and client.request["Range"] == "bytes=0-9"


@pytest.mark.parametrize("range_header", ["bytes=10-", "bytes=5-3", "bytes=-0", "items=0-1"])
def test_object_storage_rejects_unsatisfiable_ranges(range_header: str) -> None:
    storage = S3ObjectStorage(_ObjectClient(), "private")
    descriptor = ObjectDescriptor("private", "media/a/v1", None, None, "video/mp4", 10, "a" * 64)

    with pytest.raises(ValueError):
        storage.grant_read(descriptor, allowed_range=range_header)
