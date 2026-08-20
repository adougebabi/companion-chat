import assert from 'node:assert/strict';
import test from 'node:test';
import Database from 'better-sqlite3';

import {BACKEND_CONTRACT_BASELINE} from '../server/contracts/index.js';
import {createCompositionRoot} from '../server/composition-root.js';

function createDependencies(overrides = {}) {
    const events = [];
    const database = overrides.database ?? new Database(':memory:');
    const clock = overrides.clock ?? {
        now() {
            events.push('clock');
            return new Date(0);
        }
    };
    const idGenerator = overrides.idGenerator ?? {
        next(prefix) {
            events.push(`id:${prefix}`);
            return `${prefix}_test`;
        }
    };
    const capabilityDispatcher = overrides.capabilityDispatcher ?? {
        async dispatch(input) {
            events.push(['dispatch', input]);
            return {results: [], effects: []};
        }
    };
    const contextReader = overrides.contextReader ?? {
        async read(input) {
            events.push(['context', input]);
            return {fragments: []};
        }
    };
    const llmStreamingPort = overrides.llmStreamingPort ?? {
        async stream(input) {
            events.push(['llm', input]);
            return {text: 'ready', toolCalls: []};
        }
    };
    const repositories = overrides.repositories ?? {
        conversation: {
            listMessages() {
                events.push('conversation');
                return {items: []};
            }
        },
        activity: {},
        job: {},
        pending: {}
    };
    const commitAdapter = overrides.commitAdapter ?? {
        commits: [],
        commit(result) {
            this.commits.push(result);
        }
    };
    const sseAdapter = overrides.sseAdapter ?? (async () => {
        events.push('sse');
    });
    return {
        options: {
            database,
            clock,
            idGenerator,
            capabilityDispatcher,
            contextReader,
            llmStreamingPort,
            repositories,
            commitAdapter,
            sseAdapter
        },
        database,
        clock,
        idGenerator,
        capabilityDispatcher,
        contextReader,
        llmStreamingPort,
        repositories,
        commitAdapter,
        sseAdapter,
        events
    };
}

test('composition root preserves injected adapter identity and exposes the single flow wiring', () => {
    const dependencies = createDependencies();
    try {
        const composition = createCompositionRoot(dependencies.options);

        assert.equal(composition.contractVersion, BACKEND_CONTRACT_BASELINE.version);
        assert.strictEqual(composition.capabilityDispatcher, dependencies.capabilityDispatcher);
        assert.strictEqual(composition.repositories.conversation, dependencies.repositories.conversation);
        assert.strictEqual(composition.repositories.activity, dependencies.repositories.activity);
        assert.strictEqual(composition.repositories.job, dependencies.repositories.job);
        assert.strictEqual(composition.repositories.pending, dependencies.repositories.pending);
        assert.strictEqual(composition.sqliteCommitAdapter, dependencies.commitAdapter);
        assert.strictEqual(composition.commitAdapter, dependencies.commitAdapter);
        assert.strictEqual(composition.chatSseAdapter, dependencies.sseAdapter);
        assert.strictEqual(composition.sseAdapter, dependencies.sseAdapter);
        assert.strictEqual(composition.flowRegistry, composition.flows);
        assert.strictEqual(composition.flowExecutor, composition.executor);
        assert.equal(composition.flowRegistry.has('chat-turn'), true);
        assert.equal(composition.chatTurnFlow.flowId, 'chat-turn');
    } finally {
        dependencies.database.close();
    }
});

