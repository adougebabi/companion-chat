// 分区：相处体验（BP-02/03/04）
import { register, placeholder } from './registry.js';
import { renderChat, refreshStream } from '../pages/chat.js';
import { renderHistory, renderCorrections, renderChatMedia, renderFeed, renderPost, renderOutreach, renderInbox } from '../pages/companion.js';
const Z = '相处体验';
register({ route: '/c/:charId/chat', key: 'chat.thread.view', title: '对话', bp: 'BP-02 / BP-03', zone: Z, coverage: '可操作', manualRefresh: true,
  iface: [['发送/拉取', '现有·待核验'], ['在场标识', '补齐'], ['消息类型分类', '新增'], ['纠错', '新增']], source: '角色库进入默认落地；通知/动态深链。右侧摘要随 store 实时刷新',
  render: renderChat, onStore: (e, ctx) => refreshStream(ctx) });
register({ route: '/c/:charId/chat/history', key: 'chat.history.view', title: '会话历史与搜索', bp: 'BP-02', zone: Z, coverage: '可操作', iface: [['消息搜索', '新增']], source: '对话页顶部「历史」', render: renderHistory });
register({ route: '/c/:charId/chat/corrections', key: 'chat.correction.list', title: '纠错记录', bp: 'BP-02', zone: Z, coverage: '可操作', iface: [['纠错列表（派生）', '新增']], source: '对话右栏「纠错记录」；日记/记忆的“可能有误”提示', render: renderCorrections });
placeholder({ route: '/c/:charId/chat/call', key: 'chat.call.view', title: '语音与通话（P2 占位）', bp: 'BP-03.2-3', zone: Z, note: 'P2：语音输入/实时通话仅占位。' });
register({ route: '/c/:charId/chat/media', key: 'chat.media.view', title: '聊天中的图片', bp: 'BP-03', zone: Z, coverage: '仅展示', iface: [['多模态上传', '待核验']], source: '对话附件按钮（未接入）', render: renderChatMedia });
register({ route: '/feed', key: 'feed.post.list', title: '动态', bp: 'BP-04', zone: Z, coverage: '可操作', scope: '运行世界', iface: [['动态列表', '新增'], ['发布（用户本人）', '新增'], ['发帖建议', '新增']], source: '全局导航「动态」。用户只能以自己的 Actor 发帖，或向角色提建议', render: renderFeed });
register({ route: '/feed/:postId', key: 'feed.post.detail', title: '动态详情', bp: 'BP-04', zone: Z, coverage: '可操作', scope: '运行世界', render: renderPost });
register({ route: '/settings/outreach', key: 'settings.char.autonomy#outreach', title: '主动触达设置', bp: 'BP-03 / BP-16', zone: Z, coverage: '仅展示', source: '仅记录偏好，不改变运行时权限', render: renderOutreach });
register({ route: '/inbox', key: 'inbox.notice.list', title: '通知中心', bp: 'BP-03', zone: Z, coverage: '可操作', scope: '全局定义', iface: [['通知列表', '待核验']], render: renderInbox });
