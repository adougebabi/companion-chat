// Invoked only by TestModelManagementReactHTTPPersistenceAndRuntime. The
// transport adds synthetic session/CSRF headers for the isolated Go test server.
import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
import { proxy, snapshot, subscribe } from 'valtio/vanilla';
import { act, installDOM, mountReact } from './helpers/react-dom.mjs';
const base = process.env.FLUCTLIGHT_M4_HTTP_BASE, phase = process.argv[2];
const endpoint = new URL(base);
assert.equal(endpoint.hostname, '127.0.0.1');
assert.ok(['set', 'clear', 'plugins-off', 'moment-only'].includes(phase));
const environment = installDOM();
const vite = await createServer({ root: fileURLToPath(new URL('../', import.meta.url)), appType: 'custom', logLevel: 'silent', server: { middlewareMode: true } });
let view;
const notificationTimers = [];
try {
    const { BrowserClient } = await vite.ssrLoadModule('/@fs/' + fileURLToPath(new URL('../../../packages/browser-client/src/index.ts', import.meta.url)));
    const { installConfigurationProvider } = await vite.ssrLoadModule('/src/v3/providers/configuration.ts');
    const { default: Management } = await vite.ssrLoadModule('/src/react-app/pages/ManagementPage.jsx');
    const client = new BrowserClient(base, async (url, options = {}) => {
        const headers = new Headers(options.headers);
        headers.set('cookie', 'fluctlight_session=session; fluctlight_csrf=csrf');
        headers.set('origin', 'http://fluctlight.local');
        headers.set('x-csrf-token', 'csrf');
        return fetch(url, { ...options, headers });
    });
    const state = proxy({ session: { actorId: 'human-1' }, managementTab: '配置中心', managementSection: phase === 'plugins-off' || phase === 'moment-only' ? '能力插件' : '模型用途', characters: [], operations: [], runtimeCatalog: {} });
    // The production facade installs the notification polling timer. Track that
    // process-local test resource so this standalone scenario can release it.
    const originalInterval = globalThis.setInterval;
    let configuration;
    try {
        globalThis.setInterval = (...args) => { const timer = originalInterval(...args); notificationTimers.push(timer); return timer; };
        configuration = installConfigurationProvider(client, async () => { });
    }
    finally {
        globalThis.setInterval = originalInterval;
    }
    const w = { state, subscribe: fn => subscribe(state, fn), getSnapshot: () => snapshot(state), services: { configurationClient: configuration }, notify() { }, navigatePath() { } };
    const button = (text, root = view.host) => [...root.querySelectorAll('button')].find(node => node.textContent.trim() === text);
    const wait = async (predicate) => {
        for (let attempt = 0; attempt < 100; attempt++) {
            let ready;
            await act(async () => { ready = await predicate(); });
            if (ready)
                return;
            await view.flush();
        }
        throw Error('Timed out waiting for formal management page: ' + view.host.textContent);
    };
    const openVariant = async () => {
        await wait(() => button('按 Agent 或任务变体覆盖'));
        await view.click(button('按 Agent 或任务变体覆盖'));
        await view.click(view.host.querySelector('[aria-label="选择 Agent 模型任务"]'));
        await wait(() => [...document.querySelectorAll('[role=option]')].some(node => node.textContent.trim()==='记忆与自我反思'));
        const option = [...document.querySelectorAll('[role=option]')].find(node => node.textContent.trim()==='记忆与自我反思');
        assert.ok(option, 'registered reflection variant is missing');
        await view.click(option);
    };
    const row = () => view.host.querySelector('section[aria-label="记忆与自我反思"]');
    const source = () => row()?.querySelector('[data-testid=agent-model-source]')?.textContent || '';
    if (phase === 'plugins-off' || phase === 'moment-only') {
        view = await mountReact(vite, Management, { workspace: w });
        const image = () => view.host.querySelector('[aria-label="启用图片能力"]');
        const moment = () => view.host.querySelector('[aria-label="启用动态能力"]');
        const targetMoment = phase === 'moment-only';
        await wait(() => image() && moment());
        if (image().getAttribute('aria-checked') !== 'false')
            await view.click(image());
        if (moment().getAttribute('aria-checked') !== String(targetMoment))
            await view.click(moment());
        await act(async () => view.host.querySelector('form').dispatchEvent(new window.Event('submit', { bubbles: true, cancelable: true })));
        let saved;
        await wait(async () => {
            saved = await client.configurationCatalog();
            const values = saved.effective_values['plugins.domains'];
            return values.image === false && values.moment === targetMoment;
        });
        await view.dispose();
        view = await mountReact(vite, Management, { workspace: w });
        await wait(() => image()?.getAttribute('aria-checked') === 'false' && moment()?.getAttribute('aria-checked') === String(targetMoment));
        console.log('M4_UI_PLUGINS', phase, 'RELOADED', saved.effective_values['plugins.domains'], 'VERSION', saved.versions['plugins.domains']);
    }
    else {
        view = await mountReact(vite, Management, { workspace: w });
        await openVariant();
        if (phase === 'set') {
            assert.match(source(), /默认用途.*m4-primary/);
            await view.click(row().querySelector('[aria-label="模型服务"]'));
            const provider = [...document.querySelectorAll('[role=option]')].find(node => node.textContent.trim() === 'm4-local');
            assert.ok(provider);
            await view.click(provider);
            await view.input(row().querySelector('input[aria-label="模型标识"]'), 'm4-override');
        }
        else {
            assert.match(source(), /任务变体覆盖.*m4-override/);
            await view.click(button('恢复继承', row()));
        }
        await act(async () => view.host.querySelector('form').dispatchEvent(new window.Event('submit', { bubbles: true, cancelable: true })));
        const expected = phase === 'set' ? /任务变体覆盖.*m4-override/ : /默认用途.*m4-primary/;
        await wait(() => expected.test(source()));
        await view.dispose();
        // Re-mount the actual page and re-fetch the persisted configuration.
        view = await mountReact(vite, Management, { workspace: w });
        await openVariant();
        await wait(() => expected.test(source()));
        console.log('M4_UI_PHASE', phase, 'RELOADED_EFFECTIVE_SOURCE', source());
    }
}
finally {
    if (view)
        await view.dispose();
    for (const timer of notificationTimers)
        clearInterval(timer);
    await vite.close();
    environment.restore();
}