test('composition root can construct the existing SQLite adapters from one open database', () => {
    const database = new Database(':memory:');
    let prepareCalls = 0;
    const originalPrepare = database.prepare.bind(database);
    database.prepare = (...args) => {
        prepareCalls += 1;
        return originalPrepare(...args);
    };
    const clock = {now: () => new Date(0)};
    const idGenerator = {next: prefix => `${prefix}_test`};
    const commitAdapter = {commit() {}};
    const sseAdapter = () => {};
    try {
        const composition = createCompositionRoot({
            database,
            clock,
            idGenerator,
            capabilityDispatcher: {dispatch: async () => ({results: [], effects: []})},
            contextReader: {read: async () => ({})},
            llmStreamingPort: {stream: async () => ({text: '', toolCalls: []})},
            commitAdapter,
            sseAdapter
        });

        assert.equal(prepareCalls, 0);
        assert.equal(typeof composition.repositories.conversation.listMessages, 'function');
        assert.equal(typeof composition.repositories.activity.listActivities, 'function');
        assert.equal(typeof composition.repositories.job.enqueue, 'function');
        assert.equal(typeof composition.repositories.pending.findByDedupeKey, 'function');
        assert.strictEqual(composition.database, database);
        assert.strictEqual(composition.clock, clock);
        assert.strictEqual(composition.idGenerator, idGenerator);
    } finally {
        database.close();
    }
});

test('composition root wires an auto-created SSE adapter to the chat flow without running it at construction time', async () => {
    const dependencies = createDependencies();
    const sent = [];
    let ended = 0;
    try {
        const {sseAdapter: _injectedSseAdapter, ...optionsWithoutSseAdapter} = dependencies.options;
        const composition = createCompositionRoot({
            ...optionsWithoutSseAdapter,
            sendSse: (_sink, event) => sent.push(event),
            end: () => { ended += 1; }
        });
        assert.notStrictEqual(composition.chatSseAdapter, undefined);
        assert.deepEqual(dependencies.events, []);

        const sink = {writableEnded: false, on() { return this; }, removeListener() { return this; }};
        await composition.chatSseAdapter({context: {}, command: {personaId: 'persona_test', text: 'hello'}}, sink);

        assert.deepEqual(sent, [
            {type: 'token', token: 'ready'},
            {type: 'done', messages: [], message: null, learned: [], jobs: []}
        ]);
        assert.equal(ended, 1);
        assert.deepEqual(dependencies.events.map(event => Array.isArray(event) ? event[0] : event), [
            'conversation', 'context', 'llm', 'dispatch'
        ]);
    } finally {
        dependencies.database.close();
    }
});

test('commit adapter receiver binding is retained by the flow executor', async () => {
    const dependencies = createDependencies();
    try {
        const composition = createCompositionRoot(dependencies.options);
        await composition.chatTurnFlow.run({context: {}, command: {personaId: 'persona_test', text: 'hello'}});
        assert.equal(dependencies.commitAdapter.commits.length, 1);
    } finally {
        dependencies.database.close();
    }
});

test('composition root rejects missing core resources before provider or database work', () => {
    const dependencyNames = [
        ['database', /already-open database/],
        ['clock', /clock/],
        ['idGenerator', /id helper/],
        ['capabilityDispatcher', /CapabilityDispatcherPort/],
        ['contextReader', /contextReader/],
        ['llmStreamingPort', /llmStreamingPort/],
        ['commitAdapter', /SQLite commit adapter/],
        ['sseAdapter', /SSE adapter/]
    ];

    for (const [name, errorPattern] of dependencyNames) {
        const dependencies = createDependencies();
        try {
            delete dependencies.options[name];
            assert.throws(() => createCompositionRoot(dependencies.options), errorPattern, name);
        } finally {
            dependencies.database.close();
        }
    }
});

test('composition root rejects a second dispatcher port', () => {
    const dependencies = createDependencies();
    try {
        assert.throws(
            () => createCompositionRoot({...dependencies.options, dispatcher: {dispatch() {}}}),
            /exactly one capabilityDispatcher/
        );
    } finally {
        dependencies.database.close();
    }
});

test('composition root rejects a partially injected repository set', () => {
    const dependencies = createDependencies();
    try {
        const repositories = {...dependencies.options.repositories};
        delete repositories.pending;
        assert.throws(
            () => createCompositionRoot({...dependencies.options, repositories}),
            /pendingEventRepository/
        );
    } finally {
        dependencies.database.close();
    }
});
