import {marked} from '/vendor/marked/marked.esm.js';

const $ = selector => document.querySelector(selector);
let state = {settings: {}, personas: [], memories: []};
let activePersonaId = localStorage.getItem('companion-active-persona') || 'fitness';
let messages = [];
let attachments = [];
let isSending = false;
const generationMonitors = new Map();

const esc = value => String(value || '').replace(/[&<>'"]/g, c => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#039;',
    '"': '&quot;'
})[c]);
const markdownRenderer = new marked.Renderer();
markdownRenderer.html = () => '';
const allowedTags = new Set(['P', 'BR', 'STRONG', 'EM', 'DEL', 'CODE', 'PRE', 'BLOCKQUOTE', 'UL', 'OL', 'LI', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'HR', 'A', 'TABLE', 'THEAD', 'TBODY', 'TR', 'TH', 'TD']);

function renderMarkdown(text = '') {
    const parsed = marked.parse(text, {renderer: markdownRenderer, gfm: true, breaks: true});
    const doc = new DOMParser().parseFromString(parsed, 'text/html');
    [...doc.body.querySelectorAll('*')].forEach(element => {
        if (!allowedTags.has(element.tagName)) {
            element.replaceWith(...element.childNodes);
            return;
        }
        const href = element.tagName === 'A' ? element.getAttribute('href') : null;
        [...element.attributes].forEach(attribute => element.removeAttribute(attribute.name));
        if (href) {
            try {
                const url = new URL(href, location.origin);
                if (['http:', 'https:', 'mailto:'].includes(url.protocol)) {
                    element.setAttribute('href', url.href);
                    element.setAttribute('target', '_blank');
                    element.setAttribute('rel', 'noreferrer');
                }
            } catch {
            }
        }
    });
    return doc.body.innerHTML;
}

const activePersona = () => state.personas.find(persona => persona.id === activePersonaId) || state.personas[0];
const api = async (path, options) => {
    const response = await fetch(path, options);
    const payload = response.status === 204 ? null : await response.json();
    if (!response.ok) throw new Error(payload?.error || `HTTP ${response.status}`);
    return payload;
};

function avatar(persona, mini = false) {
    return `<span class="avatar${mini ? ' mini' : ''}" style="--persona:${esc(persona.color)}">${esc(persona.name.slice(0, 1))}</span>`;
}

function attachmentHtml(file) {
    if (file.kind === 'image') return `<img class="media image" src="${esc(file.url)}" alt="${esc(file.name)}">`;
    if (file.kind === 'video') return `<video class="media video" controls src="${esc(file.url)}"></video>`;
    return `<span class="file">▧ ${esc(file.name)}</span>`;
}

function generationHtml(message) {
    const job = message.generation;
    const failed = job.status === 'failed';
    const waiting = job.status === 'submitting';
    const title = failed ? (job.kind === 'video' ? '视频没能发过来' : '图片没能发过来') : waiting ? (job.kind === 'video' ? '视频发送中' : '图片发送中') : job.kind === 'video' ? '视频加载中' : '图片加载中';
    const note = failed ? `暂时出了点问题：${job.error || '请稍后再试'}` : waiting ? '正在发送...' : '正在加载...';
    return `<article class="message assistant" data-generation-message="${esc(message.id)}">${avatar(activePersona())}<div class="message-wrap"><div class="job-card${failed ? ' failed' : ''}"><div class="job-top"><span class="pulse-icon">${job.kind === 'video' ? '◉' : '◈'}</span><div><strong>${title}</strong></div><span class="job-percent">${failed ? '—' : '…'}</span></div><div class="progress"><i></i></div><p class="job-note">${esc(note)}</p>${failed ? '' : '<div class="skeleton"><span></span><span></span><span></span></div>'}</div></div></article>`;
}

function messageHtml(message) {
    if (message.generation) return generationHtml(message);
    const who = message.role === 'user' ? {name: '你', color: '#a89b71'} : activePersona();
    const thinking = message.thinking ? `<div class="thinking" aria-label="正在思考"><i></i><i></i><i></i><span>稍等，我想一下</span></div>` : '';
    const body = `${thinking}${message.text ? `<div class="markdown">${renderMarkdown(message.text)}</div>` : ''}${(message.attachments || []).map(attachmentHtml).join('')}`;
    return `<article class="message ${message.role}">${avatar(who)}<div class="message-wrap"><div class="bubble${message.thinking ? ' thinking-bubble' : ''}">${body}</div>${message.learned?.length ? `<p class="learned">已记住：${message.learned.map(item => `${esc(item.key)} · ${esc(item.value)}`).join('；')}</p>` : ''}</div></article>`;
}

