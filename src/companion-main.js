const $ = selector => document.querySelector(selector);
const esc = value => String(value ?? '').replace(/[&<>'"]/g, char => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'}[char]));
const formatTime = value => value ? new Intl.DateTimeFormat('zh-CN', {month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit'}).format(new Date(value)) : '';
const initials = name => Array.from(name || '?').slice(0, 1).join('');

let appState = {settings: {}, personas: [], groups: [], activityUnread: false};
let activePersonaId = localStorage.getItem('companion-active-persona') || '';
let activeContactGroupId = localStorage.getItem('companion-active-group') || '';
let chatDraft = '';
let activeMessages = [];
let activeDetail = null;
let activityItems = [];
let activityPersonaId = null;
let activityNextCursor = null;
let activityLoadingMore = false;
let activityPagesLoaded = 0;
let hiddenActivityItems = [];
let hiddenActivityNextCursor = null;
let hiddenActivityPersonaId = null;
let hiddenActivityLoadingMore = false;
let currentView = 'contacts';
let isSending = false;
let commentingActivityId = null;
const dialogReturnFocus = new WeakMap();
let inspectorMediaRefresh = null;

function chatViewSnapshot() {
    const stream = $('#message-stream');
    const input = $('#chat-input');
    if (input && input.value !== chatDraft) chatDraft = input.value;
    if (!stream) return {draft: chatDraft, stream: null, input: null};
    const distanceFromBottom = stream.scrollHeight - stream.clientHeight - stream.scrollTop;
    return {
        draft: chatDraft,
        stream: {scrollTop: stream.scrollTop, atBottom: distanceFromBottom < 36},
        input: input && document.activeElement === input ? {start: input.selectionStart, end: input.selectionEnd} : null
    };
}

function pinChatToLatest(stream) {
    const pin = () => {
        if (!stream.isConnected) return;
        stream.scrollTop = stream.scrollHeight;
    };
    // The stream's final height can change across grid layout, font paint, and
    // media intrinsic-size resolution. Pin at each of those early frames so a
    // newly opened conversation reliably starts at its newest message.
    pin();
    requestAnimationFrame(() => {
        pin();
        requestAnimationFrame(pin);
    });
    window.setTimeout(pin, 120);
}

function restoreChatView(snapshot, {followLatest = false} = {}) {
    const stream = $('#message-stream');
    const input = $('#chat-input');
    if (!stream) return;
    const shouldFollowLatest = followLatest || !snapshot?.stream || snapshot.stream.atBottom;
    if (shouldFollowLatest) pinChatToLatest(stream);
    requestAnimationFrame(() => {
        if (!shouldFollowLatest) stream.scrollTop = Math.min(snapshot.stream.scrollTop, stream.scrollHeight - stream.clientHeight);
        if (snapshot?.input && input && !isSending) {
            input.focus({preventScroll: true});
            input.setSelectionRange(snapshot.input.start, snapshot.input.end);
        }
    });
}

async function api(path, options = {}) {
    const response = await fetch(path, options);
    if (response.status === 204) return null;
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || `请求失败 (${response.status})`);
    return body;
}

function activePersona() {
    return appState.personas.find(item => item.id === activePersonaId) || appState.personas[0] || null;
}

function contactGroups() {
    return Array.isArray(appState.groups) ? appState.groups : [];
}

function activeContactGroup() {
    return contactGroups().find(group => String(group.id) === activeContactGroupId) || null;
}

function avatar(persona, className = '') {
    return `<span class="avatar ${className}" style="--avatar:${esc(persona.color)}">${esc(initials(persona.name))}</span>`;
}

function showDialog(dialog, trigger = document.activeElement) {
    if (!dialog.open) {
        dialogReturnFocus.set(dialog, trigger instanceof HTMLElement ? trigger : null);
        dialog.showModal();
    }
    const heading = dialog.querySelector('h2');
    if (heading) {
        heading.id ||= `${dialog.id}-title`;
        dialog.setAttribute('aria-labelledby', heading.id);
    }
    requestAnimationFrame(() => {
        if (!dialog.contains(document.activeElement)) (dialog.querySelector('[data-initial-focus], input, textarea, select, .close-dialog, button') || dialog).focus();
    });
}

function closeDialog(dialog) {
    if (dialog.open) dialog.close('cancel');
}

function bindMediaFallbacks(scope = document) {
    scope.querySelectorAll('[data-media-kind]').forEach(media => media.addEventListener('error', event => {
        const asset = event.currentTarget;
        const kind = asset.dataset.mediaKind === 'video' ? '视频' : '图片';
        const fallback = document.createElement('div');
        fallback.className = `${asset.classList.contains('activity-media') ? 'activity-media' : 'message-media'} skeleton failed`;
        fallback.setAttribute('role', 'status');
        fallback.textContent = `${kind}暂时不可用`;
        asset.replaceWith(fallback);
    }, {once: true}));
}

function build() {
    $('#app').innerHTML = `
        <div class="app-frame">
            <aside class="rail" aria-label="主导航">
                <button class="brand" id="brand-button" aria-label="知觉首页">知</button>
                <button class="rail-action active" data-view="contacts" aria-label="联系人" title="联系人"><span>⌁</span></button>
                <button class="rail-action" data-view="activity" aria-label="动态" title="动态"><span>◉</span><i id="activity-dot"></i></button>
                <span class="rail-spacer"></span>
                <button class="rail-action" id="settings-button" aria-label="设置" title="设置">⚙</button>
            </aside>
            <aside class="sidebar" id="sidebar">
                <header class="sidebar-header"><div><strong>知觉</strong><small>COMPANION</small></div><button class="icon-button" id="create-button" aria-label="创建人格" title="创建人格">＋</button></header>
                <nav class="sidebar-tabs"><button data-view="contacts" class="active">联系人</button><button data-view="activity">动态</button></nav>
                <div class="persona-list" id="persona-list"></div>
                <button class="new-persona" id="sidebar-create">＋ 创建一个陪伴者</button>
            </aside>
            <main class="main-pane" id="main-pane"></main>
            <nav class="mobile-nav" aria-label="主导航"><button data-view="contacts" aria-label="联系人" title="联系人">⌁</button><button data-view="activity" aria-label="动态" title="动态">◉<i id="mobile-activity-dot"></i></button><button data-view="settings" aria-label="设置" title="设置">⚙</button></nav>
        </div>
        <dialog id="persona-dialog"></dialog>
        <dialog id="settings-dialog"></dialog>
        <dialog id="inspector-dialog"></dialog>`;
    bindStatic();
    document.querySelectorAll('dialog').forEach(dialog => dialog.addEventListener('close', () => {
        const trigger = dialogReturnFocus.get(dialog);
        dialogReturnFocus.delete(dialog);
        if (trigger?.isConnected) trigger.focus();
    }));
}

function renderSidebar() {
    $('#persona-list').innerHTML = appState.personas.length ? appState.personas.map(persona => `
        <button class="persona-row ${persona.id === activePersonaId ? 'selected' : ''}" data-persona="${esc(persona.id)}">
            ${avatar(persona)}<span class="persona-copy"><b>${esc(persona.name)}</b><small>${esc(persona.currentSituation || persona.role)}</small></span>
            ${persona.unreadCount ? `<em>${persona.unreadCount > 99 ? '99+' : persona.unreadCount}</em>` : ''}
        </button>`).join('') : '<div class="empty-list">还没有陪伴者</div>';
    $('#activity-dot').hidden = !appState.activityUnread;
    $('#mobile-activity-dot').hidden = !appState.activityUnread;
    document.querySelectorAll('[data-view]').forEach(button => button.classList.toggle('active', button.dataset.view === currentView));
    $('#persona-list').querySelectorAll('[data-persona]').forEach(button => {
        button.setAttribute('aria-current', button.dataset.persona === activePersonaId ? 'page' : 'false');
        button.onclick = () => selectPersona(button.dataset.persona);
    });
}

function render() {
    renderSidebar();
    if (!appState.personas.length && !(currentView === 'contacts' && contactGroups().length)) return renderEmptyStart();
    if (currentView === 'contacts') return renderContacts();
    if (currentView === 'settings') return renderSettingsPage();
    if (currentView === 'activity') return renderActivities();
    return renderChat();
}

function renderContacts() {
    const selectedGroup = activeContactGroup();
    const visiblePersonas = selectedGroup
        ? appState.personas.filter(persona => String(persona.groupId || '') === String(selectedGroup.id))
        : appState.personas;
    const empty = `<div class="contact-empty"><p>${selectedGroup ? `“${esc(selectedGroup.name)}”里还没有陪伴者。` : '还没有陪伴者。'}</p><button class="quiet" id="contacts-empty-create">创建一个陪伴者</button></div>`;
    $('#main-pane').innerHTML = `<section class="contacts-view"><header class="pane-header contacts-header"><button type="button" class="contacts-title" id="contacts-group-trigger" aria-label="选择联系人分组"><span class="header-copy"><h1>联系人</h1><p>${selectedGroup ? `${esc(selectedGroup.name)} · ${visiblePersonas.length} 位陪伴者` : '所有陪伴者的聊天'}</p></span></button><button class="text-icon" id="contacts-create" aria-label="创建陪伴者" title="创建陪伴者">＋</button></header><div class="contacts-stream">${visiblePersonas.length ? visiblePersonas.map(persona => `<button class="contact-row" data-contact-persona="${esc(persona.id)}">${avatar(persona)}<span class="persona-copy"><b>${esc(persona.name)}</b><small>${esc(persona.currentSituation || persona.role || '开始聊天')}</small></span>${persona.unreadCount ? `<em>${persona.unreadCount > 99 ? '99+' : persona.unreadCount}</em>` : ''}<i>›</i></button>`).join('') : empty}</div></section>`;
    $('#contacts-group-trigger').onclick = event => openGroupPicker(event.currentTarget);
    $('#contacts-create').onclick = openPersonaWizard;
    $('#contacts-empty-create')?.addEventListener('click', openPersonaWizard);
    document.querySelectorAll('[data-contact-persona]').forEach(button => button.onclick = async () => {
        currentView = 'chat';
        await selectPersona(button.dataset.contactPersona);
    });
}

function renderSettingsPage() {
    $('#main-pane').innerHTML = `<section class="settings-view"><header class="pane-header"><div class="header-copy"><h1>设置</h1><p>管理应用与陪伴者</p></div></header><nav class="settings-menu"><button id="settings-model"><span>⚙</span><div><b>系统设置</b><small>模型、媒体与服务配置</small></div><i>›</i></button><button id="settings-create"><span>＋</span><div><b>创建陪伴者</b><small>开始一段新的陪伴关系</small></div><i>›</i></button><button id="settings-personas"><span>⌁</span><div><b>管理联系人</b><small>查看和切换已有陪伴者</small></div><i>›</i></button></nav></section>`;
    $('#settings-model').onclick = openSettings;
    $('#settings-create').onclick = openPersonaWizard;
    $('#settings-personas').onclick = openPersonaPicker;
}

function renderEmptyStart() {
    $('#main-pane').innerHTML = `<section class="empty-start"><div class="empty-mark">知</div><h1>创建一个有自己生活的陪伴者</h1><p>从几句关于她是谁、如何生活的描述开始。设定完成后，她会在自己的世界里自然地上课、休息、见朋友，并把重要瞬间留在动态里。</p><button class="primary" id="empty-create">开始创建</button></section>`;
    $('#empty-create').onclick = openPersonaWizard;
}

function messageMediaHtml(message) {
    const attachments = Array.isArray(message.attachments) ? message.attachments : [];
    const generation = message.generation && typeof message.generation === 'object' ? message.generation : null;
    if (appState.settings?.simplifiedMediaMode && (attachments.length || generation)) {
        const kind = (attachments[0]?.kind || generation?.kind) === 'video' ? '视频' : '图片';
        const failed = generation?.status === 'failed';
        const request = generation?.request ? `<small>${esc(generation.request)}</small>` : '';
        const status = attachments.length
            ? `${kind}已生成（简化模式未加载）`
            : failed ? `${kind}暂时不可用` : `${kind}生成中（简化模式）`;
        return `<div class="message-media simplified-media ${failed ? 'failed' : ''}"><span>${status}</span>${request}</div>`;
    }
    if (attachments.length) return attachments.map(asset => asset.kind === 'video'
        ? `<video class="message-media" controls preload="metadata" src="${esc(asset.url)}" data-media-kind="video">视频无法播放</video>`
        : `<img class="message-media" src="${esc(asset.url)}" alt="生成的图片" data-media-kind="image">`).join('');
    if (!generation) return '';
    const failed = generation.status === 'failed';
    const kind = generation.kind === 'video' ? '视频' : '图片';
    const request = generation.request ? `<small>${esc(generation.request)}</small>` : '';
    return `<div class="message-media skeleton ${failed ? 'failed' : ''}"><span>${failed ? `${kind}暂时不可用` : `${kind}生成中`}</span>${request}</div>`;
}

function messageHtml(message) {
    if (message.transient === 'typing') return `<article class="message incoming transient-typing" aria-live="polite"><div class="bubble"><span class="pending-text">正在输入…</span></div></article>`;
    const content = esc(message.text).replace(/\n/g, '<br>');
    const media = messageMediaHtml(message);
    const placeholder = !content && !media ? '<span class="pending-text">正在输入…</span>' : '';
    return `<article class="message ${message.role === 'user' ? 'outgoing' : 'incoming'}"><div class="bubble">${content ? `<div class="message-copy">${content}</div>` : placeholder}${media}<time>${formatTime(message.createdAt)}</time></div></article>`;
}

function renderChat(options = {}) {
    const persona = activePersona();
    if (!persona) return renderEmptyStart();
    const snapshot = chatViewSnapshot();
    $('#main-pane').innerHTML = `
        <section class="chat-view">
            <header class="pane-header chat-pane-header">
                <button class="text-icon" id="chat-back" aria-label="返回联系人" title="返回联系人">‹</button>
                <button class="chat-title" id="profile-button" aria-label="查看人格详情">${avatar(persona, 'header-avatar')}<span><h1>${esc(persona.name)}</h1><p>${esc(persona.currentSituation || persona.role)}${persona.mood ? ` · ${esc(persona.mood)}` : ''}</p></span></button>
                <div class="chat-tools"><button class="text-icon" id="chat-settings" aria-label="设置" title="设置">⋯</button></div>
            </header>
            <div class="message-stream" id="message-stream">${activeMessages.length ? activeMessages.map(messageHtml).join('') : `<div class="chat-empty">${avatar(persona, 'large')}<h2>${esc(persona.name)}</h2><p>${esc(persona.currentSituation || '正在过自己的日常。')}</p><button class="soft-prompt" data-prompt="今天过得怎么样？">今天过得怎么样？</button></div>`}</div>
            <form class="composer" id="composer"><textarea id="chat-input" rows="1" placeholder="给 ${esc(persona.name)} 发消息" aria-label="消息内容">${esc(chatDraft)}</textarea><button class="send-button" aria-label="发送" title="发送" ${isSending ? 'disabled' : ''}>↑</button></form>
        </section>`;
    $('#composer').onsubmit = sendMessage;
    $('#chat-back').onclick = () => { currentView = 'contacts'; render(); };
    $('#profile-button').onclick = () => openPersonaDetail(persona.id);
    $('#chat-settings').onclick = () => appState.debugInspector ? openInspector(persona.id) : openSettings();
    $('#chat-input').addEventListener('keydown', event => {
        if (event.key !== 'Enter' || event.shiftKey) return;
        event.preventDefault();
        $('#composer').requestSubmit();
    });
    $('#chat-input').addEventListener('input', event => { chatDraft = event.currentTarget.value; });
    bindMediaFallbacks($('#message-stream'));
    $('[data-prompt]')?.addEventListener('click', event => {
        chatDraft = event.currentTarget.dataset.prompt;
        $('#chat-input').value = chatDraft;
        $('#chat-input').focus();
    });
    restoreChatView(snapshot, options);
}

function mediaHtml(activity) {
    if (!activity.mediaMode || activity.mediaMode === 'none') return '';
    const asset = Array.isArray(activity.media) ? activity.media[0] : null;
    if (asset) return asset.kind === 'video'
        ? `<video class="activity-media" controls preload="metadata" src="${esc(asset.url)}" data-media-kind="video">视频无法播放</video>`
        : `<img class="activity-media" src="${esc(asset.url)}" alt="${esc(activity.persona?.name || '陪伴者')}分享的图片" data-media-kind="image">`;
    const failed = activity.mediaStatus === 'failed';
    return `<div class="activity-media skeleton ${failed ? 'failed' : ''}"><span>${failed ? '图片生成暂不可用' : '图片生成中'}</span></div>`;
}

function activityHtml(activity) {
    const persona = activity.persona;
    const commentComposer = commentingActivityId === activity.id
        ? `<form class="comment-form" data-comment-form="${esc(activity.id)}"><label>评论<input id="comment-${esc(activity.id)}" maxlength="500" placeholder="写下你的评论" aria-label="评论内容"></label><button aria-label="发布评论">↑</button><button type="button" class="quiet" data-cancel-comment="${esc(activity.id)}">取消</button></form>`
        : '';
    return `<article class="activity-card" data-activity="${esc(activity.id)}">
        <header><button class="activity-author" data-open-persona="${esc(persona.id)}" aria-label="查看 ${esc(persona.name)} 的资料">${avatar(persona, 'small')}<span><b>${esc(persona.name)}</b><small>${formatTime(activity.createdAt)} · ${esc(persona.currentSituation || persona.role)}</small></span></button><button class="more-button" data-hide="${esc(activity.id)}" aria-label="隐藏动态" title="隐藏动态">···</button></header>
        <p class="activity-body">${esc(activity.content).replace(/\n/g, '<br>')}</p>${mediaHtml(activity)}
        <footer><button class="reaction ${activity.liked ? 'liked' : ''}" data-like="${esc(activity.id)}" aria-pressed="${activity.liked ? 'true' : 'false'}" aria-label="${activity.liked ? '取消赞' : '赞'}这条动态">♡ <span>${activity.liked ? '已赞' : '赞'}</span></button><button data-focus-comment="${esc(activity.id)}">评论</button><button data-chat-persona="${esc(persona.id)}">私聊</button></footer>
        <div class="comment-list">${(activity.comments || []).map(comment => `<p><b>${esc(comment.authorName)}</b><span>${esc(comment.content)}</span></p>`).join('')}</div>
        ${commentComposer}
    </article>`;
}

function renderActivities() {
    const momentsPersona = activityPersonaId ? appState.personas.find(item => item.id === activityPersonaId) : null;
    const loadMore = activityNextCursor ? `<div class="load-more"><button class="quiet" id="load-more-activities" ${activityLoadingMore ? 'disabled' : ''}>${activityLoadingMore ? '正在加载...' : '加载更多'}</button></div>` : '';
    $('#main-pane').innerHTML = `<section class="activity-view"><header class="pane-header"><div class="header-copy"><h1>${momentsPersona ? `${esc(momentsPersona.name)}的动态` : '动态'}</h1><p>${momentsPersona ? '只属于她的生活瞬间' : '所有陪伴者的生活瞬间'}</p></div>${momentsPersona ? '<button class="quiet" id="all-activities">全部动态</button>' : ''}<button class="refresh-button" id="refresh-activity" aria-label="刷新动态" title="刷新动态">↻</button></header><div class="activity-stream" id="activity-stream">${activityItems.length ? activityItems.map(activityHtml).join('') : '<div class="activity-empty">还没有动态。陪伴者的日常和事件会自然地出现在这里。</div>'}${loadMore}</div></section>`;
    appState.activityUnread = false;
    api('/api/companion/activities/read', {method: 'POST'}).catch(() => {});
    bindActivityEvents();
    bindMediaFallbacks($('#activity-stream'));
}

function bindActivityEvents() {
    $('#refresh-activity').onclick = () => refreshActivities();
    $('#load-more-activities')?.addEventListener('click', () => loadMoreActivities());
    $('#all-activities')?.addEventListener('click', () => { activityPersonaId = null; loadActivities(); });
    document.querySelectorAll('[data-open-persona]').forEach(button => button.onclick = () => openPersonaDetail(button.dataset.openPersona));
    document.querySelectorAll('[data-chat-persona]').forEach(button => button.onclick = async () => {
        currentView = 'chat';
        await selectPersona(button.dataset.chatPersona);
    });
    document.querySelectorAll('[data-like]').forEach(button => button.onclick = async () => {
        const activity = activityItems.find(item => item.id === button.dataset.like);
        try {
            await api(`/api/companion/activities/${activity.id}/like`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({liked: !activity.liked})});
            activity.liked = !activity.liked;
            renderActivities();
        } catch (error) { window.alert(error.message); }
    });
    document.querySelectorAll('[data-hide]').forEach(button => button.onclick = async () => {
        if (!window.confirm('隐藏这条动态？之后可以在调试阶段通过接口恢复。')) return;
        try {
            await api(`/api/companion/activities/${button.dataset.hide}/hide`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({hidden: true})});
            if (commentingActivityId === button.dataset.hide) commentingActivityId = null;
            activityItems = activityItems.filter(item => item.id !== button.dataset.hide);
            renderActivities();
        } catch (error) { window.alert(error.message); }
    });
    document.querySelectorAll('[data-comment-form]').forEach(form => form.onsubmit = async event => {
        event.preventDefault();
        const activity = activityItems.find(item => item.id === form.dataset.commentForm);
        const input = form.querySelector('input');
        const content = input.value.trim();
        if (!content) return;
        try {
            const comment = await api(`/api/companion/activities/${activity.id}/comments`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({content})});
            activity.comments.push(comment);
            commentingActivityId = null;
            renderActivities();
        } catch (error) { window.alert(error.message); }
    });
    document.querySelectorAll('[data-focus-comment]').forEach(button => button.onclick = () => {
        commentingActivityId = button.dataset.focusComment;
        renderActivities();
        requestAnimationFrame(() => document.getElementById(`comment-${commentingActivityId}`)?.focus());
    });
    document.querySelectorAll('[data-cancel-comment]').forEach(button => button.onclick = () => {
        commentingActivityId = null;
        renderActivities();
    });
    document.querySelectorAll('[data-comment-form] input').forEach(input => input.addEventListener('keydown', event => {
        if (event.key !== 'Escape') return;
        commentingActivityId = null;
        renderActivities();
    }));
}

