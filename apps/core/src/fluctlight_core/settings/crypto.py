"""Single-key AEAD codec; plaintext never serializes outside this module."""

from __future__ import annotations

import base64
import binascii
from dataclasses import dataclass
from secrets import token_bytes

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives.ciphers.aead import AESGCM


class SecretConfigurationError(ValueError):
    """The configured settings key cannot securely operate on a secret."""


@dataclass(frozen=True, slots=True)
class EncryptedSecret:
    ciphertext: bytes
    nonce: bytes
    purpose: str


@dataclass(frozen=True, slots=True)
class SecretValue:
    _value: str

    def reveal_for_provider(self) -> str:
        return self._value

    def __repr__(self) -> str:
        return "SecretValue([redacted])"


class SecretCodec:
    def __init__(self, encoded_key: str) -> None:
        try:
            key = base64.b64decode(encoded_key, validate=True)
        except (ValueError, binascii.Error) as exc:
            raise SecretConfigurationError("FLUCTLIGHT_SETTINGS_KEY must be base64") from exc
        if len(key) != 32:
            raise SecretConfigurationError("FLUCTLIGHT_SETTINGS_KEY must decode to 32 bytes")
        self._cipher = AESGCM(key)

    def encrypt(self, *, purpose: str, plaintext: str) -> EncryptedSecret:
        if not purpose or not plaintext:
            raise ValueError("secret purpose and plaintext are required")
        nonce = token_bytes(12)
        packed = self._cipher.encrypt(nonce, plaintext.encode("utf-8"), purpose.encode("utf-8"))
        return EncryptedSecret(ciphertext=packed, nonce=nonce, purpose=purpose)

    def decrypt(self, encrypted: EncryptedSecret) -> SecretValue:
        try:
            plaintext = self._cipher.decrypt(
                encrypted.nonce, encrypted.ciphertext, encrypted.purpose.encode("utf-8")
            )
        except InvalidTag as exc:
            raise SecretConfigurationError("settings secret cannot be authenticated") from exc
        return SecretValue(plaintext.decode("utf-8"))