function loadingHtml(job) {
    return generationHtml({id: job.id, generation: {...job, status: 'submitting'}});
}

function renderMessages() {
    const stream = $('#stream');
    if (!messages.length) {
        stream.innerHTML = `<div class="welcome">${avatar(activePersona())}<h2>今天想聊点什么？</h2><p>${esc(activePersona().role)}</p><div class="examples"><button>帮我制定今天的训练计划</button><button>给我一套练腿动作，并配上每个动作的示范图</button></div></div>`;
    } else stream.innerHTML = messages.map(messageHtml).join('');
    stream.scrollTop = stream.scrollHeight;
    bindExamples();
}

function renderPersonaList() {
    $('#personas').innerHTML = state.personas.filter(persona => persona.enabled).map(persona => `<div class="persona-row ${persona.id === activePersonaId ? 'selected' : ''}"><button class="persona" data-persona="${persona.id}">${avatar(persona, true)}<span><strong>${esc(persona.name)}</strong><small>${esc(persona.role)}</small></span><i></i></button><button class="persona-more" data-edit-persona="${persona.id}" aria-label="编辑 ${esc(persona.name)}">⋯</button></div>`).join('');
    $('#personas').querySelectorAll('.persona').forEach(button => {
        button.onclick = () => {
            switchPersona(button.dataset.persona);
            // 关闭移动端菜单
            const rail = document.querySelector('.rail');
            const people = document.querySelector('.people');
            const overlay = document.querySelector('#mobile-overlay');
            if (rail) rail.classList.remove('mobile-visible');
            if (people) people.classList.remove('mobile-visible');
            if (overlay) overlay.classList.remove('visible');
        };
    });
    $('#personas').querySelectorAll('.persona-more').forEach(button => button.onclick = () => openPersonaEditor(button.dataset.editPersona));
}

function renderMemory() {
    const persona = activePersona();
    const memories = state.memories.filter(memory => memory.personaId === persona.id);
    $('#memory-list').innerHTML = memories.length ? memories.map(memory => `<li><span>${esc(memory.key)}</span><p>${esc(memory.value)}</p><small>用户偏好 · ${new Date(memory.updatedAt).toLocaleDateString('zh-CN')}</small></li>`).join('') : '<li class="no-memory">还没有可长期保留的偏好</li>';
}

function render() {
    const persona = activePersona();
    document.title = `${persona.name} · 知觉`;
    $('#title').textContent = persona.name;
    $('#subtitle').textContent = persona.role;
    $('#header-avatar').innerHTML = avatar(persona);
    $('#personality-preview').textContent = persona.basePrompt;
    $('#memory-count').textContent = state.memories.filter(memory => memory.personaId === persona.id).length;
    renderPersonaList();
    renderMemory();
    renderMessages();
}

