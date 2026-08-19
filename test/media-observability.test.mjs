import assert from 'node:assert/strict';
import {mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import test from 'node:test';

const dataDir = mkdtempSync(join(tmpdir(), 'local-ai-companion-observability-test-'));
process.env.DATA_DIR = dataDir;
process.env.COMPANION_TEST = '1';
process.env.COMPANION_DEBUG_INSPECTOR = '1';
const {companionTestHooks, mediaObservabilityTestHooks} = await import(`../server.js?observability=${Date.now()}`);
const {database, createPersona, createChatMediaRequest, completePolledMediaJob, debugContextFor, saveSettings, publicSettings} = companionTestHooks;
const {runH3, parseH3ProgressOutput, recordMediaJobProgress, createMediaProgressReporter, settleJob} = mediaObservabilityTestHooks;

test('h3 progress snapshots preserve the final prompt, redact output, and reject stale leases', () => {
    const persona = createPersona({name: '进度检查', role: '视频创作者', foundation: '进度检查会耐心等待本地视频生成完成。'});
    const request = createChatMediaRequest(persona.id, {kind: 'video', prompt: '在林间行走的红狐'});
    const leaseOwner = 'lease_h3_progress';
    const leaseExpiresAt = new Date(Date.now() + 60_000).toISOString();
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, attempt_count = 1 WHERE id = ?").run(leaseOwner, leaseExpiresAt, request.jobId);
    const job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const parsed = parseH3ProgressOutput('\u001b[32msampling 12.5% /private/models/MiniMax-H3 Bearer test-progress-secret\r');
    assert.equal(parsed.percent, 12.5);
    assert.equal(parsed.output.includes('\u001b'), false);
    assert.equal(parsed.output.includes('/private/models'), false);
    assert.equal(parsed.output.includes('test-progress-secret'), false);

    const written = recordMediaJobProgress(job, {
        stage: 'generating', output: '\u001b[32msampling 12.5% /private/models/MiniMax-H3 Bearer test-progress-secret\r', latestStream: 'stderr', outputLineCountDelta: 1,
        result: {provider: 'h3', finalPrompt: '最终送往 h3 的安全提示词'}
    });
    assert.equal(written.changed, true);
    assert.equal(written.progress.stage, 'generating');
    assert.equal(written.progress.percent, 12.5);
    assert.equal(written.progress.outputLineCount, 1);

    const stale = recordMediaJobProgress({...job, lease_owner: 'stale_h3_progress'}, {stage: 'generating', output: '99%', latestStream: 'stdout'});
    assert.equal(stale.changed, false);

    const completed = completePolledMediaJob(job, 'h3_result_test_progress', [{filename: '/private/output/progress.mp4', type: 'h3', format: 'video', path: '/private/output/progress.mp4'}], 'h3');
    assert.equal(completed.completed, true);
    const row = database.prepare('SELECT result_json FROM companion_jobs WHERE id = ?').get(request.jobId);
    const result = JSON.parse(row.result_json);
    assert.equal(result.finalPrompt, '最终送往 h3 的安全提示词');
    assert.equal(result.progress.stage, 'complete');
    assert.equal(result.progress.percent, 100);
    assert.equal(JSON.stringify(result).includes('/private/output/progress.mp4'), false);

    const debugJob = debugContextFor(persona.id).mediaJobs.find(item => item.id === request.jobId);
    assert.equal(debugJob.finalPrompt, '最终送往 h3 的安全提示词');
    assert.equal(debugJob.progress.stage, 'complete');
    assert.equal(debugJob.progress.percent, 100);
    assert.equal(debugJob.progress.latestOutput.length <= 480, true);
    assert.equal(debugJob.progress.latestOutput.includes('/private/models'), false);
    assert.equal(debugJob.progress.latestOutput.includes('test-progress-secret'), false);
});

test('h3 stream capture reports stdout and stderr while throttled progress keeps the final output', async () => {
    const observed = [];
    await runH3(process.execPath, ['-e', "process.stdout.write('12% sampling\\r'); process.stderr.write('50% validating\\n');"], 5_000, {
        onOutput: (stream, output) => observed.push({stream, output})
    });
    assert.equal(observed.some(item => item.stream === 'stdout' && item.output.includes('12%')), true);
    assert.equal(observed.some(item => item.stream === 'stderr' && item.output.includes('50%')), true);

    const persona = createPersona({name: '节流检查', role: '视频创作者', foundation: '节流检查会记录本地生成过程。'});
    const request = createChatMediaRequest(persona.id, {kind: 'video', prompt: '夜色中的海面'});
    const leaseOwner = 'lease_progress_throttle';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, attempt_count = 1 WHERE id = ?").run(leaseOwner, new Date(Date.now() + 60_000).toISOString(), request.jobId);
    const job = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const reporter = createMediaProgressReporter(job);
    assert.equal(reporter.output('stdout', '1% preparing').changed, true);
    assert.equal(reporter.output('stderr', '2% sampling').throttled, true);
    assert.equal(reporter.output('stderr', 'sampling without a reported percentage').throttled, true);
    assert.equal(reporter.flush().changed, true);
    const progress = JSON.parse(database.prepare('SELECT result_json FROM companion_jobs WHERE id = ?').get(request.jobId).result_json).progress;
    assert.equal(progress.percent, 2);
    assert.equal(progress.latestOutput, 'sampling without a reported percentage');
    assert.equal(progress.outputLineCount, 3);
});

test('a failed h3 attempt keeps its terminal snapshot and a retry starts a fresh attempt', () => {
    const persona = createPersona({name: '重试检查', role: '视频创作者', foundation: '重试检查会在失败后重新尝试本地视频生成。'});
    const request = createChatMediaRequest(persona.id, {kind: 'video', prompt: '雨后的城市街道'});
    const firstLease = 'lease_progress_retry_first';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, attempt_count = 1 WHERE id = ?").run(firstLease, new Date(Date.now() + 60_000).toISOString(), request.jobId);
    const firstJob = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    recordMediaJobProgress(firstJob, {stage: 'generating', output: '88% sampling', latestStream: 'stderr', outputLineCountDelta: 1, result: {provider: 'h3', finalPrompt: 'retry prompt'}});
    const failed = settleJob(firstJob, {error: 'h3 进程退出码 1', progressStage: 'failed'});
    assert.equal(failed.changed, true);
    assert.equal(failed.status, 'queued');
    const stored = JSON.parse(database.prepare('SELECT result_json FROM companion_jobs WHERE id = ?').get(request.jobId).result_json);
    assert.equal(stored.progress.stage, 'failed');
    assert.equal(stored.progress.percent, 88);

    const secondLease = 'lease_progress_retry_second';
    database.prepare("UPDATE companion_jobs SET status = 'leased', lease_owner = ?, lease_expires_at = ?, attempt_count = 2 WHERE id = ?").run(secondLease, new Date(Date.now() + 60_000).toISOString(), request.jobId);
    const secondJob = database.prepare('SELECT * FROM companion_jobs WHERE id = ?').get(request.jobId);
    const restarted = recordMediaJobProgress(secondJob, {stage: 'preparing'});
    assert.equal(restarted.changed, true);
    assert.equal(restarted.progress.attempt, 2);
    assert.equal(restarted.progress.stage, 'preparing');
    assert.equal(restarted.progress.percent, null);
    assert.equal(restarted.progress.outputLineCount, 0);
});

test('debug context merges a poll child into its source media task and persists simplified media mode', () => {
    saveSettings({simplifiedMediaMode: true});
    assert.equal(publicSettings().simplifiedMediaMode, true);

    const persona = createPersona({name: '聚合检查', role: '视频创作者', foundation: '聚合检查会通过本地检查器观察媒体任务。'});
    const request = createChatMediaRequest(persona.id, {kind: 'video', prompt: '城市夜景延时视频'});
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

test.after(() => rmSync(dataDir, {recursive: true, force: true}));