async function loadBootstrap() {
    const nextState = await api('/api/companion/bootstrap');
    appState = {...nextState, groups: Array.isArray(nextState.groups) ? nextState.groups : []};
    if (!appState.personas.some(persona => persona.id === activePersonaId)) activePersonaId = appState.personas[0]?.id || '';
    if (!contactGroups().some(group => String(group.id) === activeContactGroupId)) {
        activeContactGroupId = String(contactGroups().find(group => group.isDefault)?.id || contactGroups()[0]?.id || '');
        if (activeContactGroupId) localStorage.setItem('companion-active-group', activeContactGroupId);
        else localStorage.removeItem('companion-active-group');
    }
}

async function selectPersona(personaId, {followLatest = true} = {}) {
    activePersonaId = personaId;
    localStorage.setItem('companion-active-persona', personaId);
    const messages = await api(`/api/companion/conversations/${personaId}`);
    activeMessages = messages.items;
    await loadBootstrap();
    if (currentView === 'chat') renderChat({followLatest});
    else render();
}

async function loadActivities(personaId = activityPersonaId) {
    commentingActivityId = null;
    const nextPersonaId = personaId || null;
    const params = new URLSearchParams();
    if (nextPersonaId) params.set('personaId', nextPersonaId);
    const feed = await api(`/api/companion/activities${params.size ? `?${params}` : ''}`);
    activityPersonaId = nextPersonaId;
    activityItems = feed.items;
    activityNextCursor = feed.nextCursor;
    activityLoadingMore = false;
    activityPagesLoaded = 1;
    renderActivities();
}