function build() {
    document.querySelector('#app').innerHTML = `<button class="mobile-menu-toggle" id="mobile-menu-toggle" aria-label="菜单">☰</button><div class="mobile-overlay" id="mobile-overlay"></div><main class="app-shell"><aside class="rail"><div class="logo">知</div><button class="rail-button active" aria-label="聊天" title="聊天">◌</button><button class="rail-button" id="memory-toggle" aria-label="共同记忆" title="共同记忆">◫</button><button class="rail-button" id="console-toggle" aria-label="运行控制台" title="运行控制台">⌘</button><span></span><button class="rail-button" id="settings-toggle" aria-label="设置" title="设置">⚙</button></aside><aside class="people"><header><div class="wordmark">知觉<small>COMPANION</small></div><button class="add-persona" id="add-persona" aria-label="添加人格" title="添加人格">＋</button></header><div class="section-label">联系人</div><nav id="personas"></nav><div class="evolution"><span class="spark">✦</span><div><strong>最近联系</strong><p>聊过的事会留在这里</p></div></div></aside><section class="chat"><header class="chat-header"><div class="identity" id="header-avatar"></div><div><h1 id="title"></h1><p id="subtitle"></p></div><button id="info-toggle" class="header-action">记忆 <b id="memory-count">0</b></button></header><div class="stream" id="stream"></div><section class="composer-zone"><div class="file-queue" id="file-queue"></div><div class="composer" id="drop-zone"><textarea id="input" placeholder="给 ${esc(activePersona().name)} 发消息..." rows="1"></textarea><div class="tool-row"><label title="添加图片、视频或文件" aria-label="添加图片、视频或文件" class="icon-tool">＋<input id="file-input" type="file" multiple accept="image/*,video/*,.pdf,.txt"></label><button id="emoji" class="icon-tool" aria-label="表情" title="表情">☺</button><div id="emoji-menu" class="emoji-menu">😀　🙂　💪　✨　🎨　🎬　❤️</div><span class="enter-hint">Enter 发送</span><button id="send" class="send" aria-label="发送">↑</button></div></div></section></section><aside class="memory-panel" id="memory-panel"><header><div><p>SHARED NOTES</p><h2>共同记忆</h2></div><button id="close-memory" class="close" aria-label="关闭记忆面板">×</button></header><p class="memory-explain">这里会保留你们聊过的重要偏好；新的、更具体的想法会自然更新旧记录。</p><ul id="memory-list"></ul><section class="prompt-origin"><span>基础人格</span><p id="personality-preview"></p></section></aside><aside class="console-panel" id="console-panel"><header><div><p>RUNTIME CONSOLE</p><h2>生成调用</h2></div><button id="close-console" class="close" aria-label="关闭控制台">×</button></header><div class="console-actions"><button id="refresh-console" class="quiet">刷新</button><button id="clear-console" class="quiet">清空</button></div><div id="console-log" class="console-log"></div></aside></main><dialog id="settings"><form method="dialog" id="settings-form"><header><div><p>LOCAL CONFIGURATION</p><h2>系统设置</h2></div><button class="close" value="cancel" aria-label="关闭设置">×</button></header><div class="settings-body"><section><h3>MTPLX</h3><label>服务地址<input name="lmStudioUrl"></label><label>API Key<input name="lmStudioApiKey" type="password" autocomplete="off" placeholder="MTPLX API Key"></label><label>模型 ID<input name="model" placeholder="点击下方按钮自动发现"></label><button type="button" id="detect-model" class="quiet">读取可用模型</button><small id="model-result">使用 MTPLX 的 OpenAI 兼容接口，API Key 会作为 Bearer Token 发送。</small></section><section><h3>ComfyUI</h3><label>服务地址<input name="comfyUrl"></label><label>生图工作流 JSON<textarea name="imageWorkflow" rows="4" placeholder="粘贴 API 格式工作流 JSON"></textarea></label><label>生视频工作流 JSON<textarea name="videoWorkflow" rows="4" placeholder="粘贴 API 格式工作流 JSON"></textarea></label><small>在工作流的正向提示词节点使用 <code>{{prompt}}</code>。AI 会在需要画面说明时自行调用对应工作流，用户无需输入任何触发词。</small></section></div><footer><button value="cancel" class="quiet">取消</button><button class="save">保存设置</button></footer></form></dialog><dialog id="persona-dialog"><form method="dialog" id="persona-form"><header><div><p>NEW PERSONA</p><h2>添加人格</h2></div><button class="close" value="cancel" aria-label="关闭创建人格">×</button></header><label>名字<input name="name" required placeholder="例如：小燃"></label><label>角色<input name="role" required placeholder="例如：跑步教练"></label><label>基础人格<textarea name="basePrompt" rows="7" required placeholder="定义稳定身份、语气、专业边界与默认行为"></textarea></label><footer><button value="cancel" class="quiet">取消</button><button class="save">创建人格</button></footer></form></dialog>`;
    bind();
    bindMobileMenu();
}

async function switchPersona(personaId) {
    activePersonaId = personaId;
    localStorage.setItem('companion-active-persona', personaId);
    messages = await api(`/api/conversations/${personaId}`);
    render();
    resumePendingGenerations(personaId, messages);
    $('#input').placeholder = `给 ${activePersona().name} 发消息...`;
}

function bindExamples() {
    $('#stream').querySelectorAll('.examples button').forEach(button => button.onclick = () => {
        $('#input').value = button.textContent;
        send();
    });
}

function showFiles() {
    $('#file-queue').innerHTML = attachments.map((file, index) => `<span>${esc(file.name)}<button data-remove="${index}">×</button></span>`).join('');
    $('#file-queue').querySelectorAll('button').forEach(button => button.onclick = () => {
        attachments.splice(Number(button.dataset.remove), 1);
        showFiles();
    });
}

