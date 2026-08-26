import pytest
from fluctlight_core.actors.security import (
    MIN_OWNER_PASSWORD_LENGTH,
    hash_password,
    issue_opaque_token,
    verify_password,
)


def test_passwords_are_argon2id_and_rehashable() -> None:
    password_hash = hash_password("correct horse battery staple")
    valid, needs_rehash = verify_password(password_hash, "correct horse battery staple")
    assert password_hash.startswith("$argon2id$")
    assert valid is True
    assert needs_rehash is False
    assert verify_password(password_hash, "wrong")[0] is False


def test_opaque_tokens_do_not_expose_their_value_in_repr() -> None:
    token = issue_opaque_token()
    assert len(token.digest) == 64
    assert token.value not in repr(token)


def test_owner_passwords_require_six_characters_without_a_strength_policy() -> None:
    password_hash = hash_password("123456")
    assert verify_password(password_hash, "123456")[0] is True
    with pytest.raises(ValueError, match="at least 6 characters"):
        hash_password("12345")
    assert MIN_OWNER_PASSWORD_LENGTH == 6
