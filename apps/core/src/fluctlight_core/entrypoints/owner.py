"""Explicit local-only Owner setup and recovery commands."""

from __future__ import annotations

import argparse
import asyncio
import getpass
from datetime import timedelta

from fluctlight_core.actors.service import AuthService
from fluctlight_core.platform.configuration import PlatformSettings
from fluctlight_core.platform.persistence import UnitOfWorkFactory, create_engine


async def run(args: argparse.Namespace) -> None:
    settings = PlatformSettings.from_environ()
    engine = create_engine(settings.database_url)
    try:
        service = AuthService(UnitOfWorkFactory(engine))
        if args.command == "issue-setup-token":
            print(
                await service.issue_setup_token(expires_in=timedelta(minutes=args.expires_minutes))
            )
        elif args.command == "revoke-all-sessions":
            token = getpass.getpass("Active session token: ")
            await service.revoke_all(await service.resolve(token))
        elif args.command == "reset-password":
            token = getpass.getpass("Active session token: ")
            password = getpass.getpass("New password: ")
            await service.reset_password(await service.resolve(token), password=password)
        else:
            raise RuntimeError("unsupported owner command")
    finally:
        await engine.dispose()


def main() -> None:
    parser = argparse.ArgumentParser(prog="fluctlight owner")
    subcommands = parser.add_subparsers(dest="command", required=True)
    issue = subcommands.add_parser("issue-setup-token")
    issue.add_argument("--expires-minutes", type=int, default=60)
    subcommands.add_parser("revoke-all-sessions")
    subcommands.add_parser("reset-password")
    asyncio.run(run(parser.parse_args()))
