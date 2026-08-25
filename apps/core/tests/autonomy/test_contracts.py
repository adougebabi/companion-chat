from datetime import UTC, datetime, time, timedelta

from fluctlight_core.life_world.contracts import AutonomyPolicy


def test_autonomy_policy_denies_paused_budget_and_quiet_actions() -> None:
    now = datetime.now(UTC)
    assert AutonomyPolicy(mode="paused").allows("moment", now, 0.1) == (False, "autonomy_paused")
    assert AutonomyPolicy(budget_remaining=0.0).allows("moment", now, 0.1) == (
        False,
        "budget_exhausted",
    )
    policy = AutonomyPolicy(quiet_hours=((time(0), time(23, 59)),))
    assert policy.allows("moment", now, 0.1)[0] is False
    assert AutonomyPolicy(cooldown_until=now + timedelta(hours=1)).allows("moment", now, 0.1) == (
        False,
        "cooldown_active",
    )
