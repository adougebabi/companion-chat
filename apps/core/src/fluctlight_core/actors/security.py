"""Transport-neutral Owner credential and opaque-session primitives."""

from __future__ import annotations

from dataclasses import dataclass
from hashlib import sha256
from secrets import token_urlsafe

from argon2 import PasswordHasher
from argon2.exceptions import InvalidHashError, VerifyMismatchError

_PASSWORD_HASHER = PasswordHasher()


@dataclass(frozen=True, slots=True)
class OpaqueToken:
    value: str

    @property
    def digest(self) -> str:
        return sha256(self.value.encode("utf-8")).hexdigest()

    def __repr__(self) -> str:
        return "OpaqueToken([redacted])"


def issue_opaque_token() -> OpaqueToken:
    return OpaqueToken(token_urlsafe(32))


def hash_password(password: str) -> str:
    if not password:
        raise ValueError("password must not be empty")
    return _PASSWORD_HASHER.hash(password)


def verify_password(password_hash: str, password: str) -> tuple[bool, bool]:
    try:
        valid = _PASSWORD_HASHER.verify(password_hash, password)
    except (InvalidHashError, VerifyMismatchError):
        return False, False
    return valid, _PASSWORD_HASHER.check_needs_rehash(password_hash)