async function loadMoreActivities() {
    if (!activityNextCursor || activityLoadingMore) return;
    activityLoadingMore = true;
    renderActivities();
    try {
        const params = new URLSearchParams({cursor: activityNextCursor});
        if (activityPersonaId) params.set('personaId', activityPersonaId);
        const feed = await api(`/api/companion/activities?${params}`);
        const seen = new Set(activityItems.map(item => item.id));
        activityItems = [...activityItems, ...feed.items.filter(item => !seen.has(item.id))];
        activityNextCursor = feed.nextCursor;
        activityPagesLoaded += 1;
    } catch (error) {
        window.alert(error.message);
    } finally {
        activityLoadingMore = false;
        if (currentView === 'activity') renderActivities();
    }
}

async function refreshActivities() {
    commentingActivityId = null;
    const params = new URLSearchParams();
    if (activityPersonaId) params.set('personaId', activityPersonaId);
    const feed = await api(`/api/companion/activities${params.size ? `?${params}` : ''}`);
    const seen = new Set(feed.items.map(item => item.id));
    activityItems = activityPagesLoaded <= 1 ? feed.items : [...feed.items, ...activityItems.filter(item => !seen.has(item.id))];
    if (activityPagesLoaded <= 1) activityNextCursor = feed.nextCursor;
    if (currentView === 'activity') renderActivities();
}

async function sendMessage(event) {
    event.preventDefault();
    if (isSending) return;
    const input = $('#chat-input');
    const text = chatDraft.trim();
    if (!text) return;
    const personaId = activePersonaId;
    activeMessages.push({id: `local-${Date.now()}`, role: 'user', text, createdAt: new Date().toISOString()});
    const pending = {id: `pending-${Date.now()}`, role: 'assistant', transient: 'typing', text: '', createdAt: new Date().toISOString()};
    activeMessages.push(pending);
    isSending = true;
    let completed = false;
    renderChat({followLatest: true});
    try {
        const response = await fetch('/api/companion/chat', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({personaId, text})});
        if (!response.ok || !response.body) throw new Error((await response.json().catch(() => ({}))).error || '发送失败');
        chatDraft = '';
        if ($('#chat-input')) $('#chat-input').value = '';
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        while (true) {
            const {value, done} = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, {stream: true});
            const frames = buffer.split('\n\n');
            buffer = frames.pop() || '';
            for (const frame of frames) {
                const raw = frame.split('\n').find(line => line.startsWith('data: '))?.slice(6);
                if (!raw) continue;
                const payload = JSON.parse(raw);
                if (payload.type === 'token') pending.text += payload.token;
                if (payload.type === 'done') {
                    const completedMessages = Array.isArray(payload.messages) && payload.messages.length ? payload.messages : [payload.message].filter(Boolean);
                    const pendingIndex = activeMessages.indexOf(pending);
                    if (pendingIndex !== -1) activeMessages.splice(pendingIndex, 1, ...completedMessages);
                    completed = true;
                }
                if (payload.type === 'error') throw new Error(payload.error || '发送失败');
                if (currentView === 'chat' && activePersonaId === personaId) renderChat({followLatest: true});
            }
        }
    } catch (error) {
        activeMessages = activeMessages.filter(message => message !== pending);
        window.alert(`消息没有收到回复：${error.message}`);
    } finally {
        if (!completed) activeMessages = activeMessages.filter(message => message !== pending);
        isSending = false;
        await loadBootstrap().catch(() => {});
        if (currentView === 'chat' && activePersonaId === personaId) renderChat({followLatest: completed});
    }
}

function openGroupWizard(trigger = document.activeElement) {
    const dialog = $('#persona-dialog');
    dialog.innerHTML = `<form class="persona-wizard group-wizard" id="group-form"><header><div><small>CONTACT GROUP</small><h2>创建分组</h2></div><button type="button" class="close-dialog" id="close-group" aria-label="关闭">×</button></header><div class="wizard-body"><p class="wizard-intro">把陪伴者按你想要的方式整理起来，之后可以在联系人页快速切换。</p><label>分组名称<input name="name" maxlength="60" required data-initial-focus placeholder="例如：学习伙伴"></label></div><footer class="wizard-footer"><button type="button" class="quiet" id="cancel-group">取消</button><button class="primary">创建分组</button></footer></form>`;
    showDialog(dialog, trigger);
    $('#close-group').onclick = () => closeDialog(dialog);
    $('#cancel-group').onclick = () => closeDialog(dialog);
    $('#group-form').onsubmit = async event => {
        event.preventDefault();
        const form = event.currentTarget;
        if (!form.reportValidity()) return;
        const submit = form.querySelector('.primary');
        submit.disabled = true;
        try {
            const created = await api('/api/companion/groups', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({name: String(new FormData(form).get('name') || '').trim()})});
            await loadBootstrap();
            const group = created?.group || created;
            if (group?.id) {
                activeContactGroupId = String(group.id);
                localStorage.setItem('companion-active-group', activeContactGroupId);
            }
            dialog.close('submitted');
            render();
        } catch (error) {
            submit.disabled = false;
            window.alert(error.message);
        }
    };
}

function openGroupPicker(trigger = document.activeElement) {
    const dialog = $('#persona-dialog');
    const groups = contactGroups();
    const groupItems = groups.length ? groups.map(group => `<button type="button" class="group-choice ${String(group.id) === activeContactGroupId ? 'selected' : ''}" data-contact-group="${esc(group.id)}"><span><b>${esc(group.name)}</b><small>${Number(group.personaCount || 0)} 位联系人</small></span><i aria-hidden="true">${String(group.id) === activeContactGroupId ? '✓' : '›'}</i></button>`).join('') : '<p class="muted">还没有可用分组。</p>';
    dialog.innerHTML = `<section class="detail-sheet group-picker"><header><div><small>CONTACT GROUP</small><h2>选择分组</h2></div><button class="close-dialog" id="close-group-picker" aria-label="关闭">×</button></header><div class="detail-scroll"><div class="group-picker-list">${groupItems}</div><button class="new-persona" id="group-picker-create">＋ 创建分组</button></div></section>`;
    showDialog(dialog, trigger);
    $('#close-group-picker').onclick = () => closeDialog(dialog);
    $('#group-picker-create').onclick = () => { closeDialog(dialog); openGroupWizard(); };
    dialog.querySelectorAll('[data-contact-group]').forEach(button => button.onclick = () => {
        activeContactGroupId = button.dataset.contactGroup;
        localStorage.setItem('companion-active-group', activeContactGroupId);
        closeDialog(dialog);
        renderContacts();
    });
}

