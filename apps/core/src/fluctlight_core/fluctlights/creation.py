"""Clean-start Fluctlight creation lifecycle application service."""

from __future__ import annotations

from dataclasses import dataclass, fields
from datetime import UTC, datetime, timedelta
from hashlib import sha256
from typing import Any, Protocol

from fluctlight_core.inner_state import (
    GoalEvidence,
    GoalSource,
    GoalStatus,
    InnerStateService,
    IntentionEvidence,
    SemanticTrigger,
)
from fluctlight_core.platform.timezones import canonical_timezone

from .contracts import (
    BehavioralPolicy,
    CreateFluctlight,
    FluctlightSnapshot,
    FoundationProvenance,
    FoundationValidationError,
    Identity,
    InitializationMode,
    LifeProfile,
    Personality,
    PersonalityUpdatePolicy,
)
from .service import FluctlightLifecycleError, FluctlightNotFoundError, FluctlightService


class CreationError(RuntimeError):
    """Raised when an analysis or activation request is not valid."""

    def __init__(
        self, message: str, *, code: str | None = None, details: dict[str, Any] | None = None
    ) -> None:
        super().__init__(message)
        self.code = code or "creation_invalid"
        self.details = details or {}


class InitializationAnalyzer(Protocol):
    async def analyze_initialization(self, description: str) -> dict[str, Any]: ...


class InitialAgencyInitializer(Protocol):
    async def ensure_for(
        self,
        fluctlight: FluctlightSnapshot,
        *,
        actor_id: str,
        goals: list[dict[str, Any]],
        intentions: list[dict[str, Any]],
    ) -> object: ...


@dataclass(frozen=True, slots=True)
class CreationPreview:
    identity: dict[str, Any]
    personality: dict[str, Any]
    behavioral_policy: dict[str, Any]
    life_profile: dict[str, Any]
    foundation_provenance: dict[str, Any]
    initial_goals: list[dict[str, Any]]
    initial_intentions: list[dict[str, Any]]
    provenance: dict[str, Any]

    def as_payload(self) -> dict[str, Any]:
        return {
            "initialization_mode": InitializationMode.LLM_DEFINED.value,
            "foundation": {
                "identity": self.identity,
                "personality": self.personality,
                "behavioral_policy": self.behavioral_policy,
                "life_profile": self.life_profile,
                "initial_goals": self.initial_goals,
                "initial_intentions": self.initial_intentions,
            },
            "provenance": {**self.provenance, "foundation": self.foundation_provenance},
        }


def _without_id(payload: dict[str, Any]) -> dict[str, Any]:
    result = dict(payload)
    result.pop("id", None)
    return result


def _identity_from_payload(identity_id: str, payload: dict[str, Any]) -> Identity:
    values = _without_id(payload)
    timezone = values.get("timezone")
    if timezone is not None:
        if not isinstance(timezone, str):
            raise FoundationValidationError("identity.timezone must be text")
        try:
            normalized = canonical_timezone(timezone)
        except ValueError as exc:
            raise FoundationValidationError("identity.timezone must be an IANA timezone") from exc
        values["timezone"] = normalized
    return Identity(id=identity_id, **values)


def _require_profile_fields(payload: dict[str, Any], *, required: set[str], name: str) -> None:
    missing = sorted(required - set(payload))
    if missing:
        raise FoundationValidationError(f"{name} is missing required fields: {', '.join(missing)}")


def _personality_from_payload(
    payload: dict[str, Any], *, require_complete_model_profile: bool = False
) -> Personality:
    values = dict(payload)
    if require_complete_model_profile:
        if "update_policy" in values:
            raise FoundationValidationError("personality.update_policy is server governed")
        _require_profile_fields(
            values,
            required={item.name for item in fields(Personality) if item.name != "update_policy"},
            name="personality",
        )
    update_policy = values.pop("update_policy", None)
    if update_policy is not None:
        if not isinstance(update_policy, dict):
            raise FoundationValidationError("personality.update_policy must be an object")
        values["update_policy"] = PersonalityUpdatePolicy(**update_policy)
    return Personality(**values)


def _behavioral_policy_from_payload(
    payload: dict[str, Any], *, require_complete_model_profile: bool = False
) -> BehavioralPolicy:
    values = dict(payload)
    if require_complete_model_profile:
        _require_profile_fields(
            values,
            required={item.name for item in fields(BehavioralPolicy)},
            name="behavioral_policy",
        )
    return BehavioralPolicy(**values)