function bind() {
    $('#settings-toggle').onclick = openSettings;
    $('#add-persona').onclick = () => $('#persona-dialog').showModal();
    $('#info-toggle').onclick = toggleMemory;
    $('#memory-toggle').onclick = toggleMemory;
    $('#close-memory').onclick = toggleMemory;
    $('#console-toggle').onclick = toggleConsole;
    $('#close-console').onclick = toggleConsole;
    $('#refresh-console').onclick = refreshConsole;
    $('#clear-console').onclick = clearConsole;
    $('#send').onclick = send;
    $('#input').addEventListener('keydown', event => {
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            send();
        }
    });
    $('#emoji').onclick = () => $('#emoji-menu').classList.toggle('visible');
    $('#emoji-menu').onclick = event => {
        if (event.target.textContent) {
            $('#input').value += event.target.textContent.trim();
            $('#input').focus();
        }
    };
    $('#file-input').onchange = async event => {
        attachments.push(...await toAttachments([...event.target.files]));
        showFiles();
    };
    const drop = $('#drop-zone');
    drop.ondragover = event => {
        event.preventDefault();
        drop.classList.add('drag');
    };
    drop.ondragleave = () => drop.classList.remove('drag');
    drop.ondrop = async event => {
        event.preventDefault();
        drop.classList.remove('drag');
        attachments.push(...await toAttachments([...event.dataTransfer.files]));
        showFiles();
    };
    $('#settings-form').onsubmit = saveSettings;
    $('#detect-model').onclick = detectModel;
    $('#persona-form').onsubmit = addPersona;
}

function bindMobileMenu() {
    const toggle = $('#mobile-menu-toggle');
    const overlay = $('#mobile-overlay');
    const rail = $('.rail');
    const people = $('.people');

    if (!toggle || !overlay || !rail || !people) return;

    const openMenu = () => {
        rail.classList.add('mobile-visible');
        people.classList.add('mobile-visible');
        overlay.classList.add('visible');
    };

    const closeMenu = () => {
        rail.classList.remove('mobile-visible');
        people.classList.remove('mobile-visible');
        overlay.classList.remove('visible');
    };

    toggle.onclick = openMenu;
    overlay.onclick = closeMenu;

    // 选择人格后自动关闭菜单
    document.querySelectorAll('.persona').forEach(button => {
        button.addEventListener('click', closeMenu);
    });
}

async function toAttachments(files) {
    return Promise.all(files.map(file => new Promise(resolve => {
        const reader = new FileReader();
        reader.onload = () => resolve({
            name: file.name,
            kind: file.type.startsWith('image/') ? 'image' : file.type.startsWith('video/') ? 'video' : 'file',
            url: reader.result
        });
        reader.readAsDataURL(file);
    })));
}

function toggleMemory() {
    $('#memory-panel').classList.toggle('open');
}

function consoleItem(entry) {
    const time = new Date(entry.at).toLocaleString('zh-CN', {hour12: false});
    if (entry.type === 'generation') {
        const type = entry.kind === 'video' ? '视频' : '图片';
        const target = entry.promptTarget ? `<span>注入字段：${esc(entry.promptTarget)}</span>` : '';
        const error = entry.error ? `<p class="console-error">${esc(entry.error)}</p>` : '';
        return `<article class="console-entry"><header><strong>${type}生成</strong><time>${time}</time></header><span class="console-status ${entry.status === '提交失败' ? 'failed' : ''}">${esc(entry.status)}</span>${target}<details><summary>完整 ComfyUI 提示词</summary><pre>${esc(entry.prompt || '未提供提示词')}</pre></details><small>${entry.promptId ? `ComfyUI 任务：${esc(entry.promptId)}` : ''}</small>${error}</article>`;
    }
    const title = entry.type === 'memory' ? '人格与记忆整理' : entry.type === 'function' ? `Function call · ${esc(entry.functionName || '')}` : '聊天模型';
    const phase = entry.phase === 'input' ? '输入' : entry.phase === 'output' ? '输出' : '失败';
    const error = entry.error ? `<p class="console-error">${esc(entry.error)}</p>` : '';
    const input = entry.inputPrompt ? `<details open><summary>${entry.type === 'function' ? 'function call 提示词' : entry.type === 'memory' ? '人格/记忆提取提示词' : '传给 LLM 的提示词'}</summary><pre>${esc(entry.inputPrompt)}</pre></details>` : '';
    const content = entry.content !== undefined ? `<details open><summary>LLM 返回 content</summary><pre>${esc(entry.content || '(空内容)')}</pre></details>` : '';
    const parsed = entry.parsed ? `<details><summary>解析后的记忆调整</summary><pre>${esc(JSON.stringify({
        parsed: entry.parsed,
        acceptedMemories: entry.acceptedMemories
    }, null, 2))}</pre></details>` : '';
    return `<article class="console-entry"><header><strong>${title}</strong><time>${time}</time></header><span class="console-status ${entry.phase === 'error' ? 'failed' : ''}">${phase}${entry.personaName ? ` · ${esc(entry.personaName)}` : ''}</span>${input}${content}${parsed}${error}</article>`;
}