function openPersonaPicker() {
    const dialog = $('#persona-dialog');
    dialog.innerHTML = `<section class="detail-sheet persona-picker"><header><div><small>COMPANIONS</small><h2>切换陪伴者</h2></div><button class="close-dialog" id="close-persona-picker" aria-label="关闭">×</button></header><div class="detail-scroll"><div class="persona-list picker-list">${appState.personas.length ? appState.personas.map(persona => `<button class="persona-row ${persona.id === activePersonaId ? 'selected' : ''}" data-picker-persona="${esc(persona.id)}">${avatar(persona)}<span class="persona-copy"><b>${esc(persona.name)}</b><small>${esc(persona.currentSituation || persona.role)}</small></span>${persona.unreadCount ? `<em>${persona.unreadCount > 99 ? '99+' : persona.unreadCount}</em>` : ''}</button>`).join('') : '<p class="muted">还没有陪伴者</p>'}</div><button class="new-persona" id="picker-create">＋ 创建一个陪伴者</button></div></section>`;
    showDialog(dialog);
    $('#close-persona-picker').onclick = () => closeDialog(dialog);
    $('#picker-create').onclick = () => { closeDialog(dialog); openPersonaWizard(); };
    dialog.querySelectorAll('[data-picker-persona]').forEach(button => button.onclick = async () => {
        closeDialog(dialog);
        currentView = 'chat';
        await selectPersona(button.dataset.pickerPersona);
    });
}

async function openPersonaWizard() {
    const dialog = $('#persona-dialog');
    dialog.innerHTML = `<section class="persona-wizard"><header><div><small>PERSONA INTERVIEW</small><h2>认识一下她</h2></div><button type="button" class="close-dialog" id="close-interview" aria-label="关闭">×</button></header><div class="wizard-body"><p class="wizard-intro">正在准备几个真正会影响她日常生活的问题。</p></div></section>`;
    showDialog(dialog);
    $('#close-interview').onclick = () => closeDialog(dialog);
    try {
        const interview = await api('/api/companion/interviews', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: '{}'});
        if (dialog.open) renderInterview(dialog, interview);
    } catch (error) {
        closeDialog(dialog);
        window.alert(error.message);
    }
}

function renderInterview(dialog, interview) {
    const question = interview.question;
    if (!question) return renderInterviewPreview(dialog, interview);
    const input = question.type === 'textarea'
        ? `<textarea name="answer" rows="5" maxlength="${question.maxLength}" ${question.required ? 'required' : ''} placeholder="${esc(question.placeholder)}"></textarea>`
        : `<input name="answer" type="text" maxlength="${question.maxLength}" ${question.required ? 'required' : ''} placeholder="${esc(question.placeholder)}">`;
    dialog.innerHTML = `<form class="persona-wizard" id="interview-form"><header><div><small>PERSONA INTERVIEW</small><h2>认识一下她</h2></div><button type="button" class="close-dialog" id="close-interview" aria-label="关闭">×</button></header><div class="wizard-body"><p class="wizard-intro">只问还不知道、且会影响她生活设定的信息。${question.required ? '' : ' 这题可以跳过，系统会明确标记为 AI 推断。'}</p><label>${esc(question.label)}${input}</label></div><footer class="wizard-footer">${question.required ? '' : '<button type="button" class="quiet" id="skip-interview-question">跳过</button>'}<button class="primary">继续</button></footer></form>`;
    $('#close-interview').onclick = () => closeDialog(dialog);
    const submit = async skip => {
        const form = $('#interview-form');
        if (!skip && !form.reportValidity()) return;
        const answer = skip ? '' : new FormData(form).get('answer');
        const button = form.querySelector('.primary');
        button.disabled = true;
        try {
            const next = await api(`/api/companion/interviews/${encodeURIComponent(interview.id)}/answers`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({key: question.key, answer})});
            renderInterview(dialog, next);
        } catch (error) {
            button.disabled = false;
            window.alert(error.message);
        }
    };
    $('#interview-form').onsubmit = event => { event.preventDefault(); submit(false); };
    $('#skip-interview-question')?.addEventListener('click', () => submit(true));
}

function renderInterviewPreview(dialog, interview) {
    const preview = interview.preview;
    const inferred = preview.inferredFields.length ? preview.inferredFields.join('、') : '无';
    dialog.innerHTML = `<form class="persona-wizard" id="interview-preview-form"><header><div><small>PERSONA PREVIEW</small><h2>确认她的生活设定</h2></div><button type="button" class="close-dialog" id="close-interview" aria-label="关闭">×</button></header><div class="wizard-body"><p class="wizard-intro">以下内容会成为她稳定的身份与生活蓝图。你提供的信息和 AI 推断会一直保留为不同来源。</p><div class="preview-card"><b>AI 推断：${esc(inferred)}</b><p><strong>日常作息</strong>${esc(preview.blueprint.routine.map(item => item.label).join(' · '))}</p></div><label>基础人格<textarea name="foundation" rows="5" maxlength="3000" required>${esc(preview.foundation)}</textarea></label><label>兴趣<input name="interests" maxlength="180" value="${esc(preview.blueprint.interests.join('、'))}"></label><label>外观和日常穿衣印象<input name="visualBaseline" maxlength="240" value="${esc(preview.blueprint.visualBaseline)}"></label><label>身边最早出现的人<input name="supportingCast" maxlength="180" value="${esc(preview.blueprint.supportingCast.map(item => item.name).join('、'))}"></label></div><footer class="wizard-footer"><button type="button" class="quiet" id="back-to-interview">返回修改</button><button class="primary">确认并创建</button></footer></form>`;
    $('#close-interview').onclick = () => closeDialog(dialog);
    $('#back-to-interview').onclick = () => window.alert('已回答的信息会保留。关闭后重新开始可调整访谈答案。');
    $('#interview-preview-form').onsubmit = async event => {
        event.preventDefault();
        const form = event.currentTarget;
        if (!form.reportValidity()) return;
        const button = form.querySelector('.primary');
        button.disabled = true;
        try {
            const persona = await api(`/api/companion/interviews/${encodeURIComponent(interview.id)}/activate`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({overrides: Object.fromEntries(new FormData(form))})});
            dialog.close('submitted');
            await loadBootstrap();
            await selectPersona(persona.id);
        } catch (error) {
            button.disabled = false;
            window.alert(error.message);
        }
    };
}