def _life_profile_from_payload(
    payload: dict[str, Any], *, require_complete_model_profile: bool = False
) -> LifeProfile:
    values = dict(payload)
    # Canonicalize a single model-emitted entry to the declared array shape.
    # This preserves semantic content and does not invent a habit.
    for name in (
        "life_habits",
        "recurring_commitments",
        "relationship_seeds",
        "character_constraints",
    ):
        if isinstance(values.get(name), dict):
            values[name] = [values[name]]
    if require_complete_model_profile:
        _require_profile_fields(
            values,
            required={item.name for item in fields(LifeProfile)},
            name="life_profile",
        )
    return LifeProfile(**values)


def _foundation_provenance_from_payload(
    payload: dict[str, Any], *, require_complete: bool = False
) -> FoundationProvenance:
    values = dict(payload)
    sources = values.get("field_sources")
    if not isinstance(sources, dict):
        if require_complete:
            raise FoundationValidationError("provenance.field_sources is required")
        return FoundationProvenance(**values)
    completed = dict(sources)
    for path in _required_field_source_paths():
        completed.setdefault(path, "model_generated")
    values["field_sources"] = completed
    if require_complete and not completed:
        raise FoundationValidationError("provenance.field_sources is required")
    return FoundationProvenance(**values)


def _validate_field_sources(
    provenance: FoundationProvenance,
    identity: Identity,
    personality: Personality,
    policy: BehavioralPolicy,
    life_profile: LifeProfile,
) -> None:
    required = _required_field_source_paths()
    missing = sorted(required - set(provenance.field_sources))
    if missing:
        raise FoundationValidationError(
            f"provenance.field_sources is missing required paths: {', '.join(missing)}"
        )


def _required_field_source_paths() -> set[str]:
    return {
        *(f"identity.{item.name}" for item in fields(Identity) if item.name != "id"),
        *(
            f"personality.{item.name}"
            for item in fields(Personality)
            if item.name != "update_policy"
        ),
        *(f"behavioral_policy.{item.name}" for item in fields(BehavioralPolicy)),
        *(f"life_profile.{item.name}" for item in fields(LifeProfile)),
    }


def _complete_field_sources(
    values: dict[str, Any],
    identity: Identity,
    personality: Personality,
    policy: BehavioralPolicy,
    life_profile: LifeProfile,
) -> FoundationProvenance:
    return _foundation_provenance_from_payload(values, require_complete=True)


def _validate_life_profile_semantics(life_profile: LifeProfile) -> None:
    payload = life_profile.as_payload()
    missing = [name for name, value in payload.items() if not value]
    if missing:
        raise FoundationValidationError(
            f"life_profile cannot contain empty generated fields: {', '.join(missing)}"
        )


def _agency_list(payload: Any, name: str) -> list[dict[str, Any]]:
    if payload is None:
        return []
    if not isinstance(payload, list) or not all(isinstance(item, dict) for item in payload):
        raise FoundationValidationError(f"{name} must be an array of objects")
    if len(payload) > 8:
        raise FoundationValidationError(f"{name} cannot exceed 8 entries")
    return [dict(item) for item in payload]


def _validate_initial_agency(goals: list[dict[str, Any]], intentions: list[dict[str, Any]]) -> None:
    if not goals or not intentions:
        raise FoundationValidationError(
            "description creation requires initial goals and intentions"
        )
    for item in goals:
        if not isinstance(item.get("description"), str) or not item["description"].strip():
            raise FoundationValidationError("initial goal description is required")
        for field in ("importance", "urgency"):
            value = item.get(field)
            if not isinstance(value, int | float) or isinstance(value, bool) or not 0 <= value <= 1:
                raise FoundationValidationError(f"initial goal {field} must be between 0 and 1")
    for item in intentions:
        goal_index = item.get("goal_index")
        if not isinstance(goal_index, int) or goal_index < 0 or goal_index >= len(goals):
            raise FoundationValidationError("initial intention goal_index is required")
        if not isinstance(item.get("action"), str) or not item["action"].strip():
            raise FoundationValidationError("initial intention action is required")
        confidence = item.get("confidence")
        if (
            not isinstance(confidence, int | float)
            or isinstance(confidence, bool)
            or not 0 <= confidence <= 1
        ):
            raise FoundationValidationError("initial intention confidence must be between 0 and 1")
        expiration_hours = item.get("expiration_hours", 24)
        if (
            not isinstance(expiration_hours, int | float)
            or isinstance(expiration_hours, bool)
            or not 0 < expiration_hours <= 168
        ):
            raise FoundationValidationError(
                "initial intention expiration_hours must be between 0 and 168"
            )


