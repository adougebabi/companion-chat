const MAX_ID_LENGTH = 240;
const MAX_FOUNDATION_LENGTH = 6_000;
const MAX_REASON_LENGTH = 240;

function isRecord(value) {
    return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function statusError(message, status) {
    return Object.assign(new Error(message), {status});
}

function record(value, field) {
    if (!isRecord(value)) throw statusError(`${field}必须是 JSON 对象`, 400);
    return value;
}

function requiredText(value, field, maxLength = MAX_ID_LENGTH) {
    if (typeof value !== 'string' || value.trim() === '') throw statusError(`${field}不能为空`, 400);
    const normalized = value.trim();
    if (normalized.length > maxLength) throw statusError(`${field}不能超过 ${maxLength} 个字符`, 400);
    return normalized;
}

function sourceFor(options, names) {
    const repositories = isRecord(options.repositories) ? options.repositories : {};
    for (const source of [options, repositories]) {
        for (const name of names) if (source[name] !== undefined) return source[name];
    }
    return undefined;
}

function methodFor(port, names, field, {optional = false} = {}) {
    if (typeof port === 'function') return port;
    if (isRecord(port)) {
        for (const name of names) if (typeof port[name] === 'function') return port[name].bind(port);
    }
    if (optional) return null;
    throw statusError(`基础设定服务缺少 ${field}.${names[0]}()`, 501);
}

function mapMaybe(value, mapper) {
    return value && typeof value.then === 'function' ? value.then(mapper) : mapper(value);
}

function clockFor(value) {
    if (typeof value === 'function') return value;
    if (isRecord(value) && typeof value.now === 'function') return value.now.bind(value);
    return () => new Date().toISOString();
}

function personaIdFor(command) {
    const input = record(command, '人格请求');
    return requiredText(input.personaId ?? input.persona_id, '人格 ID');
}

/**
 * Foundation revisions are application-owned transitions. The foundation
 * repository stores revisions; an explicitly supplied life-model port owns
 * any replan requested by the caller.
 */
export function createFoundationService(options = {}) {
    if (!isRecord(options)) throw new TypeError('Foundation service options must be an object');
    const foundation = sourceFor(options, ['foundation', 'foundationRepository', 'foundationPort']);
    const personas = sourceFor(options, ['persona', 'personas', 'personaRepository', 'personaPort']);
    const lifeModel = sourceFor(options, ['lifeModel', 'lifeBlueprint', 'lifeModelRepository', 'lifeModelPort']);
    const now = clockFor(options.clock ?? options.now);

    const findPersona = methodFor(personas, ['findActive', 'find', 'get'], 'persona port', {optional: true});
    const requirePersona = personaId => {
        if (!findPersona) return {id: personaId};
        const value = findPersona(personaId);
        if (value && typeof value.then === 'function') throw new TypeError('Foundation persona lookup must be synchronous');
        if (!value) throw statusError('人格不存在', 404);
        return value;
    };
    const callFoundation = (names, input, field) => {
        const operation = methodFor(foundation, names, field, {optional: true});
        if (!operation) throw statusError(`基础设定服务缺少 ${field} port`, 501);
        return operation(input);
    };

    function draft(command = {}) {
        const personaId = personaIdFor(command);
        requirePersona(personaId);
        const operation = methodFor(foundation, ['getDraft', 'getCurrent', 'findCurrent', 'draft', 'current'], 'draft', {optional: true});
        if (!operation) throw statusError('基础设定服务缺少 draft port', 501);
        return mapMaybe(operation({personaId}), value => {
            if (!value) throw statusError('基础设定不存在', 404);
            return value;
        });
    }

    function update(command = {}) {
        const input = record(command, '基础设定更新请求');
        const personaId = personaIdFor(input);
        requirePersona(personaId);
        const foundationText = requiredText(input.foundation, '基础设定', MAX_FOUNDATION_LENGTH);
        const reason = input.reason === undefined ? '用户修订基础人格' : requiredText(input.reason, '修订原因', MAX_REASON_LENGTH);
        const result = callFoundation(['updateFoundation', 'update', 'createRevision', 'insertRevision'], {
            ...input, personaId, foundation: foundationText, reason, updatedAt: input.updatedAt ?? now()
        }, 'update');
        const complete = lifeModel && input.replanLife === true;
        if (!complete) return result;

        const replan = methodFor(lifeModel, ['replan', 'replanLife', 'updateFromFoundation', 'saveRevision'], 'life model replan', {optional: true});
        if (!replan) throw statusError('基础设定更新需要 life model replan port', 501);
        return mapMaybe(result, foundationResult => mapMaybe(replan({
            ...input,
            personaId,
            foundation: foundationText,
            reason,
            foundationRevision: foundationResult
        }), lifeResult => ({...(isRecord(foundationResult) ? foundationResult : {value: foundationResult}), lifeModelRevision: lifeResult})));
    }

    function restore(command = {}) {
        const input = record(command, '基础设定恢复请求');
        const personaId = personaIdFor(input);
        const revisionId = requiredText(input.revisionId ?? input.revision_id, '版本 ID');
        requirePersona(personaId);
        return callFoundation(['restoreFoundationRevision', 'restoreRevision', 'restore'], {
            ...input, personaId, revisionId, restoredAt: input.restoredAt ?? now()
        }, 'restore');
    }

    return Object.freeze({
        draft,
        getDraft: draft,
        getFoundationDraft: draft,
        update,
        updateFoundation: update,
        restore,
        restoreRevision: restore,
        restoreFoundationRevision: restore
    });
}

export const createFoundationApplicationService = createFoundationService;
export default createFoundationService;