async function openPersonaDetail(personaId) {
    try {
        activeDetail = await api(`/api/companion/personas/${personaId}`);
        const dialog = $('#persona-dialog');
        const {persona, state, schedule, supportingCharacters, memories, evolutions, foundationSummary, blueprint} = activeDetail;
        const stateSource = state?.source?.kind === 'schedule' ? '来自已确认安排'
            : state?.source?.kind === 'daily_plan' ? '来自当天计划'
                : state?.source?.kind === 'daily_plan_baseline' ? '来自当天计划基线'
                    : state?.source?.kind === 'routine' ? '来自日常作息'
                        : state?.source?.kind === 'recovery' ? '服务恢复后同步'
                            : state?.source ? '来自已记录生活事件' : '正在同步状态来源';
        const inferredFields = Object.entries(blueprint?.provenance || {}).filter(([, source]) => source === 'inferred').map(([field]) => ({routine: '作息', interests: '兴趣', visualBaseline: '外观印象', supportingCast: '初始社交圈', foundation: '基础人格'})[field] || field);
        const foundationLines = [foundationSummary?.identity, foundationSummary?.routine?.length ? `日常节奏：${foundationSummary.routine.join(' · ')}` : '', foundationSummary?.interests?.length ? `喜欢：${foundationSummary.interests.join('、')}` : ''].filter(Boolean);
        const groups = contactGroups();
        const defaultGroup = groups.find(group => group.isDefault) || groups[0];
        const currentGroupId = String(persona.groupId || defaultGroup?.id || '');
        const groupOptions = groups.map(group => `<option value="${esc(group.id)}" ${String(group.id) === currentGroupId ? 'selected' : ''}>${esc(group.name)}${Number(group.personaCount || 0) ? ` (${Number(group.personaCount)})` : ''}</option>`).join('');
        dialog.innerHTML = `<section class="detail-sheet"><header><div>${avatar(persona)}<span><small>PERSONA</small><h2>${esc(persona.name)}</h2><p>${esc(persona.role)}</p></span></div><button class="close-dialog" id="close-detail" aria-label="关闭">×</button></header><div class="detail-scroll"><section><h3>现在</h3><p class="state-line">${esc(state?.situation || '正在过自己的日常')}<small>${esc(state?.mood || '平静')}</small></p><p class="state-source">${esc(state?.scene || state?.room || '日常场景')}${state?.location ? ` · ${esc(state.location)}` : ''}</p><p class="state-source">${esc(stateSource)}${state?.source?.rationale ? ` · ${esc(state.source.rationale)}` : ''}</p>${Object.keys(state?.appearance || {}).length ? `<p class="state-source">当前外观：${esc(Object.values(state.appearance).join(' · '))}</p>` : ''}</section><section><h3>近期安排</h3>${schedule.length ? `<ul class="schedule-list">${schedule.map(item => `<li><b>${esc(item.title)}</b><small>${formatTime(item.startsAt)}</small><button class="schedule-reschedule" data-reschedule="${esc(item.id)}" aria-label="改期">↻</button><button class="schedule-cancel" data-schedule="${esc(item.id)}" aria-label="取消这项安排">×</button></li>`).join('')}</ul>` : '<p class="muted">暂无公开的近期安排</p>'}</section><section><h3>生活设定</h3><p class="detail-text">${foundationLines.map(esc).join('<br>')}</p>${inferredFields.length ? `<p class="state-source">AI 推断：${esc(inferredFields.join('、'))}</p>` : '<p class="state-source">所有初始设定均由你提供</p>'}<button class="quiet" id="edit-foundation">修订基础人格</button>${activeDetail.foundationRevisions.length > 1 ? `<ul class="foundation-list">${activeDetail.foundationRevisions.map((revision, index) => `<li><span><b>版本 ${revision.version}</b><small>${esc(revision.reason)} · ${formatTime(revision.createdAt)}</small></span>${index ? `<button class="quiet" data-restore-foundation="${esc(revision.id)}">恢复此版本</button>` : '<small>当前版本</small>'}</li>`).join('')}</ul>` : ''}</section><section><h3>她认识的你</h3>${memories.length ? `<ul class="memory-list">${memories.map(memory => `<li><span>${esc(memory.key)}</span><p>${esc(memory.value)}</p><button data-memory="${esc(memory.id)}" aria-label="删除这条记忆">×</button></li>`).join('')}</ul>` : '<p class="muted">她还在慢慢了解你。</p>'}</section><section><h3>关系变化</h3>${evolutions.length ? `<ul class="evolution-list">${evolutions.map((item, index) => `<li><b>${esc(item.reason)}</b><small>${formatTime(item.createdAt)}</small><span>${esc(item.evidenceSummary)}</span>${(item.changes || []).map(change => `<p class="evolution-diff">${esc(change.field)}：${esc(change.before)} → ${esc(change.after)}</p>`).join('')}${item.status === 'applied' && index === 0 ? `<button data-rollback="${esc(item.id)}" class="quiet">撤销这次变化</button>` : `<span>${item.status === 'reverted' ? '已撤销' : '已归档'}</span>`}</li>`).join('')}</ul>` : '<p class="muted">还没有需要保留的关系变化。</p>'}</section><section><h3>身边的人</h3><p class="supporting-names">${supportingCharacters.length ? supportingCharacters.map(item => esc(item.name)).join(' · ') : '会在生活里慢慢认识新朋友'}</p></section><section><h3>管理</h3><button class="quiet" id="persona-moments">查看她的动态</button><button class="quiet" id="screen-persona">${persona.screened ? '取消屏蔽' : '屏蔽动态与主动私聊'}</button><button class="quiet" id="hidden-activities">管理已隐藏动态</button><button class="quiet danger" id="delete-persona">删除此人格</button></section><section class="persona-group-setting"><h3>所属分组</h3><p class="state-source">在这里设置 ${esc(persona.name)} 出现在哪个联系人分组中。</p><div class="persona-group-controls"><select id="persona-group-select" aria-label="选择 ${esc(persona.name)} 所属分组">${groupOptions}</select><button type="button" class="quiet" id="persona-group-save">保存</button></div></section></div></section>`;
        showDialog(dialog);
        $('#close-detail').onclick = () => closeDialog(dialog);
        $('#screen-persona').onclick = () => toggleScreen(persona);
        $('#edit-foundation').onclick = () => reviseFoundation(persona);
        dialog.querySelectorAll('[data-memory]').forEach(button => button.onclick = () => deleteMemory(persona.id, button.dataset.memory));
        dialog.querySelectorAll('[data-rollback]').forEach(button => button.onclick = () => rollbackEvolution(persona.id, button.dataset.rollback));
        dialog.querySelectorAll('[data-restore-foundation]').forEach(button => button.onclick = () => restoreFoundation(persona.id, button.dataset.restoreFoundation));
        dialog.querySelectorAll('[data-schedule]').forEach(button => button.onclick = () => cancelSchedule(persona.id, button.dataset.schedule));
        dialog.querySelectorAll('[data-reschedule]').forEach(button => button.onclick = () => openRescheduleSchedule(persona.id, schedule.find(item => item.id === button.dataset.reschedule)));
        $('#persona-moments').onclick = async () => {
            closeDialog(dialog);
            currentView = 'activity';
            await loadActivities(persona.id);
        };
        $('#hidden-activities').onclick = () => openHiddenActivities(persona.id);
        $('#delete-persona').onclick = () => deletePersona(persona);
        $('#persona-group-save').onclick = async () => {
            const select = $('#persona-group-select');
            const button = $('#persona-group-save');
            const nextGroupId = String(select.value || '').trim();
            if (!nextGroupId || nextGroupId === currentGroupId) return;
            button.disabled = true;
            try {
                await api(`/api/companion/personas/${encodeURIComponent(persona.id)}/group`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({groupId: nextGroupId})});
                await loadBootstrap();
                if (currentView === 'contacts') renderContacts();
                await openPersonaDetail(persona.id);
                renderSidebar();
            } catch (error) {
                button.disabled = false;
                window.alert(error.message);
            }
        };
    } catch (error) { window.alert(error.message); }
}

async function cancelSchedule(personaId, scheduleId) {
    if (!window.confirm('取消这项安排？她会保留一条可解释的取消记录。')) return;
    try {
        await api(`/api/companion/personas/${personaId}/schedule/${scheduleId}/cancel`, {method: 'POST'});
        await openPersonaDetail(personaId);
    } catch (error) { window.alert(error.message); }
}

function toDateTimeLocal(value) {
    const date = new Date(value);
    const offset = date.getTimezoneOffset() * 60_000;
    return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function openRescheduleSchedule(personaId, schedule) {
    if (!schedule) return;
    const dialog = $('#inspector-dialog');
    dialog.innerHTML = `<form class="inspector" id="reschedule-form"><header><div><small>RESCHEDULE</small><h2>调整安排</h2></div><button class="close-dialog" type="button" id="close-reschedule" aria-label="关闭">×</button></header><div><label class="detail-label">安排名称<input name="title" maxlength="120" value="${esc(schedule.title)}"></label><label class="detail-label">开始时间<input name="startsAt" type="datetime-local" required value="${esc(toDateTimeLocal(schedule.startsAt))}"></label><label class="detail-label">结束时间<input name="endsAt" type="datetime-local" required value="${esc(toDateTimeLocal(schedule.endsAt || new Date(new Date(schedule.startsAt).getTime() + 90 * 60 * 1000)))}"></label><label class="detail-label">地点或场景<input name="scene" maxlength="120" value="${esc(schedule.details?.scene || '')}"></label></div><footer class="wizard-footer"><button type="button" class="quiet" id="cancel-reschedule">取消</button><button class="primary">确认改期</button></footer></form>`;
    showDialog(dialog);
    $('#close-reschedule').onclick = () => closeDialog(dialog);
    $('#cancel-reschedule').onclick = () => closeDialog(dialog);
    $('#reschedule-form').onsubmit = async event => {
        event.preventDefault();
        const values = Object.fromEntries(new FormData(event.currentTarget));
        values.startsAt = new Date(values.startsAt).toISOString();
        values.endsAt = new Date(values.endsAt).toISOString();
        try {
            await api(`/api/companion/personas/${encodeURIComponent(personaId)}/schedule/${encodeURIComponent(schedule.id)}`, {method: 'PATCH', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(values)});
            closeDialog(dialog);
            await openPersonaDetail(personaId);
        } catch (error) { window.alert(error.message); }
    };
}

async function toggleScreen(persona) {
    try {
        await api(`/api/companion/personas/${persona.id}/screen`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({screened: !persona.screened})});
        await loadBootstrap();
        await openPersonaDetail(persona.id);
        renderSidebar();
    } catch (error) { window.alert(error.message); }
}

async function deleteMemory(personaId, memoryId) {
    if (!window.confirm('删除这条长期记忆？')) return;
    try { await api(`/api/companion/personas/${personaId}/memories/${memoryId}`, {method: 'DELETE'}); await openPersonaDetail(personaId); } catch (error) { window.alert(error.message); }
}

async function deletePersona(persona) {
    if (!window.confirm(`确定永久删除“${persona.name}”吗？这会删除她的对话、记忆、动态和未完成任务，且无法恢复。`)) return;
    try {
        await api(`/api/companion/personas/${encodeURIComponent(persona.id)}`, {method: 'DELETE'});
        if (activePersonaId === persona.id) {
            activePersonaId = '';
            localStorage.removeItem('companion-active-persona');
            activeMessages = [];
        }
        closeDialog($('#persona-dialog'));
        await loadBootstrap();
        if (activePersonaId) await selectPersona(activePersonaId);
        else render();
    } catch (error) { window.alert(error.message); }
}

async function rollbackEvolution(personaId, evolutionId) {
    if (!window.confirm('撤销这次关系层变化？基础人格和已有记忆不会被删除。')) return;
    try {
        await api(`/api/companion/personas/${personaId}/evolutions/${evolutionId}/rollback`, {method: 'POST'});
        await openPersonaDetail(personaId);
    } catch (error) { window.alert(error.message); }
}

async function openHiddenActivities(personaId) {
    try {
        if (hiddenActivityPersonaId !== personaId) {
            const data = await api(`/api/companion/activities?personaId=${encodeURIComponent(personaId)}&visibility=hidden`);
            hiddenActivityPersonaId = personaId;
            hiddenActivityItems = data.items;
            hiddenActivityNextCursor = data.nextCursor;
            hiddenActivityLoadingMore = false;
        }
        renderHiddenActivities();
    } catch (error) { window.alert(error.message); }
}

