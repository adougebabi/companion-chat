# Fluctlight Auth Contract

## Scenario: Mandatory Single-Owner Authentication With Future Actor Extensibility

### 1. Scope / Trigger

- Trigger: first-run setup, login/logout, session resolution/revocation, browser mutation, BFF→Core request, Actor authorization, or local account recovery.
- First complete delivery supports exactly one Owner Human account. Actor/Participant schema may represent future Humans, but multi-account product behavior is out of scope.
- Python Core owns account/session/authorization. Node BFF owns browser cookie/CSRF transport.

### 2. Signatures

```text
POST /auth/setup
POST /auth/login
POST /auth/logout
POST /auth/revoke-all
GET  /auth/session

fluctlight owner reset-password
fluctlight owner revoke-all-sessions
```

Authoritative records:

```text
OwnerAccount
  human_actor_id
  credential_hash / algorithm / parameters
  created_at / updated_at

Session
  id / token_hash
  human_actor_id
  created_at / expires_at / last_seen_at / revoked_at
  user_agent_hash / optional bounded audit metadata
```

The browser cookie contains an opaque random session token only. BFF→Core also carries a separate service credential/identity that cannot substitute for the Human session.

### 3. Contracts

- First startup creates no default username/password. One cryptographically random setup token is emitted through an explicit local channel and stored only as a hash/one-time state.
- Successful setup atomically creates the Owner Human Actor/account, consumes the setup token, and creates/requires a normal login session.
- Passwords use Argon2id through a vetted library with versioned parameters. Plaintext password/setup/session tokens are never logged or persisted.
- Sessions are opaque, random, hashed at rest, expiring, revocable, and rotated at login/privilege transition to prevent fixation.
- BFF sets `HttpOnly`, `Secure`, and `SameSite=Lax` cookie flags in production and enforces CSRF token/origin policy for state-changing browser requests.
- BFF forwards the opaque session; Core resolves the Human Actor and authorizes every command/query/media grant. Browser-supplied Actor IDs never establish authority.
- BFF service identity and Human session are independently required/validated. Core is not host-exposed by default.
- Production has no anonymous mode, automatic owner, default credential, or localhost-trust fallback.
- Local recovery CLI is explicit, audited, and revokes all sessions when resetting credentials.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Setup token missing/invalid/expired/consumed | Reject; create no Actor/account/session. |
| Owner already exists | Disable setup endpoint/state; reject second owner creation. |
| Password verification fails | Return generic bounded auth failure; do not reveal account existence or hash details. |
| Session absent/expired/revoked/hash mismatch | Treat as unauthenticated and clear browser cookie where applicable. |
| Mutation lacks valid CSRF/origin | Reject before calling Core command. |
| Browser submits another Actor ID | Ignore as authority and authorize against resolved session Actor; reject unauthorized target. |
| BFF service credential invalid | Reject request independently of Human session. |
| Password reset succeeds | Rotate credential revision and revoke all sessions atomically. |
| Core/DB is unavailable | Do not create local BFF-only session or cached authorization fallback. |

### 5. Good / Base / Bad Cases

- Good: local setup token creates one Owner, login rotates a secure session, and every Core command resolves the same Human Actor.
- Good: password reset from the NAS CLI revokes stolen browser sessions.
- Base: an expired cookie produces unauthenticated UI and no domain request.
- Bad: default admin/admin, JWT in localStorage, BFF trusting `actorId` from JSON, storing sessions only in Redis, or allowing anonymous mode on LAN.

### 6. Tests Required

- Setup tests for one-time use, race/double-submit, expiry, no default account, transaction rollback, and no secret logging.
- Argon2id tests for verification, parameter/version storage, rehash-on-policy-change path, bounded errors, and plaintext absence.
- Session tests for entropy, hash-at-rest, fixation rotation, expiry, last-seen policy, revoke, revoke-all, and concurrent requests.
- BFF cookie/CSRF tests for flags, origin/token validation, login/logout clearing, and no localStorage credential contract.
- Authorization tests for Actor spoofing, cross-Fluctlight/Conversation/Memory/media scope, and session-vs-service identity separation.
- Network/Compose tests proving only BFF is host-exposed by default.
- Recovery CLI tests for explicit local authorization, reset, audit, revoke-all, and failed partial operation rollback.

### 7. Wrong vs Correct

#### Wrong

```typescript
const actorId = request.body.actorId;
const session = jwt.decode(localStorageToken);
return core.command({actorId, session});
```

#### Correct

```typescript
const opaqueSession = request.cookies.fluctlightSession;
return coreClient.command(command, {
  humanSession: opaqueSession,
  serviceIdentity: bffServiceCredential,
});
```
