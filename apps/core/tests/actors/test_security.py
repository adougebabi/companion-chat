from fluctlight_core.actors.security import hash_password, issue_opaque_token, verify_password


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