@dataclass(slots=True)
class InitialAgencyService:
    inner_state: InnerStateService

    async def ensure_for(
        self,
        fluctlight: FluctlightSnapshot,
        *,
        actor_id: str,
        goals: list[dict[str, Any]],
        intentions: list[dict[str, Any]],
    ) -> None:
        if not goals and not intentions:
            return
        existing_goals, _ = await self.inner_state.goals_and_intentions(fluctlight.id)
        if existing_goals:
            return
        now = datetime.now(UTC)
        evidence_ref = f"foundation:{fluctlight.id}"
        created_goals = []
        for index, item in enumerate(goals):
            goal = await self.inner_state.create_goal(
                GoalEvidence(
                    fluctlight_id=fluctlight.id,
                    source=GoalSource.SELF,
                    description=str(item["description"]),
                    importance=float(item["importance"]),
                    urgency=float(item["urgency"]),
                    evidence_refs=(evidence_ref,),
                    goal_id=f"goal_initial_{fluctlight.id}_{index}",
                )
            )
            created_goals.append(
                await self.inner_state.transition_goal(
                    goal.id,
                    target=GoalStatus.ACTIVE,
                    expected_revision=goal.revision,
                    actor_id=actor_id,
                    reason="initialization",
                )
            )
        for index, item in enumerate(intentions):
            goal_index = item.get("goal_index")
            if (
                not isinstance(goal_index, int)
                or goal_index < 0
                or goal_index >= len(created_goals)
            ):
                raise FoundationValidationError("initial intention goal_index is invalid")
            expiration_hours = item.get("expiration_hours", 24)
            if not isinstance(expiration_hours, int | float) or isinstance(expiration_hours, bool):
                raise FoundationValidationError(
                    "initial intention expiration_hours must be numeric"
                )
            intention = await self.inner_state.create_intention(
                IntentionEvidence(
                    fluctlight_id=fluctlight.id,
                    goal_id=created_goals[goal_index].id,
                    action=str(item["action"]),
                    trigger=SemanticTrigger("semantic.trigger.v1", (evidence_ref,)),
                    confidence=float(item["confidence"]),
                    expiration=now + timedelta(hours=float(expiration_hours)),
                    evidence_refs=(evidence_ref,),
                    intention_id=f"intention_initial_{fluctlight.id}_{index}",
                )
            )
            await self.inner_state.qualify_intention(
                intention.id,
                expected_revision=intention.revision,
                actor_id=actor_id,
                reason="initialization",
            )


