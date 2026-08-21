import {randomUUID} from 'node:crypto';

export const ACTIVITY_SERVICE_VERSION = 1;
export const ACTIVITY_DEFAULT_PAGE_SIZE = 20;
export const ACTIVITY_MAX_PAGE_SIZE = 50;
export const ACTIVITY_COMMENT_MAX_LENGTH = 500;

const ACTIVITY_COMMENT_MEMORY_KEY = '动态互动';
const ACTIVITY_COMMENT_MEMORY_SOURCE = 'activity_comment';

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function requiredText(value, field, maxLength = 240) {
    if (typeof value !== 'string' || value.trim() === '') {
        throw new TypeError(`${field} must be a non-empty string`);
    }
    const text = value.trim();
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function boundedText(value, field, maxLength, {allowEmpty = false} = {}) {
    if (typeof value !== 'string') throw new TypeError(`${field} must be a string`);
    const text = value.trim();
    if (!allowEmpty && text === '') throw new TypeError(`${field} must not be empty`);
    if (text.length > maxLength) throw new RangeError(`${field} exceeds ${maxLength} characters`);
    return text;
}

function valueFor(row, camelName, snakeName, fallback) {
    if (row?.[camelName] !== undefined) return row[camelName];
    if (row?.[snakeName] !== undefined) return row[snakeName];
    return fallback;
}

function rowId(row) {
    return valueFor(row, 'id', 'id');
}

function rowPersonaId(row) {
    return valueFor(row, 'personaId', 'persona_id')
        ?? valueFor(row, 'ownerPersonaId', 'owner_persona_id');
}

function rowCreatedAt(row) {
    return valueFor(row, 'createdAt', 'created_at');
}

function timestamp(value, field) {
    const normalized = value instanceof Date ? value.toISOString() : value;
    if (typeof normalized !== 'string' || !normalized.trim() || !Number.isFinite(Date.parse(normalized))) {
        throw new TypeError(`${field} must be a valid timestamp`);
    }
    return normalized;
}

function clockFor(clock) {
    if (clock === undefined) return () => new Date().toISOString();
    if (typeof clock === 'function') return () => timestamp(clock(), 'Activity service clock value');
    if (isRecord(clock) && typeof clock.now === 'function') {
        return () => timestamp(clock.now(), 'Activity service clock value');
    }
    throw new TypeError('Activity service clock must be a function or provide now()');
}

function idFor(idGenerator) {
    if (idGenerator === undefined) return prefix => `${prefix}_${randomUUID()}`;
    if (typeof idGenerator === 'function') {
        return prefix => requiredText(idGenerator(prefix), 'Activity service generated id', 240);
    }
    if (isRecord(idGenerator) && typeof idGenerator.next === 'function') {
        return prefix => requiredText(idGenerator.next(prefix), 'Activity service generated id', 240);
    }
    throw new TypeError('Activity service idGenerator must be a function or provide next()');
}

function sourceFor(options, names) {
    const repositories = isRecord(options.repositories) ? options.repositories : {};
    for (const source of [options, repositories]) {
        for (const name of names) {
            if (source[name] !== undefined) return source[name];
        }
    }
    return undefined;
}

function methodFor(source, names, field, {optional = false} = {}) {
    if (typeof source === 'function') return source;
    if (isRecord(source)) {
        for (const name of names) {
            if (typeof source[name] === 'function') return source[name].bind(source);
        }
    }
    if (optional) return null;
    throw new TypeError(`Activity service requires ${field}.${names[0]}()`);
}

function sync(value, field) {
    if (value && typeof value.then === 'function') throw new TypeError(`Activity service ${field}() must be synchronous`);
    return value;
}

function notFound(message = '动态不存在') {
    return Object.assign(new Error(message), {status: 404});
}

function invalidCursor() {
    return Object.assign(new Error('动态游标无效'), {status: 400});
}

function normalizeCursor(value) {
    if (value === undefined || value === null || value === '') return null;
    if (isRecord(value)) {
        const createdAt = value.createdAt ?? value.created_at;
        const id = value.id;
        if (typeof createdAt === 'string' && createdAt && typeof id === 'string' && id) {
            return {createdAt, id};
        }
        throw invalidCursor();
    }
    try {
        const parsed = JSON.parse(Buffer.from(String(value), 'base64url').toString('utf8'));
        if (!isRecord(parsed) || typeof parsed.createdAt !== 'string' || !parsed.createdAt
            || typeof parsed.id !== 'string' || !parsed.id) throw invalidCursor();
        return {createdAt: parsed.createdAt, id: parsed.id};
    } catch (error) {
        if (error?.status === 400) throw error;
        throw invalidCursor();
    }
}

function encodeCursor(row) {
    const createdAt = rowCreatedAt(row);
    const id = rowId(row);
    if (typeof createdAt !== 'string' || !createdAt || typeof id !== 'string' || !id) throw invalidCursor();
    return Buffer.from(JSON.stringify({createdAt, id})).toString('base64url');
}

function pageSize(value) {
    if (value === undefined || value === null || value === '') return ACTIVITY_DEFAULT_PAGE_SIZE;
    const number = Number(value);
    if (!Number.isFinite(number)) return ACTIVITY_DEFAULT_PAGE_SIZE;
    return Math.min(ACTIVITY_MAX_PAGE_SIZE, Math.max(1, Math.trunc(number)));
}

function visibilityFor(value) {
    if (value === undefined || value === null || value === '') return 'visible';
    if (value !== 'visible' && value !== 'hidden') throw new TypeError('Activity visibility must be visible or hidden');
    return value;
}

function hiddenFor(row) {
    if (row?.hidden !== undefined) return row.hidden === true || row.hidden === 1 || row.hidden === '1';
    const hiddenAt = valueFor(row, 'hiddenAt', 'hidden_at');
    if (hiddenAt !== undefined) return Boolean(hiddenAt);
    const visibility = row?.visibility;
    if (isRecord(visibility)) return Boolean(visibility.hiddenAt ?? visibility.hidden_at);
    return undefined;
}

function screenedAtFor(row) {
    return valueFor(row, 'screenedAt', 'screened_at');
}

function screenedFor(row, owner, visibility) {
    if (visibility !== 'visible') return false;
    const screenedAt = screenedAtFor(owner) ?? screenedAtFor(row);
    const createdAt = rowCreatedAt(row);
    return Boolean(screenedAt && createdAt && createdAt >= screenedAt);
}

function booleanValue(value) {
    return value === true || value === 1 || value === '1' || value === 'true';
}

function personaDto(persona) {
    if (!persona) return null;
    const groupId = valueFor(persona, 'groupId', 'group_id', null);
    return {
        id: persona.id,
        name: persona.name,
        role: persona.role,
        color: persona.color,
        groupId: groupId ?? null,
        groupName: valueFor(persona, 'groupName', 'group_name', null),
        screened: booleanValue(valueFor(persona, 'screened', 'screened_at', false)),
        currentSituation: valueFor(persona, 'currentSituation', 'current_situation', valueFor(persona, 'situation', 'situation', '')) || '',
        mood: typeof persona.mood === 'string' ? persona.mood : '',
        unreadCount: Number(valueFor(persona, 'unreadCount', 'unread_count', 0)) || 0,
        updatedAt: valueFor(persona, 'updatedAt', 'updated_at', undefined)
    };
}

function commentDto(row, owner) {
    const authorKind = valueFor(row, 'authorKind', 'author_kind', '');
    const authorName = valueFor(row, 'authorName', 'author_name', undefined)
        ?? valueFor(row, 'supportingCharacterName', 'supporting_character_name', undefined)
        ?? (authorKind === 'user' ? '我' : owner?.name || '');
    return {
        id: rowId(row),
        authorKind,
        authorName,
        content: valueFor(row, 'content', 'content', ''),
        createdAt: rowCreatedAt(row)
    };
}

function mediaDto(asset, link) {
    const id = valueFor(asset, 'id', 'id') ?? valueFor(link, 'mediaId', 'media_id');
    if (typeof id !== 'string' || !id) return null;
    const kind = valueFor(asset, 'kind', 'media_kind', undefined)
        ?? valueFor(link, 'kind', 'media_kind', undefined);
    const url = valueFor(asset, 'url', 'url', undefined)
        ?? valueFor(link, 'url', 'url', `/api/companion/media/${id}`);
    return {id, ...(kind === undefined ? {} : {kind}), url};
}

function commandPersonaId(command) {
    const value = command?.personaId ?? command?.persona_id;
    return value === undefined || value === null || value === '' ? undefined : requiredText(value, 'Activity personaId');
}

function commandActivityId(command) {
    return requiredText(command?.activityId ?? command?.activity_id ?? command?.id, 'Activity id');
}

function runTransaction(transaction, work) {
    if (transaction === undefined) return work();
    const runner = typeof transaction === 'function'
        ? transaction
        : methodFor(transaction, ['run', 'transaction', 'execute'], 'transaction');
    const result = runner(work);
    return sync(result, 'transaction');
}

function transactionFor(options) {
    return options.transaction ?? options.transactionPort;
}

function mediaLookup(mediaPort, assetId) {
    const lookup = methodFor(mediaPort, ['findAsset', 'getAsset', 'find', 'get'], 'media port', {optional: true});
    if (!lookup) return null;
    return sync(lookup(assetId), 'media lookup');
}

function ownerFromRow(row) {
    return row?.persona ?? row?.owner ?? row?.personaSummary ?? null;
}

/**
 * Application service for the activity feed and its interactions.
 *
 * Repositories return storage rows only. This service owns cursor encoding,
 * visibility/screening policy, memory evidence, settings read markers, and
 * browser DTO shaping. It deliberately has no transport or storage imports.
 */
export function createActivityService(options = {}) {
    if (!isRecord(options)) throw new TypeError('Activity service options must be an object');
    const activityPort = sourceFor(options, ['activity', 'activityRepository', 'activityPort']);
    if (!activityPort) throw new TypeError('Activity service requires an activity port');
    const listRaw = methodFor(activityPort, ['listActivities', 'list', 'feed'], 'activity port');
    const findRaw = methodFor(activityPort, ['findActivity', 'find', 'get'], 'activity port', {optional: true});
    const listCommentsRaw = methodFor(activityPort, ['listActivityComments', 'listComments', 'comments'], 'activity port', {optional: true});
    const insertCommentRaw = methodFor(activityPort, ['insertActivityComment', 'insertComment', 'addComment'], 'activity port', {optional: true});
    const listMediaRaw = methodFor(activityPort, ['listActivityMedia', 'listMedia', 'media'], 'activity port', {optional: true});
    const reactionRaw = methodFor(activityPort, ['setUserReaction', 'setReaction', 'react'], 'activity port', {optional: true});
    const likedRaw = methodFor(activityPort, ['getUserReaction', 'findUserReaction', 'userReaction', 'hasUserReaction', 'isLiked'], 'activity port', {optional: true});
    const visibilityRaw = methodFor(activityPort, ['setActivityVisibility', 'setVisibility'], 'activity port', {optional: true});
    const hideRaw = methodFor(activityPort, ['hideActivity', 'hide'], 'activity port', {optional: true});
    const restoreRaw = methodFor(activityPort, ['restoreActivity', 'restore'], 'activity port', {optional: true});

    const memoryPort = sourceFor(options, ['memory', 'memoryRepository', 'memoryPort']);
    const memoryInsert = methodFor(memoryPort, ['insertMemory', 'insert', 'create', 'recordEvidence', 'add'], 'memory port', {optional: true});
    const settingsPort = sourceFor(options, ['settings', 'settingsRepository', 'settingsPort']);
    const settingsRead = methodFor(settingsPort, ['read', 'get'], 'settings port', {optional: true});
    const settingsWrite = methodFor(settingsPort, ['write', 'update', 'save'], 'settings port', {optional: true});
    const mediaPort = sourceFor(options, ['media', 'mediaRepository', 'mediaPort']);
    const personaPort = sourceFor(options, ['persona', 'personaRepository', 'personaPort']);
    const personaFind = methodFor(personaPort, ['findActive', 'find', 'get'], 'persona port', {optional: true});
    const personaSummary = methodFor(personaPort, ['summary', 'toDto', 'shape'], 'persona port', {optional: true});
    const now = clockFor(options.clock ?? options.now);
    const generateId = idFor(options.idGenerator ?? options.id);
    const transaction = transactionFor(options);

    function findActivity(activityId, personaId) {
        if (!findRaw) throw new TypeError('Activity service activity port must provide findActivity()');
        const row = sync(findRaw({activityId, ...(personaId ? {personaId} : {})}), 'activity lookup');
        if (!row) throw notFound();
        return row;
    }

    function findPersona(personaId) {
        if (!personaId || !personaFind) return null;
        const row = sync(personaFind(personaId), 'persona lookup');
        if (!row) throw Object.assign(new Error('人格不存在'), {status: 404});
        return row;
    }

    function ownerFor(row, requestedPersonaId, cache) {
        const personaId = requestedPersonaId ?? rowPersonaId(row);
        if (cache.has(personaId)) return cache.get(personaId);
        let owner = ownerFromRow(row);
        if (!owner && personaId && personaFind) owner = findPersona(personaId);
        if (owner && personaSummary) owner = sync(personaSummary(owner), 'persona summary');
        cache.set(personaId, owner);
        return owner;
    }

    function list(command = {}) {
        if (!isRecord(command)) throw new TypeError('Activity list command must be an object');
        const personaId = commandPersonaId(command);
        const cursor = normalizeCursor(command.cursor ?? command.nextCursor);
        const visibility = visibilityFor(command.visibility);
        const limit = pageSize(command.limit);
        if (personaId) findPersona(personaId);

        const rows = [];
        const owners = new Map();
        let repositoryCursor = cursor;
        let exhausted = false;
        while (rows.length <= limit && !exhausted) {
            const batchLimit = Math.min(ACTIVITY_MAX_PAGE_SIZE, limit + 1);
            const batch = sync(listRaw({
                ...(personaId ? {personaId} : {}),
                cursor: repositoryCursor,
                limit: batchLimit,
                visibility
            }), 'activity list');
            if (!Array.isArray(batch)) throw new TypeError('Activity list port must return an array');
            if (batch.length === 0) {
                exhausted = true;
                break;
            }
            for (const row of batch) {
                if (!isRecord(row)) throw new TypeError('Activity list port must return object rows');
                const owner = ownerFor(row, personaId, owners);
                const hidden = hiddenFor(row);
                const visibilityMatch = hidden === undefined || (visibility === 'hidden' ? hidden : !hidden);
                if (visibilityMatch && !screenedFor(row, owner, visibility)) rows.push(row);
                if (rows.length > limit) break;
            }
            const last = batch.at(-1);
            const nextRepositoryCursor = {createdAt: rowCreatedAt(last), id: rowId(last)};
            if (!nextRepositoryCursor.createdAt || !nextRepositoryCursor.id) throw invalidCursor();
            if (repositoryCursor && repositoryCursor.createdAt === nextRepositoryCursor.createdAt
                && repositoryCursor.id === nextRepositoryCursor.id) {
                throw new Error('Activity list port did not advance its cursor');
            }
            repositoryCursor = nextRepositoryCursor;
            if (rows.length > limit || batch.length < batchLimit) exhausted = true;
        }

        const items = rows.slice(0, limit).map(row => activityDto(row, ownerFor(row, personaId, owners)));
        return {
            items,
            nextCursor: rows.length > limit ? encodeCursor(items.at(-1)) : null
        };
    }

    function activityDto(row, owner) {
        const personaId = rowPersonaId(row);
        const comments = listCommentsRaw
            ? sync(listCommentsRaw({activityId: rowId(row), ...(personaId ? {personaId} : {}), limit: 8}), 'comment list')
            : [];
        if (!Array.isArray(comments)) throw new TypeError('Activity comment port must return an array');
        const links = listMediaRaw
            ? sync(listMediaRaw({activityId: rowId(row), ...(personaId ? {personaId} : {})}), 'media link list')
            : [];
        if (!Array.isArray(links)) throw new TypeError('Activity media port must return an array');

        let liked = valueFor(row, 'liked', 'liked', undefined);
        if (likedRaw) {
            const reaction = sync(likedRaw({activityId: rowId(row), ...(personaId ? {personaId} : {})}), 'reaction lookup');
            liked = typeof reaction === 'boolean' ? reaction : Boolean(reaction);
        }
        const media = links.map(link => {
            const assetId = valueFor(link, 'mediaId', 'media_id');
            const asset = mediaLookup(mediaPort, assetId) ?? link.asset ?? link;
            return mediaDto(asset, link);
        }).filter(Boolean);
        return {
            id: rowId(row),
            persona: personaDto(owner ?? ownerFromRow(row)),
            content: valueFor(row, 'content', 'content', ''),
            mediaMode: valueFor(row, 'mediaMode', 'media_mode', 'none'),
            mediaStatus: valueFor(row, 'mediaStatus', 'media_status', 'none'),
            createdAt: rowCreatedAt(row),
            comments: comments.slice(0, 8).map(comment => commentDto(comment, owner)),
            liked: Boolean(liked),
            media
        };
    }

    function comment(command = {}) {
        if (!isRecord(command)) throw new TypeError('Activity comment command must be an object');
        const activityId = commandActivityId(command);
        const personaId = commandPersonaId(command);
        const activity = findActivity(activityId, personaId);
        const ownerId = rowPersonaId(activity);
        const content = boundedText(command.content, 'Activity comment content', ACTIVITY_COMMENT_MAX_LENGTH);
        if (!insertCommentRaw) throw new TypeError('Activity service activity port must provide insertActivityComment()');
        const createdAt = now();
        const commentId = generateId('comment');
        const rawInput = {
            id: commentId,
            activityId: rowId(activity),
            personaId: ownerId,
            parentCommentId: null,
            authorKind: 'user',
            authorPersonaId: null,
            supportingCharacterId: null,
            content,
            createdAt
        };
        const stored = runTransaction(transaction, () => {
            const row = sync(insertCommentRaw(rawInput), 'comment insert') ?? rawInput;
            if (memoryInsert) {
                sync(memoryInsert({
                    id: generateId('memory'),
                    personaId: ownerId,
                    memoryKey: ACTIVITY_COMMENT_MEMORY_KEY,
                    value: content,
                    confidence: 0.7,
                    status: 'active',
                    sourceType: ACTIVITY_COMMENT_MEMORY_SOURCE,
                    sourceId: commentId,
                    createdAt,
                    updatedAt: createdAt
                }), 'memory evidence insert');
            }
            return row;
        });
        return commentDto({...rawInput, ...(isRecord(stored) ? stored : {})}, ownerFromRow(activity));
    }

    function like(command = {}) {
        if (!isRecord(command)) throw new TypeError('Activity reaction command must be an object');
        const activityId = commandActivityId(command);
        const personaId = commandPersonaId(command);
        const activity = findActivity(activityId, personaId);
        if (typeof command.liked !== 'boolean') throw new TypeError('Activity reaction liked must be a boolean');
        if (!reactionRaw) throw new TypeError('Activity service activity port must provide setUserReaction()');
        sync(reactionRaw({
            activityId: rowId(activity),
            personaId: rowPersonaId(activity),
            liked: command.liked,
            createdAt: now()
        }), 'reaction write');
        return {liked: command.liked};
    }

    function hide(command = {}) {
        if (!isRecord(command)) throw new TypeError('Activity visibility command must be an object');
        const activityId = commandActivityId(command);
        const personaId = commandPersonaId(command);
        const activity = findActivity(activityId, personaId);
        if (typeof command.hidden !== 'boolean') throw new TypeError('Activity visibility hidden must be a boolean');
        const updatedAt = now();
        const input = {
            activityId: rowId(activity),
            personaId: rowPersonaId(activity),
            hidden: command.hidden,
            hiddenAt: command.hidden ? updatedAt : null,
            updatedAt
        };
        if (visibilityRaw) sync(visibilityRaw(input), 'visibility write');
        else if (command.hidden && hideRaw) sync(hideRaw(input), 'visibility write');
        else if (!command.hidden && restoreRaw) sync(restoreRaw(input), 'visibility write');
        else throw new TypeError('Activity service activity port must provide visibility write methods');
        return {hidden: command.hidden};
    }

    function markRead() {
        if (!settingsWrite) throw new TypeError('Activity service requires settings write port');
        const current = settingsRead ? sync(settingsRead(), 'settings read') : {};
        const next = {...(isRecord(current) ? current : {}), activityReadAt: now()};
        sync(settingsWrite(next), 'settings write');
        return undefined;
    }

    const activities = Object.freeze({
        list,
        comment,
        like,
        hide,
        markRead,
        read: markRead,
        addComment: comment,
        setReaction: like,
        setVisibility: hide
    });
    return Object.freeze({
        version: ACTIVITY_SERVICE_VERSION,
        activities,
        list,
        listActivities: list,
        comment,
        commentActivity: comment,
        like,
        likeActivity: like,
        hide,
        hideActivity: hide,
        markRead,
        markActivitiesRead: markRead
    });
}

export const createCompanionActivityService = createActivityService;
export default createActivityService;
