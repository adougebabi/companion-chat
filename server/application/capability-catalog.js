/**
 * Canonical transport descriptors for the native companion capabilities.
 *
 * This module intentionally contains only immutable data. Capability
 * validation, marker compatibility, and persistence remain application-flow
 * concerns; consumers can pass the OpenAI-compatible tools directly to an LLM
 * transport without importing the runtime composition layer.
 */

function deepFreeze(value) {
    if (!value || typeof value !== 'object' || Object.isFrozen(value)) return value;
    Object.freeze(value);
    for (const child of Object.values(value)) deepFreeze(child);
    return value;
}

const DESCRIPTORS = {
    scene_event: {
        name: 'scene_event',
        description: 'Persist a material shared-scene start, switch, or end.',
        parameters: {
            type: 'object',
            additionalProperties: false,
            required: ['operation'],
            properties: {
                operation: {type: 'string', enum: ['start', 'switch', 'end']},
                location: {type: 'string', maxLength: 160},
                room: {type: 'string', maxLength: 120},
                activity: {type: 'string', maxLength: 160},
                situation: {type: 'string', maxLength: 240},
                mood: {type: 'string', maxLength: 80},
                objects: {type: 'array', maxItems: 12, items: {type: 'string', maxLength: 80}},
                participants: {type: 'array', maxItems: 2, items: {type: 'string', enum: ['user', 'persona']}}
            }
        }
    },
    appearance_event: {
        name: 'appearance_event',
        description: 'Persist a material outfit change for the current persona.',
        parameters: {
            type: 'object',
            additionalProperties: false,
            required: ['operation'],
            properties: {
                operation: {type: 'string', enum: ['set', 'clear']},
                outfit: {type: 'string', maxLength: 240},
                reason: {type: 'string', maxLength: 240}
            }
        }
    },
    media_event: {
        name: 'media_event',
        description: 'Queue a validated image or video delivery.',
        parameters: {
            type: 'object',
            additionalProperties: false,
            required: ['kind', 'request', 'count', 'personaMediaConcept'],
            properties: {
                kind: {type: 'string', enum: ['image', 'video']},
                request: {type: 'string', maxLength: 500},
                count: {type: 'integer', minimum: 1, maximum: 3},
                personaMediaConcept: {type: 'object'}
            }
        }
    },
    pending_event: {
        name: 'pending_event',
        description: 'Register one bounded, explicit follow-up fact for durable later evaluation.',
        parameters: {
            type: 'object',
            additionalProperties: false,
            required: ['schemaVersion', 'summary', 'notBefore', 'expiresAt', 'dedupeKey'],
            properties: {
                schemaVersion: {type: 'integer', enum: [1]},
                summary: {type: 'string', minLength: 1, maxLength: 280},
                notBefore: {type: 'string', maxLength: 80},
                expiresAt: {type: 'string', maxLength: 80},
                dedupeKey: {type: 'string', minLength: 1, maxLength: 120}
            }
        }
    },
    memory_event: {
        name: 'memory_event',
        description: 'Explicitly record one persona-private memory from the current user message.',
        parameters: {
            type: 'object',
            additionalProperties: false,
            required: ['memory'],
            properties: {
                memory: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['operation', 'key', 'value', 'confidence', 'idempotencyKey'],
                    properties: {
                        schemaVersion: {type: 'integer', enum: [1]},
                        operation: {type: 'string', enum: ['insert', 'upsert']},
                        key: {type: 'string', minLength: 1, maxLength: 120},
                        value: {type: 'string', minLength: 1, maxLength: 2_000},
                        confidence: {type: 'number', minimum: 0, maximum: 1},
                        sourceType: {type: 'string', maxLength: 80},
                        sourceId: {type: 'string', maxLength: 240},
                        sourceMessageId: {type: 'string', maxLength: 160},
                        idempotencyKey: {type: 'string', minLength: 1, maxLength: 240}
                    }
                }
            }
        }
    },
    affect_event: {
        name: 'affect_event',
        description: 'Report one bounded hidden affect event; the server owns PAD deltas.',
        parameters: {
            type: 'object',
            additionalProperties: false,
            required: ['event'],
            properties: {
                event: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['type', 'confidence', 'idempotencyKey'],
                    properties: {
                        type: {type: 'string', enum: ['social_connection', 'social_friction', 'exploration_discovery', 'exploration_blocked', 'restored', 'fatigue']},
                        confidence: {type: 'number', minimum: 0, maximum: 1},
                        idempotencyKey: {type: 'string', minLength: 1, maxLength: 240}
                    }
                }
            }
        }
    },
    drive_signal: {
        name: 'drive_signal',
        description: 'Report one bounded drive pressure change; the server owns the magnitude.',
        parameters: {
            type: 'object',
            additionalProperties: false,
            required: ['signal'],
            properties: {
                signal: {
                    type: 'object',
                    additionalProperties: false,
                    required: ['drive', 'direction', 'confidence', 'idempotencyKey'],
                    properties: {
                        drive: {type: 'string', pattern: '^[a-z][a-z0-9_:-]{0,79}$'},
                        direction: {type: 'string', enum: ['increase_pressure', 'decrease_pressure', 'neutral']},
                        confidence: {type: 'number', minimum: 0, maximum: 1},
                        idempotencyKey: {type: 'string', minLength: 1, maxLength: 240}
                    }
                }
            }
        }
    }
};

const DESCRIPTOR_ORDER = [
    'scene_event',
    'appearance_event',
    'media_event',
    'pending_event',
    'memory_event',
    'affect_event',
    'drive_signal'
];

const frozenDescriptors = Object.fromEntries(
    DESCRIPTOR_ORDER.map(name => [name, deepFreeze(DESCRIPTORS[name])])
);

const frozenCatalog = deepFreeze(frozenDescriptors);

const frozenTools = deepFreeze(DESCRIPTOR_ORDER.map(name => ({
    type: 'function',
    function: frozenCatalog[name]
})));

/** Frozen descriptor map keyed by the native capability name. */
export const CAPABILITY_DESCRIPTORS = frozenCatalog;

/** Explicit descriptor-map name for consumers that distinguish tools from schemas. */
export const CAPABILITY_TOOL_DESCRIPTORS = CAPABILITY_DESCRIPTORS;

/** Frozen descriptors in the stable transport order. */
export const CAPABILITY_DESCRIPTOR_ORDER = deepFreeze(DESCRIPTOR_ORDER.map(name => frozenCatalog[name]));

/** OpenAI-compatible `tools` payload in the stable transport order. */
export const CAPABILITY_TOOLS = frozenTools;

/** Canonical catalog alias for callers that prefer the catalog terminology. */
export const CAPABILITY_CATALOG = CAPABILITY_DESCRIPTORS;

/** Return every native tool in canonical order; this catalog does not filter. */
export function getAllCapabilityTools() {
    return CAPABILITY_TOOLS;
}

export const getAllTools = getAllCapabilityTools;

export default CAPABILITY_CATALOG;
