import pytest
from fluctlight_core.platform.timezones import canonical_timezone


@pytest.mark.parametrize("value", ["UTC+8", "UTC+08:00", "GMT+8", "China Standard Time"])
def test_canonical_timezone_normalizes_china_utc_plus_eight_aliases(value: str) -> None:
    assert canonical_timezone(value) == "Asia/Shanghai"


def test_canonical_timezone_rejects_unknown_offset_labels() -> None:
    with pytest.raises(ValueError, match="IANA"):
        canonical_timezone("UTC+9")
