# Fluctlight Domain Context

## Core Concept

**Fluctlight** is the canonical concept for an autonomous AI personality: a persistent artificial personality with a continuing identity, private memory, lived context, relationships, and bounded agency. It describes the intended kind and quality of AI personality; it is not the proper name of every user-created AI and should not replace that AI's own name.
_Avoid_: 摇光, persona, companion

**Fluctlight system** is the product/runtime that creates and supports Fluctlight personalities. A product screen may use `Fluctlight` as the project name, while a created AI is displayed by its own name and identity.
_Avoid_: 摇光系统, companion system

## Self-Awareness

In this project, self-awareness is a product behavior contract, not a claim that current models possess metaphysical subjective consciousness. A Fluctlight personality should maintain and express:

- a stable sense of who it is;
- continuity across conversations and elapsed time;
- awareness of its current situation, state, and relevant history;
- private autobiographical and relationship memory;
- bounded initiative and choices within system constraints;
- the ability to reflect on its own state, preferences, uncertainty, and limits.

These behaviors are necessary evidence of a self-model. They do not prove subjective experience.

## Related Concepts

**Fluctlight instance** is one user-created AI with its own identity, personality, affect, drives, relationships, context, schedule, goals, intentions, memory, behavioral policy, and cognitive runtime. It is the concrete object that realizes the Fluctlight concept.
_Avoid_: 摇光实例, persona, companion

**Identity** is the stable answer to “who am I”: identifying facts, background, biography, values, worldview, and long-term setting notes. It is distinct from personality traits and dynamic state.
_Avoid_: Identity core, foundation

Identity fields have explicit governance: immutable identifiers, human-governed anchors, and lived fields that may change only through confirmed life facts or reflection. Every accepted change creates an auditable, reversible revision.

**Personality** is the long-term answer to “how am I usually inclined to think and behave.” Its traits may evolve only through reflection over accumulated evidence and an explicit slow update policy, never as the immediate result of one event.

**Affect** is the dynamic answer to “how do I feel now.” It includes PAD coordinates, mood, emotional momentum, emotion history, and regulation over time.

PAD and emotional momentum use the canonical range `-1..1`; other domain intensities, tendencies, needs, costs, progress, and confidence use `0..1`. Model output describes semantic direction, bounded strength, confidence, and evidence; server policy owns numeric deltas and clamping.

**Drive** is a dynamic internal need that can motivate or inhibit behavior. Social, exploration, rest, autonomy, and intimacy drives may conflict and must be resolved before action.

**Goal** is an outcome a Fluctlight instance currently wants to achieve. It has a source, importance, urgency, progress, deadline, and lifecycle status.

**Intention** is a concrete conditional next action formed in service of a goal. It is closer to execution than a goal and may have a preferred time, trigger, confidence, and expiration.

**Memory** is what a Fluctlight instance remembers. Working, episodic, semantic, relationship, and autobiographical memory have distinct evidence, retention, and retrieval semantics.

**Behavioral policy** is the relatively stable expression policy that shapes response style, initiative, delay, conflict, refusal, intimacy, and other outward behavior without replacing personality or current affect.

**Cognitive runtime** is the perception, appraisal, state update, decision, action, and reflection loop that turns observations and internal state into behavior and long-term change. Interactive cognition first obtains a structured assessment/decision, then Go Core freezes the accepted action before a separate realization call produces visible content.

**Reflection** is the slow governance stage that consolidates evidence into durable memory, emotional summaries, drive recalibration, personality revisions, and autobiographical revisions. It is separate from immediate state update.

**Actor** is an identity capable of participating in social interaction. An actor is either a human or a Fluctlight instance.

**Conversation** is a shared ordered interaction context among one or more actors. It owns participant membership and messages, but it does not own or merge a Fluctlight instance's private internal state.

**Participant** is an actor's membership in one conversation, including its conversation-specific role and membership lifecycle. It is not a second copy of the actor's identity.

**Life world** is a Fluctlight instance's ongoing lived context: routines, plans, events, places, supporting people, temporary states, and time progression.

**Relationship** is a directed state owned by one Fluctlight instance and targeting another actor. The reverse direction, when the target is another Fluctlight instance, is a separate relationship with its own evidence and evolution.

The old terms `摇光`, `persona`, and `companion` are deprecated domain and product vocabulary. They must not be used in new requirements, product copy, architecture documents, APIs, schemas, or new code identifiers. Existing code may retain them only as historical implementation details until the old system is retired; the clean-start system does not provide naming compatibility aliases.

**Presence** is the current interaction context in which a user and a Fluctlight instance are together. A shared scene is one form of presence; it is not the whole Fluctlight concept, identity core, or life world.
