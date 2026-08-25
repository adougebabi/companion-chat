# Fluctlight Domain Model Source

## Status

Normalized from the user's attached definition on 2026-08-24. This is a source requirement for the clean-start system, not a description of the old implementation. The prohibited old product name in examples has been normalized to `Fluctlight`.

## Identity

Defines “who I am” and is the most stable foundation.

`id`, `name`, `age`, `gender`, `occupation`, `residence`, `timezone`, `birthday`, `background`, `biography`, `core_values`, `worldview`, `notes`.

Confirmed governance: `id` is immutable. Other fields receive an explicit mutability class: human-governed anchor or lived field. Lived fields may change only from confirmed life facts or reflection. Every accepted change creates an auditable, reversible revision.

## Personality

Defines “who I usually am” as long-term tendencies. Most traits do not change because of one event.

`openness`, `conscientiousness`, `extraversion`, `agreeableness`, `neuroticism`, `curiosity`, `independence`, `patience`, `empathy`, `assertiveness`, `humor`, `sociability`, `risk_tolerance`, and `update_policy` defining the allowed rate of long-term change.

Confirmed governance: a single interaction or immediate appraisal cannot update personality. Reflection may propose small changes only from accumulated cross-event evidence and within the configured evidence window, maximum delta, and cooldown. Accepted changes are versioned, auditable, user-correctable, and reversible.

## Affect

Defines “how I feel now” as a continuously changing emotional system.

Confirmed numeric contract: PAD and emotional momentum use `-1..1`. Other normalized intensities, tendencies, needs, costs, progress, and confidence use `0..1`. Model output does not own numeric deltas; Python policy calculates and clamps them from structured semantic signals and elapsed wall time.

### PAD

- `pleasure`: positive for happiness/satisfaction/liking; negative for disgust/dissatisfaction/pain.
- `arousal`: high for excitement/tension/activity; low for calm/fatigue/sleepiness.
- `dominance`: high for assertive/active/in-control; low for compliant/withdrawn/passive.

### Mood

`label`, `intensity`, `source`, `started_at`, `expected_decay_at`.

### Emotional Momentum

`pleasure_momentum`, `arousal_momentum`, `dominance_momentum`, `decay_rate`.

Momentum prevents an ordinary next message from immediately resetting an accumulated emotional response.

### Emotion History

`timestamp`, `event_id`, `previous_pad`, `delta`, `resulting_pad`, `cause`, `intensity`.

### Regulation

`natural_decay_rate`, `sleep_recovery`, `positive_event_recovery`, `negative_event_amplification`, `emotional_stability`.

## Drives

Defines “what I currently want” as internal motivation. Each drive has `level`, `baseline`, `growth_rate`, `satisfaction_rate`, and `urgency_threshold`.

- `social`
- `exploration`
- `rest`
- `autonomy`: desire to be alone or act independently
- `intimacy`: desire to establish or maintain close relationships
- `drive_conflict`: resolves incompatible simultaneous needs before behavior is selected

## Relationships

Defines how this Fluctlight instance regards one concrete Actor. The source draft used `user_id`; planning replaced it with a typed Actor reference so Human and Fluctlight targets use the same directed model.

`target_actor_id` plus `intimacy`, `trust`, `familiarity`, `attachment`, `respect`, `affection`, `annoyance`, `psychological_safety`, `dependence`, `interaction_frequency`, `last_interaction_at`, `last_meaningful_interaction_at`, `relationship_trend` (`improving | stable | declining`), `relationship_summary`, and `emotional_association`.

## Context

Defines the current situation.

`scene`, `activity`, `location`, `time`, `day_of_week`, `weather`, `people_present`, `user_presence`, `current_task`, `interruption_level`, `environment`.

## Schedule

Defines the intended life plan for one local date. It is not cron and may be replanned by events, affect, drives, interaction, or goals.

- `date`
- `timezone`
- `generated_at`
- `generated_from`
- `items[]`
  - `id`
  - `start_at`
  - `end_at`
  - `activity`
  - `scene`
  - `priority`
  - `flexibility`
  - `interruption_cost`
  - `status`: `planned | active | completed | skipped`
- `reschedule_policy`

## Goals

Defines near-term desired outcomes.

- `active[]`
  - `id`
  - `source`: `drive | event | user | self`
  - `description`
  - `importance`
  - `urgency`
  - `progress`
  - `deadline`
  - `status`
- `completed[]`

## Intentions

Defines concrete next actions, closer to execution than goals.

- `pending[]`
  - `id`
  - `goal_id`
  - `action`
  - `preferred_time`
  - `trigger_condition`
  - `confidence`
  - `expiration`
- `history[]`

## Memory

Defines what a Fluctlight instance remembers.

### Working

`recent_messages`, `active_topics`, `unresolved_questions`, `current_conversation_state`.

### Episodic

`event_id`, `timestamp`, `scene`, `people`, `content`, `emotional_significance`, `importance`.

### Semantic

`fact`, `confidence`, `source`, `created_at`, `last_confirmed_at`.

### Relationship

Counterpart reference, `important_events`, `relationship_turning_points`, `preferences`, `emotional_associations`.

### Autobiographical

`life_events`, `important_people`, `important_places`, `achievements`, `failures`, `identity_forming_events`.

## Behavioral Policy

Defines habitual outward expression.

`response_style`, `message_length`, `emoji_frequency`, `punctuation_style`, `humor_style`, `sarcasm_tendency`, `directness`, `initiative`, `topic_initiation`, `silence_tolerance`, `response_delay`, `emotional_expression`, `conflict_style`, `refusal_style`, `intimacy_expression`.

## Cognitive Runtime

The system that makes a Fluctlight instance operate.

Confirmed ownership: LLM structured output owns semantic perception, appraisal, candidate decision, and reflection. Python owns authoritative facts, validation, numeric policy, safety, transactions, workflow, and action execution. Code heuristics and default semantic fallbacks are prohibited; failure yields explicit failure, retry, `deferred`, `no_op`, or terminal failure.

Confirmed interaction shape: an invisible structured assessment/decision call precedes Python validation, state update, and final-decision freeze. A separate realization call produces visible content only for the frozen action and cannot introduce new semantic state. Reflection is always asynchronous.

### Perception

`event_classifier`, `intent_detection`, `sentiment_detection`, `social_signal_detection`, `environment_interpretation`.

### Appraisal

`relevance`, `goal_congruence`, `reward`, `loss`, `social_threat`, `controllability`, `responsibility`, `relationship_significance`, `expected_effect`.

### State Update

`update_pad`, `update_mood`, `update_emotional_momentum`, `update_drives`, `update_relationship`, `update_context`, `update_goals`.

### Decision

Candidate actions: `reply_now`, `delay_reply`, `ignore`, `send_short_reply`, `change_topic`, `initiate_topic`, `change_scene`, `create_feed`, `generate_media`, `update_schedule`.

Decision factors: `action_scoring`, `drive_conflict_resolution`, `relationship_cost`, `emotional_cost`, `social_cost`, `final_decision`.

### Action

`send_message`, `delay_message`, `ignore_message`, `change_scene`, `create_goal`, `create_intention`, `create_memory`, `update_relationship`, `create_feed`, `generate_media`, `update_schedule`.

### Reflection

`daily_reflection`, `relationship_reflection`, `memory_consolidation`, `emotional_summary`, `drive_recalibration`, `personality_update`, `autobiographical_update`.