function renderHiddenActivities() {
    const personaId = hiddenActivityPersonaId;
    if (!personaId) return;
    const dialog = $('#inspector-dialog');
    const loadMore = hiddenActivityNextCursor ? `<div class="load-more"><button class="quiet" id="load-more-hidden" ${hiddenActivityLoadingMore ? 'disabled' : ''}>${hiddenActivityLoadingMore ? '正在加载...' : '加载更多'}</button></div>` : '';
    dialog.innerHTML = `<section class="inspector"><header><div><small>HIDDEN ACTIVITIES</small><h2>已隐藏动态</h2></div><button class="close-dialog" id="close-hidden" aria-label="关闭">×</button></header><div class="hidden-list">${hiddenActivityItems.length ? hiddenActivityItems.map(item => `<article><p>${esc(item.content)}</p><small>${formatTime(item.createdAt)}</small><button class="quiet" data-restore="${esc(item.id)}">恢复</button></article>`).join('') : '<p class="muted">没有已隐藏动态。</p>'}${loadMore}</div></section>`;
    showDialog(dialog);
    $('#close-hidden').onclick = () => closeDialog(dialog);
    $('#load-more-hidden')?.addEventListener('click', () => loadMoreHiddenActivities());
    dialog.querySelectorAll('[data-restore]').forEach(button => button.onclick = async () => {
        try {
            await api(`/api/companion/activities/${button.dataset.restore}/hide`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({hidden: false})});
            hiddenActivityItems = hiddenActivityItems.filter(item => item.id !== button.dataset.restore);
            renderHiddenActivities();
            if (currentView === 'activity') await refreshActivities();
        } catch (error) { window.alert(error.message); }
    });
}

async function loadMoreHiddenActivities() {
    if (!hiddenActivityNextCursor || hiddenActivityLoadingMore || !hiddenActivityPersonaId) return;
    hiddenActivityLoadingMore = true;
    renderHiddenActivities();
    try {
        const data = await api(`/api/companion/activities?personaId=${encodeURIComponent(hiddenActivityPersonaId)}&visibility=hidden&cursor=${encodeURIComponent(hiddenActivityNextCursor)}`);
        const seen = new Set(hiddenActivityItems.map(item => item.id));
        hiddenActivityItems = [...hiddenActivityItems, ...data.items.filter(item => !seen.has(item.id))];
        hiddenActivityNextCursor = data.nextCursor;
    } catch (error) {
        window.alert(error.message);
    } finally {
        hiddenActivityLoadingMore = false;
        renderHiddenActivities();
    }
}

async function reviseFoundation(persona) {
    let foundation;
    try {
        foundation = (await api(`/api/companion/personas/${encodeURIComponent(persona.id)}/foundation/draft`)).foundation;
    } catch (error) {
        window.alert(error.message);
        return;
    }
    foundation = window.prompt('写下新的基础人格。系统会保留前一个版本。', foundation);
    if (!foundation?.trim()) return;
    try {
        await api(`/api/companion/personas/${persona.id}/foundation`, {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({foundation, reason: '用户在详情页修订'})});
        await openPersonaDetail(persona.id);
    } catch (error) { window.alert(error.message); }
}

async function restoreFoundation(personaId, revisionId) {
    if (!window.confirm('恢复此基础人格版本？系统会保留当前版本，并把恢复结果记为一个新版本。')) return;
    try {
        await api(`/api/companion/personas/${personaId}/foundation-revisions/${revisionId}/restore`, {method: 'POST'});
        await openPersonaDetail(personaId);
        await loadBootstrap();
        renderSidebar();
    } catch (error) { window.alert(error.message); }
}

const lifecycleEventLabels = {routine: '日常推进', class: '上学/课程', study: '学习', shopping: '逛街/购物', social: '社交活动', mild_setback: '轻度挫折', rest: '休息', schedule: '安排进行中', schedule_cancelled: '安排已取消', schedule_rescheduled: '安排已改期', recovery: '服务恢复'};
const lifecycleJobLabels = {activity_image: '动态图片生成', activity_video: '动态视频生成', chat_image: '聊天图片生成', chat_video: '聊天视频生成', activity_media_poll: '等待动态媒体', chat_media_poll: '等待聊天媒体', relationship_evolution: '关系学习', proactive_message: '主动消息'};
const lifecycleStatusLabels = {queued: '等待处理', leased: '处理中', complete: '已完成', failed: '已失败'};
const mediaProgressStageLabels = {queued: '等待提交', waiting_provider: '等待 provider 输出', preparing: '正在准备', generating: '正在生成', validating_output: '校验输出', complete: '已完成', failed: '执行失败'};

function inspectorText(value, fallback = '') {
    if (typeof value === 'string') return value.trim() || fallback;
    if (typeof value === 'number' || typeof value === 'boolean') return String(value);
    return fallback;
}

function inspectorDebugText(value, fallback = '未提供') {
    const text = inspectorText(value);
    if (text) return text.slice(0, 2_000);
    if (!value || typeof value !== 'object') return fallback;
    try { return JSON.stringify(value, null, 2).slice(0, 2_000) || fallback; } catch { return fallback; }
}

function inspectorFiniteNumber(value) {
    if (typeof value === 'number') return Number.isFinite(value) ? value : null;
    if (typeof value !== 'string' || !value.trim()) return null;
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
}

function inspectorFirstText(...values) {
    for (const value of values) {
        const text = inspectorText(value);
        if (text) return text;
    }
    return '';
}

function formatInspectorElapsed(elapsedMs) {
    const seconds = Math.max(0, Math.floor(elapsedMs / 1_000));
    const hours = Math.floor(seconds / 3_600);
    const minutes = Math.floor((seconds % 3_600) / 60);
    const remainder = seconds % 60;
    if (hours) return `${hours}小时${String(minutes).padStart(2, '0')}分${String(remainder).padStart(2, '0')}秒`;
    if (minutes) return `${minutes}分${String(remainder).padStart(2, '0')}秒`;
    return `${remainder}秒`;
}

function formatInspectorTime(value) {
    const timestamp = Date.parse(inspectorText(value));
    return Number.isFinite(timestamp) ? formatTime(timestamp) : '';
}

function mediaJobProgress(job) {
    const progress = job?.progress && typeof job.progress === 'object' && !Array.isArray(job.progress) ? job.progress : {};
    const stage = inspectorFirstText(progress.stage);
    const phase = inspectorFirstText(progress.stageLabel) || mediaProgressStageLabels[stage] || stage || lifecycleStatusLabels[inspectorText(job?.status)] || '进度未知';
    const attempt = [progress.attempt, job?.attempt, job?.attempts].map(inspectorFiniteNumber).find(value => value !== null && value > 0);
    const directElapsedMs = inspectorFiniteNumber(progress.elapsedMs);
    const startedAt = Date.parse(inspectorText(progress.startedAt));
    const elapsedMs = directElapsedMs !== null && directElapsedMs >= 0
        ? directElapsedMs
        : Number.isFinite(startedAt) ? Math.max(0, Date.now() - startedAt) : null;
    const percentValue = inspectorFiniteNumber(progress.percent);
    const percent = percentValue === null ? null : Math.max(0, Math.min(100, percentValue));
    return {
        phase,
        attemptLabel: attempt ? `第 ${Math.floor(attempt)} 次` : inspectorText(job?.status) === 'queued' ? '尚未开始' : '未提供',
        elapsedLabel: elapsedMs === null ? inspectorText(job?.status) === 'queued' ? '尚未开始' : '未提供' : formatInspectorElapsed(elapsedMs),
        percentLabel: percent === null ? 'provider 未报告百分比' : `${Math.round(percent * 10) / 10}%`,
        latestOutput: inspectorFirstText(progress.latestOutput, progress.output),
        latestStream: inspectorFirstText(progress.latestStream, progress.outputStream)
    };
}

function renderMediaJobCard(rawJob, index, expandedDiagnosticIds) {
    const job = rawJob && typeof rawJob === 'object' && !Array.isArray(rawJob) ? rawJob : {};
    const jobId = inspectorFirstText(job.id) || `media-job-${index}`;
    const progress = mediaJobProgress(job);
    const kind = inspectorFirstText(job.kind) || '媒体';
    const provider = inspectorFirstText(job.provider) || '未标识 provider';
    const statusValue = inspectorText(job.status);
    const status = lifecycleStatusLabels[statusValue] || statusValue || '未知状态';
    const finalPrompt = inspectorFirstText(job.finalPrompt, job.finalProviderPrompt, job.promptSummary, job.prompt);
    const trigger = inspectorDebugText(job.trigger, '未提供');
    const envelope = inspectorDebugText(job.envelope, '未提供');
    const personaConcept = inspectorDebugText(job.personaConcept, '尚未生成');
    const promptTemplate = inspectorDebugText(job.promptTemplate, '尚未生成');
    const workflow = inspectorDebugText(job.workflowSummary ?? job.workflow, '未提供');
    const error = inspectorFirstText(job.error);
    const createdAt = formatInspectorTime(job.createdAt);
    const opened = expandedDiagnosticIds.has(jobId) ? ' open' : '';
    return `<article class="media-job-card" data-media-job-id="${esc(jobId)}"><header class="media-job-card-header"><div><p>${esc(kind)} · ${esc(provider)}</p>${createdAt ? `<small>${esc(createdAt)}</small>` : ''}</div><span class="media-job-status">${esc(status)}</span></header><section class="media-job-prompt"><b>最终 provider 提示词</b><p>${esc(finalPrompt || '最终提示词尚未持久化')}</p></section><dl class="media-job-meta"><div><dt>阶段</dt><dd>${esc(progress.phase)}</dd></div><div><dt>尝试</dt><dd>${esc(progress.attemptLabel)}</dd></div><div><dt>耗时</dt><dd>${esc(progress.elapsedLabel)}</dd></div><div><dt>进度</dt><dd>${esc(progress.percentLabel)}</dd></div></dl><section class="media-job-output"><b>最新输出</b><p>${esc(progress.latestOutput || '暂无本地输出')}</p>${progress.latestStream ? `<small>来源：${esc(progress.latestStream)}</small>` : ''}</section>${error ? `<p class="media-job-error"><b>失败说明</b>${esc(error)}</p>` : ''}<details class="media-job-diagnostics" data-media-diagnostic-id="${esc(jobId)}"${opened}><summary>媒体概念与模板诊断</summary><div class="media-job-diagnostics-body"><div><b>触发</b><pre>${esc(trigger)}</pre></div><div><b>服务器事实信封</b><pre>${esc(envelope)}</pre></div><div><b>AI 人格媒体概念</b><pre>${esc(personaConcept)}</pre></div><div><b>生图大师固定模板</b><pre>${esc(promptTemplate)}</pre></div><div><b>工作流摘要</b><pre>${esc(workflow)}</pre></div></div></details></article>`;
}

