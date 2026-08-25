"""Create T03 actor, auth, settings, and provider tables.

Revision ID: 0002_t03_auth
Revises: 0001_platform
"""

from alembic import op
from fluctlight_core.actors import schema as actors
from fluctlight_core.providers import schema as providers
from fluctlight_core.settings import schema as settings

revision = "0002_t03_auth"
down_revision = "0001_platform"
branch_labels = None
depends_on = None


_TABLES = (
    actors.actors,
    actors.owner_accounts,
    actors.auth_sessions,
    actors.owner_setup_tokens,
    actors.auth_audit,
    settings.runtime_settings,
    settings.setting_secrets,
    settings.settings_audit,
    providers.provider_endpoints,
    providers.model_roles,
    providers.provider_preflights,
    providers.provider_provenance,
)


def upgrade() -> None:
    bind = op.get_bind()
    for table in _TABLES:
        table.create(bind=bind)


def downgrade() -> None:
    bind = op.get_bind()
    for table in reversed(_TABLES):
        table.drop(bind=bind)
