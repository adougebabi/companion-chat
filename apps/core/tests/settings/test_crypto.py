import base64

import pytest
from fluctlight_core.settings.crypto import SecretCodec, SecretConfigurationError


def test_secret_round_trip_uses_distinct_nonce_and_redacted_value() -> None:
    codec = SecretCodec(base64.b64encode(b"a" * 32).decode("ascii"))
    first = codec.encrypt(purpose="provider:alpha", plaintext="test-secret")
    second = codec.encrypt(purpose="provider:alpha", plaintext="test-secret")
    assert first.nonce != second.nonce
    assert b"test-secret" not in first.ciphertext
    value = codec.decrypt(first)
    assert value.reveal_for_provider() == "test-secret"
    assert "test-secret" not in repr(value)


def test_secret_codec_rejects_invalid_key_or_authenticated_data() -> None:
    with pytest.raises(SecretConfigurationError):
        SecretCodec("not-base64")
    codec = SecretCodec(base64.b64encode(b"a" * 32).decode("ascii"))
    encrypted = codec.encrypt(purpose="provider:alpha", plaintext="test-secret")
    with pytest.raises(SecretConfigurationError):
        codec.decrypt(type(encrypted)(encrypted.ciphertext, encrypted.nonce, "provider:other"))
