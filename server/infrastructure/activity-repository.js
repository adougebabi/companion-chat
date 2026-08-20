function assertOpenDatabase(database) {
    if (!database || typeof database.prepare !== 'function') {
        throw new TypeError('Activity repository requires an open database');
    }
    return database;
}

function inputFor(first, second = {}) {
    if (typeof first === 'string') return {...second, activityId: first};
    if (!first || typeof first !== 'object' || Array.isArray(first)) {
        throw new TypeError('Activity repository input must be an object');
    }
    return first;
}

function requiredText(value, field) {
    if (typeof value !== 'string' || !value) throw new TypeError(`${field} must be a non-empty string`);
    return value;
}

function textColumn(value, field) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    return value;
}

function nullableText(value, field) {
    if (value === null || value === undefined) return null;
    return requiredText(value, field);
}

function positiveInteger(value, field) {
    if (!Number.isInteger(value) || value < 1) throw new RangeError(`${field} must be a positive integer`);
    return value;
}

function nonNegativeInteger(value, field) {
    if (!Number.isInteger(value) || value < 0) throw new RangeError(`${field} must be a non-negative integer`);
    return value;
}

function activityCursor(cursor) {
    if (cursor === undefined || cursor === null) return null;
    if (!cursor || typeof cursor !== 'object' || Array.isArray(cursor)) {
        throw new TypeError('Activity cursor must be an object');
    }
    return {
        createdAt: requiredText(cursor.createdAt ?? cursor.created_at, 'Activity cursor.createdAt'),
        id: requiredText(cursor.id, 'Activity cursor.id')
    };
}

function activityFields(input) {
    return {
        id: requiredText(input.id, 'Activity.id'),
        personaId: requiredText(input.personaId ?? input.persona_id, 'Activity.personaId'),
        eventId: nullableText(input.eventId ?? input.event_id, 'Activity.eventId'),
        content: textColumn(input.content, 'Activity.content'),
        mediaMode: input.mediaMode ?? input.media_mode ?? 'none',
        mediaStatus: input.mediaStatus ?? input.media_status ?? 'none',
        createdAt: requiredText(input.createdAt ?? input.created_at, 'Activity.createdAt')
    };
}

function commentFields(input) {
    return {
        id: requiredText(input.id, 'Activity comment.id'),
        activityId: requiredText(input.activityId ?? input.activity_id, 'Activity comment.activityId'),
        parentCommentId: nullableText(input.parentCommentId ?? input.parent_comment_id, 'Activity comment.parentCommentId'),
        authorKind: requiredText(input.authorKind ?? input.author_kind, 'Activity comment.authorKind'),
        authorPersonaId: nullableText(input.authorPersonaId ?? input.author_persona_id, 'Activity comment.authorPersonaId'),
        supportingCharacterId: nullableText(input.supportingCharacterId ?? input.supporting_character_id, 'Activity comment.supportingCharacterId'),
        content: textColumn(input.content, 'Activity comment.content'),
        createdAt: requiredText(input.createdAt ?? input.created_at, 'Activity comment.createdAt')
    };
}

/**
 * Create a table-scoped repository for companion activities and interactions.
 *
 * This adapter accepts the application's already-open SQLite connection and
 * returns raw rows. IDs, timestamps, cursor serialization, visibility policy,
 * unread policy, and browser DTO shaping remain with the caller. Methods can
 * be composed inside a caller-owned better-sqlite3 transaction.
 */
