import pytest
from fluctlight_core.moments.contracts import Moment, MomentStatus, MomentVisibility


def test_moment_contract_has_explicit_visibility_and_status() -> None:
    moment = Moment("moment-1", "fl-1", "fl-1", "A note", MomentVisibility.OWNER)
    assert moment.status is MomentStatus.VISIBLE
    with pytest.raises(ValueError):
        Moment("moment-1", "fl-1", "fl-1", "")