class CreationLifecycleService:
    """Analyze without persistence; activate one validated foundation atomically."""

    def __init__(
        self,
        fluctlights: FluctlightService,
        analyzer: InitializationAnalyzer,
        agency_initializer: InitialAgencyInitializer | None = None,
    ) -> None:
        self._fluctlights = fluctlights
        self._analyzer = analyzer
        self._agency_initializer = agency_initializer

    async def analyze_description(self, description: str) -> CreationPreview:
        if not isinstance(description, str) or not description.strip() or len(description) > 12_000:
            raise CreationError("description must contain 1 to 12000 characters")
        result = await self._analyzer.analyze_initialization(description.strip())
        foundation = result.get("foundation", result)
        if not isinstance(foundation, dict):
            raise CreationError("initialization response has no foundation")
        try:
            identity = _identity_from_payload("preview", dict(foundation["identity"]))
            personality = _personality_from_payload(
                dict(foundation["personality"]), require_complete_model_profile=True
            )
            policy = _behavioral_policy_from_payload(
                dict(foundation["behavioral_policy"]), require_complete_model_profile=True
            )
            life_profile = _life_profile_from_payload(
                dict(foundation["life_profile"]), require_complete_model_profile=True
            )
            foundation_provenance = _complete_field_sources(
                dict(foundation["provenance"]), identity, personality, policy, life_profile
            )
            _validate_field_sources(
                foundation_provenance, identity, personality, policy, life_profile
            )
            _validate_life_profile_semantics(life_profile)
            initial_goals = _agency_list(foundation.get("initial_goals"), "initial_goals")
            initial_intentions = _agency_list(
                foundation.get("initial_intentions"), "initial_intentions"
            )
            _validate_initial_agency(initial_goals, initial_intentions)
        except (KeyError, TypeError, FoundationValidationError) as exc:
            raise CreationError(
                "initialization response is not a valid Fluctlight foundation",
                code="initialization_foundation_invalid",
                details={"validation_error": str(exc), "error_type": type(exc).__name__},
            ) from exc
        provenance = result.get("provenance", {})
        if not isinstance(provenance, dict):
            provenance = {}
        return CreationPreview(
            _without_id(identity.as_payload()),
            personality.as_payload(),
            policy.as_payload(),
            life_profile.as_payload(),
            foundation_provenance.as_payload(),
            initial_goals,
            initial_intentions,
            dict(provenance),
        )

    async def activate(
        self,
        *,
        actor_id: str,
        request_id: str,
        initialization_mode: InitializationMode,
        identity: dict[str, Any],
        personality: dict[str, Any] | None = None,
        behavioral_policy: dict[str, Any] | None = None,
        life_profile: dict[str, Any] | None = None,
        foundation_provenance: dict[str, Any] | None = None,
        initial_goals: list[dict[str, Any]] | None = None,
        initial_intentions: list[dict[str, Any]] | None = None,
    ) -> FluctlightSnapshot:
        if not isinstance(request_id, str) or not request_id.strip() or len(request_id) > 256:
            raise CreationError("request_id is required", code="activation_request_invalid")
        fluctlight_id = f"fluctlight_{sha256(f'{actor_id}:{request_id}'.encode()).hexdigest()[:32]}"
        mode = InitializationMode(initialization_mode)
        if mode is InitializationMode.BLANK_SLATE and (
            personality is not None
            or behavioral_policy is not None
            or life_profile is not None
            or foundation_provenance is not None
        ):
            raise CreationError(
                "blank_slate does not accept personality or behavioral policy input",
                code="activation_mode_invalid",
            )
        try:
            resolved_identity = _identity_from_payload(fluctlight_id, identity)
            resolved_personality = (
                _personality_from_payload(personality)
                if mode is InitializationMode.LLM_DEFINED and personality is not None
                else Personality.neutral()
            )
            resolved_policy = (
                _behavioral_policy_from_payload(behavioral_policy)
                if mode is InitializationMode.LLM_DEFINED and behavioral_policy is not None
                else BehavioralPolicy()
            )
            resolved_life_profile = (
                _life_profile_from_payload(life_profile)
                if mode is InitializationMode.LLM_DEFINED and life_profile is not None
                else LifeProfile()
            )
            resolved_provenance = (
                _foundation_provenance_from_payload(foundation_provenance)
                if mode is InitializationMode.LLM_DEFINED and foundation_provenance is not None
                else FoundationProvenance()
            )
            resolved_goals = _agency_list(initial_goals, "initial_goals")
            resolved_intentions = _agency_list(initial_intentions, "initial_intentions")
            if mode is InitializationMode.LLM_DEFINED:
                _validate_initial_agency(resolved_goals, resolved_intentions)
        except (TypeError, FoundationValidationError) as exc:
            raise CreationError(
                "activation foundation is invalid",
                code="activation_foundation_invalid",
                details={"validation_error": str(exc), "error_type": type(exc).__name__},
            ) from exc
        if mode is InitializationMode.LLM_DEFINED and (
            personality is None
            or behavioral_policy is None
            or life_profile is None
            or foundation_provenance is None
        ):
            raise CreationError(
                "llm_defined activation requires the reviewed complete foundation",
                code="activation_foundation_incomplete",
            )
        if mode is InitializationMode.BLANK_SLATE and (resolved_goals or resolved_intentions):
            raise CreationError(
                "blank_slate cannot accept initial agency", code="activation_mode_invalid"
            )
        try:
            existing = await self._fluctlights.get(fluctlight_id)
        except FluctlightNotFoundError:
            existing = None
        if existing is not None:
            if (
                existing.initialization_mode is not mode
                or existing.identity != resolved_identity
                or existing.personality != resolved_personality
                or existing.behavioral_policy != resolved_policy
                or existing.life_profile != resolved_life_profile
                or existing.provenance != resolved_provenance
            ):
                raise CreationError(
                    "request_id was reused with different foundation data",
                    code="activation_request_conflict",
                )
            if self._agency_initializer is not None:
                await self._agency_initializer.ensure_for(
                    existing,
                    actor_id=actor_id,
                    goals=resolved_goals,
                    intentions=resolved_intentions,
                )
            return existing
        try:
            created = await self._fluctlights.create(
                CreateFluctlight(
                    actor_id=actor_id,
                    id=fluctlight_id,
                    initialization_mode=mode,
                    identity=resolved_identity,
                    personality=resolved_personality,
                    behavioral_policy=resolved_policy,
                    life_profile=resolved_life_profile,
                    provenance=resolved_provenance,
                )
            )
            if self._agency_initializer is not None:
                await self._agency_initializer.ensure_for(
                    created,
                    actor_id=actor_id,
                    goals=resolved_goals,
                    intentions=resolved_intentions,
                )
            return created
        except FluctlightLifecycleError as exc:
            raise CreationError(
                "activation could not be completed",
                code="activation_persistence_failed",
            ) from exc