function renderInspectorMediaJobs(mediaJobs, expandedDiagnosticIds = new Set()) {
    const jobs = Array.isArray(mediaJobs) ? mediaJobs : [];
    return `<h3>媒体任务</h3><div class="media-job-list">${jobs.length ? jobs.map((job, index) => renderMediaJobCard(job, index, expandedDiagnosticIds)).join('') : '<p class="media-job-empty">暂无媒体作业。</p>'}</div>`;
}

function updateInspectorMediaJobs(dialog, mediaJobs) {
    const region = dialog.querySelector('#inspector-media-jobs');
    if (!region) return;
    const expandedDiagnosticIds = new Set([...region.querySelectorAll('details[data-media-diagnostic-id][open]')].map(details => details.dataset.mediaDiagnosticId));
    region.innerHTML = renderInspectorMediaJobs(mediaJobs, expandedDiagnosticIds);
}

function stopInspectorMediaRefresh() {
    if (inspectorMediaRefresh?.timer !== null && inspectorMediaRefresh?.timer !== undefined) window.clearInterval(inspectorMediaRefresh.timer);
    inspectorMediaRefresh = null;
}

async function refreshInspectorMediaJobs(refresh) {
    if (inspectorMediaRefresh !== refresh || refresh.inFlight) return;
    const region = refresh.dialog.querySelector('#inspector-media-jobs');
    if (!refresh.dialog.open || !region) return stopInspectorMediaRefresh();
    refresh.inFlight = true;
    try {
        const debug = await api(`/api/companion/personas/${encodeURIComponent(refresh.personaId)}/debug-context`);
        if (inspectorMediaRefresh === refresh && refresh.dialog.open) updateInspectorMediaJobs(refresh.dialog, debug?.mediaJobs);
    } catch {
        // Keep the last successful snapshot visible; a polling error should not interrupt inspector form work.
    } finally {
        refresh.inFlight = false;
    }
}

function startInspectorMediaRefresh(personaId, dialog) {
    stopInspectorMediaRefresh();
    const refresh = {personaId, dialog, timer: null, inFlight: false};
    inspectorMediaRefresh = refresh;
    dialog.addEventListener('close', () => {
        if (inspectorMediaRefresh === refresh) stopInspectorMediaRefresh();
    }, {once: true});
    refresh.timer = window.setInterval(() => refreshInspectorMediaJobs(refresh), 1_000);
}

async function openInspector(personaId) {
    try {
        const [detail, debug] = await Promise.all([
            api(`/api/companion/personas/${encodeURIComponent(personaId)}/lifecycle`),
            api(`/api/companion/personas/${encodeURIComponent(personaId)}/debug-context`)
        ]);
        const dialog = $('#inspector-dialog');
        const layerItems = Object.entries(debug.layers || {}).map(([name, value]) => `<li><b>${esc({identity: '身份层', conversation: '对话层', life: '生活层', provider: '提供方层'}[name] || name)}</b><pre>${esc(value)}</pre></li>`).join('') || '<li>暂无可检查的组合层。</li>';
        const requestItems = (debug.recentRequests || []).map(item => `<li><b>${esc(item.status)}</b><span>${esc(item.promptSummary || item.responseSummary || '')}</span><small>${formatTime(item.createdAt)} ${esc(item.error || '')}</small></li>`).join('') || '<li>暂无近期聊天记录。</li>';
        const timelineItems = (detail.timeline || []).map(slot => `<li><b>${esc(slot.kind)} · ${esc(slot.status)}</b><span>${esc(formatTime(slot.startsAt))}–${esc(formatTime(slot.endsAt))} · ${esc(slot.source)}</span><small>${esc(slot.outcome?.reason || '')}</small></li>`).join('') || '<li>暂无时间线槽位</li>';
        const decisionItems = (detail.decisions || []).map(item => `<li><b>${esc(item.type)} · ${esc(item.status)}</b><span>${esc(item.candidate?.title || item.candidate?.kind || '')}</span><small>${esc(item.rationale?.reason || '')}</small></li>`).join('') || '<li>暂无编排决策</li>';
        const deferredItems = (detail.deferredBatches || []).map(item => `<li><b>${esc(item.status)}</b><span>待投递消息 ${esc(item.messageCount)} 条</span><small>${formatTime(item.deliverAt)}</small></li>`).join('') || '<li>暂无延迟聊天批次</li>';
        dialog.innerHTML = `<section class="inspector"><header><div><small>LOCAL DEVELOPMENT INSPECTOR</small><h2>生命周期与调试</h2></div><button class="close-dialog" id="close-inspector" aria-label="关闭">×</button></header><div><p class="inspector-warning">仅限本地开发：测试媒体请求会创建真实的耐久作业。</p><section class="h3-preflight"><h3>h3 当前配置</h3><p>此检查只验证当前服务的文件系统配置，并启动一次 <code>h3 --help</code>；不会创建媒体作业或资产。</p><button type="button" class="quiet" id="h3-preflight-button">测试当前 h3 配置</button><p class="h3-preflight-result" id="h3-preflight-result" role="status" aria-live="polite"></p></section><p><b>当前状态：</b>${esc(detail.state?.situation || '')} · ${esc(detail.state?.mood || '')}</p><p><b>下次评估：</b>${formatTime(detail.nextEvaluationAt)} · ${esc(detail.timezone)}</p><form id="simulate-form" class="simulate-form"><label>事件类型<select name="kind" aria-label="模拟事件类型"><option value="routine">日常推进</option><option value="class">上学/课程</option><option value="shopping">逛街/购物</option><option value="social">社交活动</option><option value="mild_setback">轻度挫折</option></select></label><label>状态说明<input name="situation" aria-label="当前状态说明" placeholder="当前状态（可选）"></label><label class="visual-toggle"><input type="checkbox" name="visual"> 为该动态生成活动图片</label><button class="primary">模拟允许事件</button></form><form id="test-media-form" class="test-media-form"><label>测试媒体<select name="kind" aria-label="测试媒体类型"><option value="image">图片</option><option value="video">视频</option></select></label><label>测试画面说明<input name="prompt" maxlength="500" placeholder="可选；会结合当前人格状态"></label><button class="quiet">创建测试媒体作业</button></form><h3>今日时间线</h3><ul class="event-list">${timelineItems}</ul><h3>编排决策</h3><ul class="event-list">${decisionItems}</ul><h3>睡眠延迟批次</h3><ul class="event-list">${deferredItems}</ul><h3>提示词组合层</h3><ul class="debug-list">${layerItems}</ul><h3>近期聊天请求/响应</h3><ul class="event-list">${requestItems}</ul><section class="inspector-media-region" id="inspector-media-jobs">${renderInspectorMediaJobs(debug.mediaJobs)}</section><h3>最近事件</h3><ul class="event-list">${detail.events.map(event => `<li><b>${esc(lifecycleEventLabels[event.type] || event.type)}</b><span>${esc(event.payload.situation || '')}</span><small>${formatTime(event.occurredAt)}</small></li>`).join('') || '<li>暂无事件</li>'}</ul><h3>作业</h3><ul class="event-list">${detail.jobs.map(job => `<li><b>${esc(lifecycleJobLabels[job.type] || job.type)}</b><span>${esc(lifecycleStatusLabels[job.status] || job.status)}</span><small>${esc(job.error || '')}</small></li>`).join('') || '<li>暂无作业</li>'}</ul></div></section>`;
        showDialog(dialog);
        $('#close-inspector').onclick = () => closeDialog(dialog);
        startInspectorMediaRefresh(personaId, dialog);
        $('#h3-preflight-button').onclick = async event => {
            const button = event.currentTarget;
            const result = $('#h3-preflight-result');
            button.disabled = true;
            result.textContent = '正在检查当前 h3 配置…';
            try {
                const preflight = await api('/api/companion/h3-preflight', {method: 'POST'});
                const checks = Object.entries(preflight.checks || {}).map(([name, check]) => `${{executable: '可执行文件', modelDir: '模型目录', outputDir: '输出目录'}[name] || name}：${check?.valid ? '通过' : check?.error || '失败'}`);
                const process = preflight.process?.error || (preflight.ok ? 'h3 --help 已成功启动。' : '检查失败。');
                result.textContent = `${preflight.ok ? '检查通过。' : '检查未通过。'} ${checks.join('；')} ${process}`;
            } catch (error) {
                result.textContent = error.message;
            } finally {
                button.disabled = false;
            }
        };
        $('#simulate-form').onsubmit = async event => {
            event.preventDefault();
            try {
                const values = Object.fromEntries(new FormData(event.currentTarget));
                values.visual = event.currentTarget.elements.visual.checked;
                await api(`/api/companion/personas/${personaId}/simulate`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({...values, publish: true})});
                dialog.close('submitted');
                await loadBootstrap();
                if (currentView === 'activity') await loadActivities(); else renderChat();
            } catch (error) { window.alert(error.message); }
        };
        $('#test-media-form').onsubmit = async event => {
            event.preventDefault();
            const submit = event.currentTarget.querySelector('button');
            submit.disabled = true;
            try {
                const response = await api(`/api/companion/personas/${encodeURIComponent(personaId)}/debug-media`, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(Object.fromEntries(new FormData(event.currentTarget)))});
                activeMessages.push(response.message);
                dialog.close('submitted');
                if (currentView === 'chat' && activePersonaId === personaId) renderChat();
            } catch (error) {
                submit.disabled = false;
                window.alert(error.message);
            }
        };
    } catch (error) { window.alert(error.message); }
}

