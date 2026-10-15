// Fixture-only projection source. The browser client must not import this file.
function seed() {
  return {
    meta: { seedVersion: 4, seq: 100 },
    ui: { charId: 'c1', identity: '交流', notes: false },
    groups: { grp_daily: { id: 'grp_daily', name: '日常' }, grp_none: { id: 'grp_none', name: '未分组' } },
    characters: {
      c1: { id: 'c1', name: '小遥', avatar: 'assets/avatars/xiaoyao.svg', status: '正常', groupId: 'grp_daily', worldId: 'w1', bio: '22 岁，在海边小城做插画助理。喜欢逛书店、收集碎花布料，说话轻快爱用“呢”。', traits: ['温柔', '好奇', '有点迷糊'], defVersion: 'v7', harmIntercepts: 2, locationId: 'p_cafe', doing: '在街角咖啡馆画明信片草稿', doingActionId: 'a_draw', mood: { value: '轻松愉快', state: '推测', evidence: ['msg_005', 'ev_0930'] }, energy: { value: '72%', state: '推测', evidence: ['s_morning'] } },
      c2: { id: 'c2', name: '阿青', avatar: 'assets/avatars/aqing.svg', status: '异常', statusNote: '运行世界同步失败（见运行诊断）', groupId: 'grp_daily', worldId: 'w2', bio: '大学图书馆管理员。', traits: ['安静'], locationId: null },
      c3: { id: 'c3', name: '木木', avatar: 'assets/avatars/mumu.svg', status: '已暂停', statusNote: '所有者暂停自治', groupId: 'grp_none', worldId: 'w3', bio: '面包店学徒。', traits: ['元气'], locationId: null },
      c4: { id: 'c4', name: '星野', avatar: 'assets/avatars/xingye.svg', status: '已退休', statusNote: '2026-08 退休，只读归档', groupId: 'grp_none', worldId: 'w4', bio: '旧角色，只读。', traits: [], locationId: null },
    },
    worlds: { w1: { id: 'w1', name: '海湾小城 · 运行世界 #1', defVersion: 'v2' }, w2: { id: 'w2', name: '大学城' }, w3: { id: 'w3', name: '面包街' }, w4: { id: 'w4', name: '归档世界' } },
    user: { id: 'u1', name: '我', avatar: 'assets/avatars/user.svg', background: { 称呼: '你', 城市: '上海', 职业: '平面设计师', 喜好: '向日葵、看海', 可让她知道: '工作很忙，周末有空' } },
    places: {
      p_cafe: { id: 'p_cafe', name: '街角咖啡馆', scene: 'assets/scenes/cafe.svg', desc: '小遥常去画画的地方，靠窗位有阳光。', worldId: 'w1' },
      p_oldst: { id: 'p_oldst', name: '老街', scene: 'assets/scenes/oldstreet.svg', desc: '石板路老街，今天部分路段封路施工。', worldId: 'w1' },
      p_flower: { id: 'p_flower', name: '花店', scene: 'assets/scenes/flowershop.svg', desc: '阿岚经营的小花店。', worldId: 'w1' },
      p_book: { id: 'p_book', name: '海边书店', scene: 'assets/scenes/seaside.svg', desc: '海边的二手书店，目前暂停装修。', worldId: 'w1', resourceId: 'r_book' },
      p_home: { id: 'p_home', name: '小遥的公寓', scene: 'assets/scenes/apartment.svg', desc: '两层老公寓的二楼。', worldId: 'w1' },
      p_shop: { id: 'p_shop', name: '布偶服装店', scene: 'assets/scenes/shop.svg', desc: '老街口的服装小店，有碎花裙。', worldId: 'w1' },
    },
    npcs: {
      n_alan: { id: 'n_alan', full: false, bio: '开花店五年，爽朗，常给小遥留向日葵。', name: '阿岚', avatar: 'assets/avatars/alan.svg', role: '花店店主', placeId: 'p_flower' },
      n_zhou: { id: 'n_zhou', full: false, bio: '书店老板，话不多，正在装修书店。', name: '老周', avatar: 'assets/avatars/laozhou.svg', role: '海边书店老板', placeId: 'p_book' },
      n_mei: { id: 'n_mei', full: false, bio: '咖啡馆店员，爱聊八卦。', name: '小美', avatar: 'assets/avatars/xiaomei.svg', role: '咖啡馆店员', placeId: 'p_cafe' },
    },
    relations: {
      rel_alan: { id: 'rel_alan', charId: 'c1', npcId: 'n_alan', stage: '好朋友', intimacy: { value: 64, state: '推测', evidence: ['ev_0930', 'evt_1015a'] }, version: 3,
        revisions: [ { v: 1, at: '09-02', text: '初识（花店买花）', by: '运行时' }, { v: 2, at: '09-30', text: '熟人 → 朋友（一起布置橱窗）', by: '运行时', source: 'ev_0930' }, { v: 3, at: '10-15', text: '朋友 → 好朋友（封路时借伞给小遥）', by: '运行时', source: 'evt_1015a' } ] },
      rel_zhou: { id: 'rel_zhou', charId: 'c1', npcId: 'n_zhou', stage: '熟人', intimacy: { value: 40, state: '推测', evidence: ['msg_007'] }, version: 1, revisions: [{ v: 1, at: '08-20', text: '熟客', by: '运行时' }] },
    },
    messages: {
      msg_001: { id: 'msg_001', charId: 'c1', from: 'divider', text: '10月15日 · 新会话（长期状态延续）', at: '09:00' },
      msg_002: { id: 'msg_002', charId: 'c1', from: 'char', text: '早呀～今天阳光好好，我已经在咖啡馆占到靠窗的位置啦。', at: '09:12', identity: '交流' },
      msg_003: { id: 'msg_003', charId: 'c1', from: 'user', text: '早！今天打算画什么？', at: '09:13', identity: '交流' },
      msg_004: { id: 'msg_004', charId: 'c1', from: 'sys', text: '老街封路（evt_1015a）：小遥的“老街散步”已调整', at: '09:20', link: 'evt_1015a', identity: '系统运维' },
      msg_005: { id: 'msg_005', charId: 'c1', from: 'char', text: '画一组海边明信片！本来想下午去老街散步，结果封路了，只好改天啦。', at: '09:21', identity: '交流' },
      msg_006: { id: 'msg_006', charId: 'c1', from: 'user', text: '那要不要去海边书店找找参考书？', at: '09:22', identity: '交流' },
      msg_007: { id: 'msg_007', charId: 'c1', from: 'char', text: '海边书店已经停业了呢，好可惜，老周的店我很喜欢的。', at: '09:23', identity: '交流', recall: true, memoryRef: 'm31' },
      msg_008: { id: 'msg_008', charId: 'c1', from: 'char', kind: 'image', mediaId: 'md_01', text: '给你看我刚画的草稿～', at: '09:30', identity: '交流' },
      msg_009: { id: 'msg_009', charId: 'c1', from: 'char', text: '对了，我一直想要一条碎花裙，下周和阿岚去野餐想穿～', at: '09:32', identity: '交流', goalRef: 'g_dress' },
      msg_010: { id: 'msg_010', charId: 'c1', from: 'action', actionId: 'a_buy', at: '09:33' },
      msg_011: { id: 'msg_011', charId: 'c1', from: 'action', actionId: 'a12', at: '09:40' },
      msg_101: { id: 'msg_101', charId: 'c2', from: 'sys', text: '运行世界同步失败，消息暂不可送达', at: '08:00', identity: '系统运维' },
    },
    memories: {
      m31: { id: 'm31', charId: 'c1', content: '海边书店已经停业', state: '已确认', kind: '世界认知', version: 1, sourceMsg: 'msg_007', relatedPlace: 'p_book', relatedNpc: 'n_zhou', revisions: [{ v: 1, at: '10-08', text: '海边书店已经停业', by: '运行时', note: '听小美闲聊时推断' }] },
      m32: { id: 'm32', charId: 'c1', content: '用户喜欢向日葵（原记为玫瑰）', state: '已确认', kind: '关于用户', version: 2, sourceMsg: 'msg_003', revisions: [{ v: 1, at: '09-12', text: '用户喜欢玫瑰', by: '运行时' }, { v: 2, at: '09-20', text: '用户喜欢向日葵（原记为玫瑰）', by: '所有者修改', note: '用户在聊天中纠正' }] },
      m33: { id: 'm33', charId: 'c1', content: '老街封路那天，阿岚借了伞给我', state: '已确认', kind: '经历', version: 1, sourceEvent: 'evt_1015a', revisions: [{ v: 1, at: '10-15', text: '老街封路那天，阿岚借了伞给我', by: '运行时' }] },
    },
    goals: {
      g_dress: { id: 'g_dress', charId: 'c1', type: '意向', title: '想要一条碎花裙', why: '下周和阿岚野餐想穿', state: '计划', scheduleId: 's_shop', actionId: 'a_buy', itemId: 'it_dress', sourceMsg: 'msg_009' },
      g_post: { id: 'g_post', charId: 'c1', type: '目标', title: '完成 12 张海边明信片', state: '计划', progress: '5/12' },
      g_promise: { id: 'g_promise', charId: 'c1', type: '约定', title: '周六一起去看海（和你）', state: '已确认', due: '10-17 16:00' },
    },
    schedule: {
      s_morning: { id: 's_morning', charId: 'c1', time: '09:00-11:30', title: '咖啡馆画明信片', placeId: 'p_cafe', state: '已确认', actionId: 'a_draw', fact: '09:05 到达，正在画（进行中）', factState: '已确认' },
      s_walk: { id: 's_walk', charId: 'c1', time: '14:00-15:00', title: '老街散步', placeId: 'p_oldst', state: '取消', adjustedBy: 'evt_1015a', note: '因老街封路取消', fact: '未发生：老街封路', factState: '取消' },
      s_shop: { id: 's_shop', charId: 'c1', time: '15:30-16:30', title: '去布偶服装店买碎花裙', placeId: 'p_shop', state: '计划', goalId: 'g_dress', actionId: 'a_buy', fact: '尚未开始', factState: '未知' },
      s_flower: { id: 's_flower', charId: 'c1', time: '08:00-08:30', title: '去花店取花束', placeId: 'p_flower', state: '失败', actionId: 'a12', fact: '08:05 到店，付款超时未取到', factState: '失败' },
    },
    actions: {
      a_draw: { id: 'a_draw', charId: 'c1', title: '画明信片草稿', state: '执行中', placeId: 'p_cafe', scheduleId: 's_morning' },
      a_buy: { id: 'a_buy', charId: 'c1', title: '购买碎花裙', state: '候选', phase: '待请求', placeId: 'p_shop', goalId: 'g_dress', scheduleId: 's_shop', itemId: 'it_dress', log: [] },
      a12: { id: 'a12', charId: 'c1', title: '去花店取花束', state: '失败', phase: '失败', placeId: 'p_flower', taskId: 't88', itemId: 'it_bouquet', reason: '步骤 3「确认付款」超时' },
    },
    items: {
      it_coat: { id: 'it_coat', charId: 'c1', name: '焦糖色外套', cat: '服装', img: 'assets/items/coat.svg', status: '拥有', source: '购买（9月）' },
      it_scarf: { id: 'it_scarf', charId: 'c1', name: '蓝色条纹围巾', cat: '服装', img: 'assets/items/scarf.svg', status: '拥有', source: '你送的礼物', sourceMsg: 'msg_003' },
      it_hat: { id: 'it_hat', charId: 'c1', name: '草编帽', cat: '服装', img: 'assets/items/hat.svg', status: '拥有', source: '购买（8月）' },
      it_sneaker: { id: 'it_sneaker', charId: 'c1', name: '橙色帆布鞋', cat: '服装', img: 'assets/items/sneaker.svg', status: '拥有', source: '购买' },
      it_umbrella: { id: 'it_umbrella', charId: 'c1', name: '紫色雨伞', cat: '物品', img: 'assets/items/umbrella.svg', status: '借用', source: '向阿岚借（evt_1015a）', owner: 'n_alan', sourceEvent: 'evt_1015a' },
      it_book: { id: 'it_book', charId: 'c1', name: '《海边的速写》', cat: '物品', img: 'assets/items/book.svg', status: '使用', source: '海边书店购入' },
      it_bouquet: { id: 'it_bouquet', charId: 'c1', name: '向日葵花束', cat: '礼物', img: 'assets/items/bouquet.svg', status: '入库中', source: '行动 a12（失败，待重试）', actionId: 'a12' },
      it_dress: { id: 'it_dress', charId: 'c1', name: '碎花裙', cat: '服装', img: 'assets/items/dress.svg', status: '未拥有', source: '行动 a_buy', actionId: 'a_buy', goalId: 'g_dress' },
    },
    outfit: { c1: { current: ['it_coat', 'it_scarf', 'it_sneaker'], since: '08:30', history: [] } },
    media: {
      md_01: { id: 'md_01', charId: 'c1', img: 'assets/scenes/seaside.svg', title: '明信片草稿：海边书店', nonFact: true, sourceMsg: 'msg_008' },
    },
    events: {
      evt_1015a: { id: 'evt_1015a', worldId: 'w1', title: '老街封路', at: '10-15 09:20', placeId: 'p_oldst', state: '已确认',
        impacts: [ { type: 'schedule', id: 's_walk', text: '老街散步 → 取消' }, { type: 'diary', id: 'd_1015', text: '日记提到封路' }, { type: 'memory', id: 'm33', text: '新增记忆：阿岚借伞' }, { type: 'relation', id: 'rel_alan', text: '与阿岚：朋友 → 好朋友' }, { type: 'item', id: 'it_umbrella', text: '借用紫色雨伞' } ] },
      ev_0930: { id: 'ev_0930', worldId: 'w1', title: '和阿岚一起布置花店橱窗', at: '09-30', placeId: 'p_flower', state: '已确认', impacts: [{ type: 'relation', id: 'rel_alan', text: '熟人 → 朋友' }] },
    },
    diary: { d_1014: { id: 'd_1014', charId: 'c1', date: '10-14', text: '下午在咖啡馆画画，小美说海边书店停业了，有点难过，老周的店我很喜欢的。', refs: ['m31', 'p_cafe'], replies: [] }, d_1015: { id: 'd_1015', charId: 'c1', date: '10-15', text: '老街封路了，雨来得突然，阿岚把她的紫伞借给我……', refs: ['evt_1015a', 'm33'], replies: [] } },
    tasks: {
      t88: { id: 't88', charId: 'c1', actionId: 'a12', title: '取花束（a12）', state: '失败', runs: 1,
        steps: [ { n: 1, name: '前往花店', state: 'ok' }, { n: 2, name: '挑选向日葵', state: 'ok' }, { n: 3, name: '确认付款', state: 'failed', err: '支付回执超时' }, { n: 4, name: '物品入库', state: 'pending' } ] },
    },
    proposals: { pp_1: { id: 'pp_1', charId: 'c1', title: '人格提案：更主动分享画作', state: '候选' } },
    resources: {
      r_book: { id: 'r_book', type: '地点', name: '海边书店', published: 'v4', draft: null,
        versions: {
          v3: { 名称: '海边书店', 描述: '海边的二手书店，老周经营。', 营业状态: '营业中', 标签: '书店,海边' },
          v4: { 名称: '海边书店', 描述: '海边的二手书店，老周经营，新增靠窗阅读角。', 营业状态: '已重新开业', 标签: '书店,海边,阅读角' },
        } },
      r_dresscat: { id: 'r_dresscat', type: '物品', name: '碎花裙', published: 'v1', draft: null, versions: { v1: { 名称: '碎花裙', 描述: '白底小碎花及膝裙', 营业状态: '—', 标签: '服装,春夏' } } },
      r_tpl_close: { id: 'r_tpl_close', type: '事件模板', name: '街道封路', published: 'v1', draft: null, versions: { v1: { 名称: '街道封路', 描述: '施工封路，影响途经该地点的日程。不得含永久伤害。', 营业状态: '—', 标签: '事件,交通' } } },
    },
    instances: { c1: { r_book: { applied: 'v3', overrides: { 营业状态: '暂停装修（所有者修正 10-12）' } } } },
    studioDrafts: [],
    posts: {
      po_1: { id: 'po_1', worldId: 'w1', author: 'c1', authorType: 'char', text: '今天的海边明信片画到第 5 张啦，靠窗的位置阳光刚好～', at: '10-15 10:02', mediaId: 'md_01', likes: 3, liked: false, comments: [{ id: 'pc_1', author: 'n_mei', text: '画得真好！下次给我留一张', at: '10:10' }], sourceAction: 'a_draw' },
      po_2: { id: 'po_2', worldId: 'w1', author: 'u1', authorType: 'user', text: '听说老街封路了，大家出门记得绕行。', at: '10-15 09:30', likes: 1, liked: false, comments: [], refs: ['evt_1015a'] },
    },
    postSuggestions: {},
    reflections: {
      rf_1: { id: 'rf_1', charId: 'c1', at: '10-15 21:00', title: '计划被打乱也没关系', text: '封路让散步取消了，可因此和阿岚多聊了一会儿，还借到了伞。以后的计划可以留一点余地。', state: '推测', growth: '更能接受计划变化', sources: ['evt_1015a', 'm33', 'd_1015'], feedback: null },
      rf_2: { id: 'rf_2', charId: 'c1', at: '10-08 22:00', title: '听来的消息要再确认', text: '听小美说书店停业就当真了，有点难过。下次还是自己去看看比较好。', state: '候选', growth: '对传闻保持确认的习惯', sources: ['m31', 'msg_007'], feedback: null },
    },
    worldSettings: { w1: { randomEvents: true, freq: '适中', cats: { 天气: true, 交通: true, 社交: true, 小意外: false }, harmLocked: true,
      intercepts: [ { id: 'hi_1', at: '10-03', text: '“骑车摔伤导致长期行动不便”', rule: '永久伤害' }, { id: 'hi_2', at: '10-11', text: '“阿岚永久离开小城”', rule: '永久失去重要关系' } ] } },
    prefs: { global: { 语言: '简体中文', 主题: '暖色', 通知: true, 摘要: '每日' }, autonomy: { c1: { 自治程度: '自主（重要事项告知我）', 主动触达: '适中', 免打扰: '23:00-08:00', 允许发动态: true } } },
    session: null,
    notices: [
      { id: 'nt_1', text: '老街封路影响了小遥的日程', link: '#/c/c1/life/schedule', read: false },
      { id: 'nt_2', text: '任务 t88 失败：取花束', link: '#/diag/task/t88', read: false },
      { id: 'nt_3', text: '海边书店定义 v4 待应用到小遥', link: '#/studio/edit/r_book', read: false },
    ],
    audit: [],
  };
}


export { seed };