async function refreshConsole() {
    const events = await api('/api/console');
    $('#console-log').innerHTML = events.length ? events.map(consoleItem).join('') : '<p class="console-empty">还没有调试记录。</p>';
}

async function clearConsole() {
    await fetch('/api/console', {method: 'DELETE'});
    await refreshConsole();
}

function toggleConsole() {
    const panel = $('#console-panel');
    panel.classList.toggle('open');
    if (panel.classList.contains('open')) refreshConsole().catch(error => {
        $('#console-log').innerHTML = `<p class="console-empty">读取失败：${esc(error.message)}</p>`;
    });
}

function generateUUID() {
    // 如果当前环境支持 randomUUID，直接使用
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
        return crypto.randomUUID();
    }

    // 否则，使用 getRandomValues 降级生成 v4 UUID
    if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
        return '10000000-1000-4000-8000-100000000000'.replace(/[018]/g, c =>
            (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16)
        );
    }

    // 最后的保底方案（使用 Math.random，安全性较低但不报错）
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
        const r = Math.random() * 16 | 0;
        const v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

function openSettings() {
    const form = $('#settings-form');
    Object.entries(state.settings).forEach(([key, value]) => {
        if (form.elements[key]) form.elements[key].value = value || '';
    });
    $('#settings').showModal();
}

async function saveSettings(event) {
    event.preventDefault();
    state.settings = await api('/api/settings', {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(Object.fromEntries(new FormData(event.target)))
    });
    $('#settings').close();
    render();
}

async function detectModel() {
    $('#model-result').textContent = '正在读取...';
    try {
        const data = await api('/api/models');
        const model = data.data?.find(item => !/embedding/i.test(item.id))?.id || data.data?.[0]?.id;
        $('#settings-form').elements.model.value = model || '';
        $('#model-result').textContent = model ? `已选择：${model}` : '未找到可用模型';
    } catch (error) {
        $('#model-result').textContent = error.message;
    }
}

async function addPersona(event) {
    event.preventDefault();
    const persona = await api('/api/personas', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(Object.fromEntries(new FormData(event.target)))
    });
    state.personas.push(persona);
    $('#persona-dialog').close();
    await switchPersona(persona.id);
}

function ensurePersonaEditor() {
    let dialog = document.querySelector('#persona-editor');
    if (dialog) return dialog;
    dialog = document.createElement('dialog');
    dialog.id = 'persona-editor';
    dialog.innerHTML = `<form method="dialog" id="persona-editor-form"><header><div><p>PERSONA SETTINGS</p><h2>编辑人格</h2></div><button class="close" value="cancel" aria-label="关闭编辑">×</button></header><div class="editor-body"><input name="id" type="hidden"><div class="editor-pair"><label>名字<input name="name" required></label><label>角色<input name="role" required></label></div><label>人格颜色<input name="color" type="color"></label><label>有效基础人格<textarea name="basePrompt" rows="9" required></textarea></label></div><footer><button type="button" id="delete-persona" class="danger">删除人格</button><span></span><button value="cancel" class="quiet">取消</button><button class="save">保存修改</button></footer></form>`;
    document.body.append(dialog);
    dialog.querySelector('#persona-editor-form').onsubmit = savePersonaEditor;
    dialog.querySelector('#delete-persona').onclick = deletePersona;
    return dialog;
}

function openPersonaEditor(personaId) {
    const persona = state.personas.find(item => item.id === personaId);
    if (!persona) return;
    const dialog = ensurePersonaEditor();
    const form = dialog.querySelector('form');
    Object.entries(persona).forEach(([key, value]) => {
        if (form.elements[key] && typeof value === 'string') form.elements[key].value = value;
    });
    dialog.showModal();
}

