from fluctlight_core.transport.api import EXPECTED_REVISION


def test_t09_advances_the_linear_readiness_revision() -> None:
    assert EXPECTED_REVISION == "0012_t12_consumer_effects"
