import assert from 'node:assert/strict';
import test from 'node:test';

import {createMediaObservabilityFixture} from './fixtures/media-observability-composition-fixture.mjs';
import {cleanup as cleanupLegacy, state as legacyState} from './fixtures/legacy-media-observability-domain-fixture.mjs';

test('media composition preserves bounded progress, final prompt, and lease ownership', async () => {
    const fixture = createMediaObservabilityFixture();
    const job = fixture.mediaJob('media_progress_composed');
    const rawOutput = '\u001b[32msampling 12.5% /private/models/MiniMax-H3 Bearer test-progress-secret\r';
    const parsed = fixture.observability.parseProgress(rawOutput);
    assert.equal(parsed.percent, 12.5);
    assert.equal(parsed.output.includes('\u001b'), false);
    assert.equal(parsed.output.includes('/private/models'), false);
    assert.equal(parsed.output.includes('test-progress-secret'), false);

    const written = fixture.observability.recordProgress(job, {
        stage: 'generating',
        output: rawOutput,
        latestStream: 'stderr',
        outputLineCountDelta: 1
    });
    assert.equal(written.changed, true);
    assert.equal(written.progress.stage, 'generating');
    assert.equal(written.progress.percent, 12.5);
    assert.equal(written.progress.outputLineCount, 1);

    const stale = fixture.observability.recordProgress({...job, lease_owner: 'stale_media_worker'}, {stage: 'generating', output: '99%'});
    assert.deepEqual(stale, {changed: false, reason: 'lease_rejected'});

    const completed = await fixture.dispatcher.runJob(job, {
        leaseOwner: job.lease_owner,
        now: fixture.NOW,
        signal: new AbortController().signal
    });
    assert.equal(completed.status, 'complete');
    const stored = JSON.parse(fixture.jobs.get(job.id).result_json);
    assert.equal(stored.finalPrompt, '最终送往 h3 的安全提示词');
    assert.equal(stored.progress.stage, 'complete');
    assert.equal(stored.progress.percent, 100);
    assert.equal(stored.progress.latestOutput.includes('/private/models'), false);
    assert.equal(stored.progress.latestOutput.includes('test-progress-secret'), false);
});

test('media composition captures provider stdout/stderr and throttles reporter writes', async () => {
    const fixture = createMediaObservabilityFixture();
    const observed = [];
    await fixture.observability.runProcess(process.execPath, ['-e', "process.stdout.write('12% sampling\\r'); process.stderr.write('50% validating\\n');"], 5_000, {
        onOutput: (stream, output) => observed.push({stream, output})
    });
    assert.equal(observed.some(item => item.stream === 'stdout' && item.output.includes('12%')), true);
    assert.equal(observed.some(item => item.stream === 'stderr' && item.output.includes('50%')), true);

    const job = fixture.mediaJob('media_progress_throttle');
    const reporter = fixture.observability.createReporter(job);
    assert.equal(reporter.output('stdout', '1% preparing').changed, true);
    assert.equal(reporter.output('stderr', '2% sampling').throttled, true);
    assert.equal(reporter.output('stderr', 'sampling without a reported percentage').throttled, true);
    assert.equal(reporter.flush().changed, true);
    const progress = JSON.parse(fixture.jobs.get(job.id).result_json).progress;
    assert.equal(progress.percent, 2);
    assert.equal(progress.latestOutput, 'sampling without a reported percentage');
    assert.equal(progress.outputLineCount, 3);
});

test('media composition keeps a failed attempt snapshot and resets progress on retry', () => {
    const fixture = createMediaObservabilityFixture();
    const job = fixture.mediaJob('media_progress_retry', {attempt: 1});
    fixture.observability.recordProgress(job, {
        stage: 'generating',
        output: '88% sampling',
        latestStream: 'stderr',
        outputLineCountDelta: 1
    });
    const failed = fixture.observability.settle(job, {
        status: 'retry',
        error: 'h3 process exited 1',
        progressStage: 'failed'
    });
    assert.equal(failed.changed, true);
    assert.equal(failed.status, 'queued');
    const stored = JSON.parse(fixture.jobs.get(job.id).result_json);
    assert.equal(stored.progress.stage, 'failed');
    assert.equal(stored.progress.percent, 88);

    fixture.lease(job, 'media_fixture_worker', 2);
    const restarted = fixture.observability.recordProgress(job, {stage: 'preparing'});
    assert.equal(restarted.changed, true);
    assert.equal(restarted.progress.attempt, 2);
    assert.equal(restarted.progress.stage, 'preparing');
    assert.equal(restarted.progress.percent, null);
    assert.equal(restarted.progress.outputLineCount, 0);
});

test('legacy media debug aggregation remains an explicit domain comparison fixture', () => {
    const {
        database,
        createPersona,
        createChatMediaRequest,
        debugContextFor,
        saveSettings,
        publicSettings
    } = legacyState();
    saveSettings({simplifiedMediaMode: true});
    assert.equal(publicSettings().simplifiedMediaMode, true);

    const persona = createPersona({name: '聚合检查', role: '视频创作者', foundation: '聚合检查会通过本地检查器观察媒体任务。'});
    const request = createChatMediaRequest(persona.id, {
        schemaVersion: 2,
        kind: 'video',
        request: '城市夜景延时视频',
        count: 1,
        personaMediaConcept: {
            schemaVersion: 1,
            mediaKind: 'video',
            scene: '测试场景',
            action: '测试动作',
            mood: '平静',
            narrative: '测试媒体概念',
            humanSubjects: [{label: '人格本人', role: '主体', inFrame: true}],
            nonHumanObjects: [{label: '环境', kind: 'environment', inFrame: true}],
            capture: {mode: 'external_capture', operator: '画外朋友', deviceVisibility: 'out_of_frame', framingIntent: '自然中景'},
            compositionIntent: '保持概念中的主体与环境关系。'
        },
        currentEvent: null,
        temporaryAppearance: {}
    });
    const source = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const createdAt = new Date().toISOString();
    database.prepare("UPDATE companion_jobs SET status = 'complete', result_json = ?, updated_at = ?, completed_at = ? WHERE id = ?").run(JSON.stringify({provider: 'comfyui', finalPrompt: '唯一应显示的最终 provider 提示词', externalId: 'prompt_source', pending: true}), createdAt, createdAt, source.id);
    database.prepare(`
        INSERT INTO companion_jobs (id, job_type, status, priority, run_after, lease_owner, lease_expires_at, max_attempts, persona_id, message_id, payload_json, result_json, created_at, updated_at)
        VALUES (?, 'chat_media_poll', 'leased', 4, ?, ?, ?, 60, ?, ?, ?, '{}', ?, ?)
    `).run('poll_aggregate_child', createdAt, 'poll_aggregate_lease', new Date(Date.now() + 60_000).toISOString(), persona.id, source.message_id, JSON.stringify({provider: 'comfyui', externalId: 'prompt_source', kind: 'video'}), createdAt, createdAt);

    const matches = debugContextFor(persona.id).mediaJobs.filter(job => job.id === source.id || job.id === 'poll_aggregate_child');
    assert.equal(matches.length, 1);
    assert.equal(matches[0].id, source.id);
    assert.equal(matches[0].status, 'leased');
    assert.equal(matches[0].finalPrompt, '唯一应显示的最终 provider 提示词');
    assert.equal(matches[0].progress.stage, 'waiting_provider');
});

test.after(() => cleanupLegacy());