function openSettings() {
    const dialog = $('#settings-dialog');
    const config = appState.settings;
    const providers = Array.isArray(config.mediaProviders) && config.mediaProviders.length
        ? config.mediaProviders
        : [{id: 'comfyui', label: 'ComfyUI', capabilities: ['image', 'video']}];
    const providerOptions = (kind, selected) => {
        const options = providers.filter(provider => Array.isArray(provider.capabilities) && provider.capabilities.includes(kind));
        if (!options.length) return '<option value="">暂无可用提供方</option>';
        return options.map(provider => `<option value="${esc(provider.id)}" ${provider.id === selected ? 'selected' : ''}>${esc(provider.label || provider.id)}</option>`).join('');
    };
    const h3 = config.h3Config && typeof config.h3Config === 'object' ? config.h3Config : config.h3Defaults && typeof config.h3Defaults === 'object' ? config.h3Defaults : config.h3 && typeof config.h3 === 'object' ? config.h3 : {};
    const h3Value = (key, fallback = '') => h3[key] ?? config[`h3${key[0].toUpperCase()}${key.slice(1)}`] ?? fallback;
    const h3Enabled = providers.some(provider => provider.id === 'h3' && Array.isArray(provider.capabilities) && provider.capabilities.includes('video'));
    const h3Summary = config.h3ConfigSummary && typeof config.h3ConfigSummary === 'object' ? config.h3ConfigSummary : {};
    const h3SummaryLabel = {executable: '可执行文件', modelDir: '模型目录', outputDir: '输出目录'};
    const h3SummaryHtml = Object.entries(h3SummaryLabel).map(([key, label]) => {
        const check = h3Summary[key] && typeof h3Summary[key] === 'object' ? h3Summary[key] : {};
        const detail = check.configured ? check.displayName || '已配置（路径不回显）' : '未配置';
        const status = check.valid ? '可用' : check.error || '未验证';
        return `<li><b>${esc(label)}</b><span>${esc(detail)}</span><small>${esc(status)}</small></li>`;
    }).join('');
    dialog.innerHTML = `<form id="settings-form" class="settings-sheet"><header><div><small>LOCAL CONFIGURATION</small><h2>模型与生成设置</h2></div><button type="button" class="close-dialog" id="close-settings" aria-label="关闭">×</button></header><div>
        <section class="settings-section"><h3>语言模型</h3><label>模型服务地址<input name="lmStudioUrl" value="${esc(config.lmStudioUrl || '')}"></label><label>API Key<input name="lmStudioApiKey" type="password" placeholder="${config.hasLmStudioApiKey ? '已配置，留空保持不变' : '可选'}"></label><label>模型 ID<input name="model" value="${esc(config.model || '')}"></label></section>
        <section class="settings-section"><h3>媒体提供方</h3><p class="settings-help">媒体任务由服务端执行，浏览器不会直接连接提供方。</p><label>图片提供方<select name="imageProvider">${providerOptions('image', config.imageProvider || 'comfyui')}</select></label><label>视频提供方<select name="videoProvider">${providerOptions('video', config.videoProvider || 'comfyui')}</select></label><div class="provider-list">${providers.length ? providers.map(provider => `<span class="provider-chip"><b>${esc(provider.label || provider.id)}</b><small>${esc(provider.id)} · ${Array.isArray(provider.capabilities) ? provider.capabilities.map(esc).join(' / ') : '未声明能力'}</small></span>`).join('') : '<p class="muted">暂无已注册的媒体提供方</p>'}</div></section>
        <section class="settings-section"><h3>调试与加载</h3><label class="settings-toggle"><input name="simplifiedMediaMode" type="checkbox" ${config.simplifiedMediaMode ? 'checked' : ''}><span><b>简化媒体模式</b><small>聊天窗口不加载图片或视频；仍可触发媒体任务，并在开发检查器中查看最终提示词和进度。</small></span></label></section>
        <section class="settings-section"><h3>ComfyUI（兼容模式）</h3><label>ComfyUI 地址<input name="comfyUrl" value="${esc(config.comfyUrl || '')}"></label><label>图片工作流 JSON<textarea name="imageWorkflow" rows="5">${esc(config.imageWorkflow || '')}</textarea></label><label>视频工作流 JSON<textarea name="videoWorkflow" rows="5">${esc(config.videoWorkflow || '')}</textarea></label></section>
        <section class="settings-section h3-settings"><h3>h3.c 视频配置</h3><p class="settings-help">仅在视频提供方选择 h3 时生效。路径和数值由服务端校验，命令通过参数数组启动。</p><p class="settings-help">当前服务配置只安全回显末段名称，完整路径不会发送到浏览器。</p><ul class="h3-config-summary">${h3SummaryHtml}</ul>${h3Enabled ? '' : '<p class="settings-help">当前服务端未注册 h3 提供方。</p>'}<div class="settings-grid"><label>可执行文件<input name="h3Executable" value="${esc(h3Value('executable'))}" placeholder="留空保持已保存配置"></label><label>模型/Profile 目录<input name="h3Profile" value="${esc(h3Value('profile'))}" placeholder="留空保持已保存配置"></label><label>输出目录<input name="h3OutputDir" value="${esc(h3Value('outputDir'))}" placeholder="留空保持已保存配置"></label><label>超时（毫秒）<input name="h3TimeoutMs" type="number" min="1000" max="3600000" step="1000" value="${esc(h3Value('timeoutMs', 600000))}"></label><label>宽度<input name="h3Width" type="number" min="1" max="8192" step="1" value="${esc(h3Value('width', 512))}"></label><label>高度<input name="h3Height" type="number" min="1" max="8192" step="1" value="${esc(h3Value('height', 512))}"></label><label>帧数<input name="h3Frames" type="number" min="1" max="100000" step="1" value="${esc(h3Value('frames', 24))}"></label><label>步数<input name="h3Steps" type="number" min="1" max="10000" step="1" value="${esc(h3Value('steps', 20))}"></label><label>Layers<input name="h3Layers" type="number" min="1" max="1000" step="1" value="${esc(h3Value('layers', 1))}"></label><label>Reuse<input name="h3Reuse" type="number" min="0" max="100000" step="1" value="${esc(h3Value('reuse', 2))}"></label><label class="settings-check"><input name="h3SsdStreaming" type="checkbox" ${h3Value('ssdStreaming') ? 'checked' : ''}> SSD streaming</label></div></section>
    </div><footer><button type="button" class="quiet" id="cancel-settings">取消</button><button type="submit" class="primary">保存</button></footer></form>`;
    showDialog(dialog);
    $('#close-settings').onclick = () => closeDialog(dialog);
    $('#cancel-settings').onclick = () => closeDialog(dialog);
    $('#settings-form').onsubmit = async event => {
        event.preventDefault();
        try {
            const form = event.currentTarget;
            const values = Object.fromEntries(new FormData(form));
            const numberOrUndefined = value => value === '' || value === undefined ? undefined : Number(value);
            const payload = {
                ...values,
                h3Width: numberOrUndefined(values.h3Width), h3Height: numberOrUndefined(values.h3Height), h3Frames: numberOrUndefined(values.h3Frames), h3Steps: numberOrUndefined(values.h3Steps), h3Layers: numberOrUndefined(values.h3Layers), h3Reuse: numberOrUndefined(values.h3Reuse), h3TimeoutMs: numberOrUndefined(values.h3TimeoutMs),
                h3SsdStreaming: form.elements.h3SsdStreaming.checked,
                simplifiedMediaMode: form.elements.simplifiedMediaMode.checked
            };
            payload.h3Defaults = {
                profile: payload.h3Profile, width: payload.h3Width, height: payload.h3Height, frames: payload.h3Frames, steps: payload.h3Steps, layers: payload.h3Layers,
                reuse: payload.h3Reuse, ssdStreaming: payload.h3SsdStreaming
            };
            for (const key of ['h3Executable', 'h3Profile', 'h3OutputDir']) if (!payload[key]) delete payload[key];
            appState.settings = await api('/api/companion/settings', {method: 'PUT', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(payload)});
            dialog.close('submitted');
            if (currentView === 'chat') renderChat({followLatest: false});
        } catch (error) { window.alert(error.message); }
    };
}

function bindStatic() {
    document.querySelectorAll('[data-view]').forEach(button => button.onclick = async () => {
        currentView = button.dataset.view;
        if (currentView === 'activity') await loadActivities(); else render();
    });
    $('#create-button').onclick = openPersonaWizard;
    $('#sidebar-create').onclick = openPersonaWizard;
    $('#settings-button').onclick = openSettings;
    $('#brand-button').onclick = async () => {
        currentView = 'chat';
        if (activePersonaId) await selectPersona(activePersonaId); else render();
    };
}

async function refreshQuietly() {
    const activeElement = document.activeElement;
    const editing = activeElement && (activeElement.matches('textarea, input, [contenteditable="true"]') || activeElement.isContentEditable);
    // Background refresh must never replace an in-progress composer.  Reading
    // the input also protects drafts that have not yet fired an input event.
    if (document.hidden || isSending || editing || $('#chat-input')?.value) return;
    await loadBootstrap();
    if (currentView === 'activity') {
        await refreshActivities();
    } else if (currentView === 'contacts') {
        renderContacts();
    } else {
        if (activePersonaId) {
            const messages = (await api(`/api/companion/conversations/${activePersonaId}`)).items;
            const changed = JSON.stringify(messages) !== JSON.stringify(activeMessages);
            activeMessages = messages;
            if (changed) renderChat();
            else renderSidebar();
        } else render();
    }
}

async function boot() {
    build();
    await loadBootstrap();
    if (activePersonaId) {
        const messages = await api(`/api/companion/conversations/${activePersonaId}`);
        activeMessages = messages.items;
    }
    if (currentView === 'activity') await loadActivities(); else render();
    // On mobile browsers, opening or dismissing the software keyboard can emit
    // window focus transitions.  Polling is sufficient for background updates;
    // a focus event must never rebuild the chat while a person is composing.
    setInterval(() => refreshQuietly().catch(() => {}), 15_000);
}

boot().catch(error => {
    document.body.innerHTML = `<main class="startup-error"><h1>启动失败</h1><p>${esc(error.message)}</p></main>`;
});