export function createActivityRepository({database} = {}) {
    const openDatabase = assertOpenDatabase(database);

    function findActivity(first, second = {}) {
        const input = inputFor(first, second);
        const activityId = requiredText(input.activityId ?? input.id, 'Activity.id');
        const values = [activityId];
        let ownerFilter = '';
        if (input.personaId !== undefined && input.personaId !== null) {
            ownerFilter = ' AND owners.id = ?';
            values.push(requiredText(input.personaId, 'Activity.personaId'));
        }
        return openDatabase.prepare(`
            SELECT activities.*
            FROM companion_activities activities
            JOIN companion_personas owners ON owners.id = activities.persona_id
            WHERE activities.id = ?${ownerFilter}
        `).get(...values);
    }

    function requireActivity(input) {
        const activity = findActivity(input);
        if (!activity) {
            throw new Error(input.personaId ? 'Activity does not belong to persona' : 'Activity does not exist');
        }
        return activity;
    }

    function listActivities(first = {}) {
        const input = inputFor(first);
        const limit = input.limit === undefined ? 20 : positiveInteger(Number(input.limit), 'Activity limit');
        if (limit > 50) throw new RangeError('Activity limit must not exceed 50');
        const cursor = activityCursor(input.cursor);
        const values = [];
        const filters = [];

        if (input.personaId !== undefined && input.personaId !== null) {
            filters.push('owners.id = ?');
            values.push(requiredText(input.personaId, 'Activity.personaId'));
        }

        const visibility = input.visibility === undefined ? 'visible' : input.visibility;
        if (visibility === 'visible') {
            filters.push(`NOT EXISTS (
                SELECT 1 FROM companion_activity_visibility visibility
                WHERE visibility.activity_id = activities.id AND visibility.hidden_at IS NOT NULL
            )`);
        } else if (visibility === 'hidden') {
            filters.push(`EXISTS (
                SELECT 1 FROM companion_activity_visibility visibility
                WHERE visibility.activity_id = activities.id AND visibility.hidden_at IS NOT NULL
            )`);
        } else if (visibility !== 'all') {
            throw new TypeError('Activity visibility must be visible, hidden, or all');
        }

        if (cursor) {
            filters.push('(activities.created_at < ? OR (activities.created_at = ? AND activities.id < ?))');
            values.push(cursor.createdAt, cursor.createdAt, cursor.id);
        }

        const where = filters.length ? `WHERE ${filters.join(' AND ')}` : '';
        return openDatabase.prepare(`
            SELECT activities.*
            FROM companion_activities activities
            JOIN companion_personas owners ON owners.id = activities.persona_id
            ${where}
            ORDER BY activities.created_at DESC, activities.id DESC
            LIMIT ?
        `).all(...values, limit);
    }

    function insertActivity(first) {
        const input = activityFields(inputFor(first));
        openDatabase.prepare(`
            INSERT INTO companion_activities (
                id, persona_id, event_id, content, media_mode, media_status, created_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?)
        `).run(
            input.id, input.personaId, input.eventId, input.content,
            input.mediaMode, input.mediaStatus, input.createdAt
        );
        return openDatabase.prepare('SELECT * FROM companion_activities WHERE id = ?').get(input.id);
    }

    function listActivityComments(first, second = {}) {
        const input = inputFor(first, second);
        const activity = requireActivity(input);
        const limit = input.limit === undefined ? 50 : positiveInteger(Number(input.limit), 'Activity comment limit');
        if (limit > 100) throw new RangeError('Activity comment limit must not exceed 100');
        return openDatabase.prepare(`
            SELECT comments.*
            FROM companion_activity_comments comments
            JOIN companion_activities activities ON activities.id = comments.activity_id
            JOIN companion_personas owners ON owners.id = activities.persona_id
            WHERE comments.activity_id = ?${input.personaId === undefined || input.personaId === null ? '' : ' AND owners.id = ?'}
            ORDER BY comments.created_at, comments.id
            LIMIT ?
        `).all(...(input.personaId === undefined || input.personaId === null
            ? [activity.id, limit]
            : [activity.id, requiredText(input.personaId, 'Activity.personaId'), limit]));
    }

    function insertActivityComment(first, second = {}) {
        const input = commentFields(inputFor(first, second));
        const personaId = second.personaId ?? (typeof first === 'object' ? first.personaId : undefined);
        const scope = {...input, personaId};
        requireActivity(scope);
        openDatabase.prepare(`
            INSERT INTO companion_activity_comments (
                id, activity_id, parent_comment_id, author_kind, author_persona_id,
                supporting_character_id, content, created_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        `).run(
            input.id, input.activityId, input.parentCommentId, input.authorKind,
            input.authorPersonaId, input.supportingCharacterId, input.content, input.createdAt
        );
        return openDatabase.prepare('SELECT * FROM companion_activity_comments WHERE id = ?').get(input.id);
    }

    function listActivityMedia(first, second = {}) {
        const input = inputFor(first, second);
        const activity = requireActivity(input);
        const values = [activity.id];
        let ownerFilter = '';
        if (input.personaId !== undefined && input.personaId !== null) {
            ownerFilter = ' AND owners.id = ?';
            values.push(requiredText(input.personaId, 'Activity.personaId'));
        }
        return openDatabase.prepare(`
            SELECT activity_media.*
            FROM companion_activity_media activity_media
            JOIN companion_activities activities ON activities.id = activity_media.activity_id
            JOIN companion_personas owners ON owners.id = activities.persona_id
            WHERE activity_media.activity_id = ?${ownerFilter}
            ORDER BY activity_media.position, activity_media.media_id
        `).all(...values);
    }

    function insertActivityMedia(first, second = {}) {
        const input = inputFor(first, second);
        const activityId = requiredText(input.activityId ?? input.activity_id, 'Activity media.activityId');
        const mediaId = requiredText(input.mediaId ?? input.media_id, 'Activity media.mediaId');
        const position = nonNegativeInteger(input.position, 'Activity media.position');
        requireActivity({...input, activityId});
        openDatabase.prepare(`
            INSERT OR IGNORE INTO companion_activity_media (activity_id, media_id, position)
            VALUES (?, ?, ?)
        `).run(activityId, mediaId, position);
        return openDatabase.prepare(`
            SELECT * FROM companion_activity_media WHERE activity_id = ? AND media_id = ?
        `).get(activityId, mediaId);
    }

    function reactionActivity(first, second = {}) {
        const input = inputFor(first, second);
        const activityId = requiredText(input.activityId ?? input.id, 'Activity.id');
        const personaId = input.personaId;
        const liked = input.liked;
        if (typeof liked !== 'boolean') throw new TypeError('Activity reaction.liked must be a boolean');
        requireActivity({activityId, personaId});
        const createdAt = requiredText(input.createdAt ?? input.created_at, 'Activity reaction.createdAt');
        const replace = () => {
            openDatabase.prepare(`
                DELETE FROM companion_activity_reactions
                WHERE activity_id = ? AND actor_kind = 'user'
            `).run(activityId);
            if (!liked) return null;
            openDatabase.prepare(`
                INSERT INTO companion_activity_reactions (
                    activity_id, actor_kind, supporting_character_id, created_at
                ) VALUES (?, 'user', NULL, ?)
            `).run(activityId, createdAt);
            return openDatabase.prepare(`
                SELECT * FROM companion_activity_reactions
                WHERE activity_id = ? AND actor_kind = 'user'
            `).get(activityId);
        };
        return openDatabase.inTransaction ? replace() : openDatabase.transaction(replace)();
    }

    function setActivityVisibility(first, second = {}) {
        const input = inputFor(first, second);
        const activityId = requiredText(input.activityId ?? input.id, 'Activity.id');
        requireActivity({activityId, personaId: input.personaId});
        if (typeof input.hidden !== 'boolean') throw new TypeError('Activity visibility.hidden must be a boolean');
        const updatedAt = requiredText(input.updatedAt ?? input.updated_at, 'Activity visibility.updatedAt');
        const hiddenAt = input.hidden
            ? requiredText(input.hiddenAt ?? input.hidden_at ?? updatedAt, 'Activity visibility.hiddenAt')
            : null;
        openDatabase.prepare(`
            INSERT INTO companion_activity_visibility (activity_id, hidden_at, updated_at)
            VALUES (?, ?, ?)
            ON CONFLICT(activity_id) DO UPDATE SET
                hidden_at = excluded.hidden_at,
                updated_at = excluded.updated_at
        `).run(activityId, hiddenAt, updatedAt);
        return openDatabase.prepare('SELECT * FROM companion_activity_visibility WHERE activity_id = ?').get(activityId);
    }

    function hideActivity(first, second = {}) {
        const input = inputFor(first, second);
        return setActivityVisibility({...input, hidden: true});
    }

    function restoreActivity(first, second = {}) {
        const input = inputFor(first, second);
        return setActivityVisibility({...input, hidden: false});
    }

    return Object.freeze({
        findActivity,
        listActivities,
        insertActivity,
        listActivityComments,
        insertActivityComment,
        insertComment: insertActivityComment,
        listActivityMedia,
        insertActivityMedia,
        setUserReaction: reactionActivity,
        setActivityVisibility,
        setVisibility: setActivityVisibility,
        hideActivity,
        restoreActivity
    });
}
