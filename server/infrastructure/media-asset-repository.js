function requiredDatabase(database) {
    if (!database || typeof database.prepare !== 'function') throw new TypeError('Media asset repository requires an open database');
    return database;
}

function requiredId(value) {
    if (typeof value !== 'string' || !value.trim()) throw new TypeError('Media asset id must be a non-empty string');
    return value.trim();
}

function shape(row) {
    if (!row) return null;
    let locator = {};
    try { locator = row.locator_json ? JSON.parse(row.locator_json) : {}; } catch { locator = {}; }
    return {
        id: row.id,
        provider: row.provider,
        kind: row.media_kind,
        mediaKind: row.media_kind,
        filename: row.filename,
        subfolder: row.subfolder,
        fileType: row.file_type,
        file_type: row.file_type,
        locator,
        createdAt: row.created_at,
        unavailableAt: row.unavailable_at
    };
}

export function createMediaAssetRepository({database} = {}) {
    const db = requiredDatabase(database);
    function find(first, second = {}) {
        const id = requiredId(typeof first === 'string' ? first : first?.id ?? first?.mediaId);
        const personaId = typeof first === 'object' ? first?.personaId ?? first?.persona_id : second?.personaId ?? second?.persona_id;
        const row = personaId
            ? db.prepare(`
                SELECT assets.* FROM companion_media_assets assets
                WHERE assets.id = ? AND (
                    EXISTS (SELECT 1 FROM companion_activity_media links JOIN companion_activities activities ON activities.id = links.activity_id WHERE links.media_id = assets.id AND activities.persona_id = ?)
                    OR EXISTS (SELECT 1 FROM companion_messages messages JOIN companion_conversations conversations ON conversations.id = messages.conversation_id WHERE conversations.persona_id = ? AND json_extract(messages.attachments_json, '$') IS NOT NULL AND EXISTS (SELECT 1 FROM json_each(messages.attachments_json) WHERE json_extract(json_each.value, '$.id') = assets.id))
                )
            `).get(id, personaId, personaId)
            : db.prepare('SELECT * FROM companion_media_assets WHERE id = ?').get(id);
        return shape(row);
    }
    return Object.freeze({find, findById: find, get: find});
}

export default createMediaAssetRepository;