async function savePersonaEditor(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = Object.fromEntries(new FormData(form));
    const persona = await api(`/api/personas/${data.id}`, {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({name: data.name, role: data.role, color: data.color, basePrompt: data.basePrompt})
    });
    const index = state.personas.findIndex(item => item.id === persona.id);
    state.personas[index] = persona;
    document.querySelector('#persona-editor').close();
    render();
}

async function deletePersona() {
    const dialog = document.querySelector('#persona-editor');
    const personaId = dialog.querySelector('[name="id"]').value;
    const persona = state.personas.find(item => item.id === personaId);
    if (!window.confirm(`删除人格“${persona?.name || ''}”？其历史聊天记录会保留在本地，但不会再显示。`)) return;
    try {
        const response = await fetch(`/api/personas/${personaId}`, {method: 'DELETE'});
        if (!response.ok) {
            const data = await response.json();
            throw new Error(data.error || '删除失败');
        }
        state.personas = state.personas.filter(item => item.id !== personaId);
        dialog.close();
        if (activePersonaId === personaId) await switchPersona(state.personas[0].id); else render();
    } catch (error) {
        window.alert(error.message);
    }
}

async function send() {
    if (isSending) return;
    const input = $('#input');
    const text = input.value.trim();
    if (!text && !attachments.length) return;
    const files = attachments;
    attachments = [];
    showFiles();
    input.value = '';
    messages.push({id: generateUUID(), role: 'user', text, attachments: files});
    renderMessages();
    isSending = true;
    $('#send').classList.add('waiting');
    try {
        await streamChat(text, files);
    } catch (error) {
        messages.push({id: generateUUID(), role: 'assistant', text: `消息没能发出：${error.message}`});
        renderMessages();
    } finally {
        isSending = false;
        $('#send').classList.remove('waiting');
    }
}

async function streamChat(text, files) {
    const personaId = activePersonaId;
    const assistant = {id: generateUUID(), role: 'assistant', text: '', thinking: true};
    messages.push(assistant);
    renderMessages();
    const response = await fetch('/api/chat', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({personaId, text, attachments: files})
    });
    if (!response.ok || !response.body) {
        assistant.thinking = false;
        assistant.text = '现在联系不上，请检查一下设置。';
        renderMessages();
        return;
    }
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let jobs = [];
    while (true) {
        const {value, done} = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, {stream: true});
        const events = buffer.split('\n\n');
        buffer = events.pop();
        for (const event of events) {
            const data = event.split('\n').find(line => line.startsWith('data: '))?.slice(6);
            if (!data) continue;
            const payload = JSON.parse(data);
            if (payload.type === 'token') {
                assistant.thinking = false;
                assistant.text += payload.token;
            }
            if (payload.type === 'done') {
                assistant.thinking = false;
                assistant.text = payload.message.text;
                assistant.learned = payload.learned;
                jobs = payload.jobs || [];
                state.memories.push(...payload.learned);
            }
            if (payload.type === 'error') {
                assistant.thinking = false;
                assistant.text = payload.error;
            }
            if (activePersonaId === personaId) {
                renderMessages();
                renderMemory();
            }
        }
    }
    for (const job of jobs) await runGeneration(personaId, job);
}

async function runGeneration(personaId, job) {
    try {
        const queued = await api('/api/generate', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({...job, personaId})
        });
        if (activePersonaId === personaId) {
            messages.push(queued.pendingMessage);
            renderMessages();
        }
    } catch (error) {
        if (activePersonaId === personaId) {
            messages.push({id: generateUUID(), role: 'assistant', text: `附件暂时没能发出：${error.message}`});
            renderMessages();
        }
    }
}

function resumePendingGenerations() {
}

async function refreshState() {
    state = await api('/api/state');
    messages = await api(`/api/conversations/${activePersonaId}`);
    render();
}

async function boot() {
    state = await api('/api/state');
    if (!state.personas.some(persona => persona.id === activePersonaId)) activePersonaId = state.personas[0].id;
    build();
    messages = await api(`/api/conversations/${activePersonaId}`);
    render();
    window.addEventListener('focus', () => refreshState().catch(() => {
    }));
    setInterval(() => {
        if (!isSending && document.visibilityState === 'visible') refreshState().catch(() => {
        });
    }, 5000);
}

boot().catch(error => {
    document.body.innerHTML = `<pre>启动失败：${esc(error.message)}</pre>`;
});
