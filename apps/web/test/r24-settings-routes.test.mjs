import test from 'node:test';
import assert from 'node:assert/strict';
import {approvedRoute} from '../src/approved-ui/routing.mjs';
import {normalizeSettingsSection} from '../src/approved-ui/settings-sections.mjs';

test('旧能力链接重定向到能力插件并保留其他查询条件', () => {
  const route = approvedRoute('#/settings/capabilities?fid=sample&section=旧分区');
  assert.equal(route.page, '管理中心');
  assert.equal(route.path, '/settings');
  assert.equal(route.query.get('section'), '能力插件');
  assert.equal(route.query.get('fid'), 'sample');
  const redirected = approvedRoute(route.redirectHash);
  assert.equal(redirected.query.get('section'), '能力插件');
  assert.equal(redirected.query.get('fid'), 'sample');
  assert.equal(redirected.redirectHash, undefined, '规范地址不会重复重定向');
});

test('其他已知旧设置链接映射到现有分区', () => {
  for (const [path, section] of Object.entries({
    models: '服务地址', 'reply-vision': '聊天回复',
    'media-workflows': '图片（ComfyUI）', runtime: '角色自主与主动联系',
    outreach: '角色自主与主动联系', notifications: '通知', kev: '高级',
  })) {
    const route = approvedRoute('#/settings/' + path);
    assert.equal(route.query.get('section'), section, path);
    assert.equal(approvedRoute(route.redirectHash).query.get('section'), section, path);
  }
  const diagnostics = approvedRoute('#/settings/diagnostics');
  assert.equal(diagnostics.path, '/diag');
  assert.equal(diagnostics.diagnostics, true);
});

test('未知、退役和多层旧子路径统一回到模型服务', () => {
  for (const path of ['owner', 'unknown-private-key', 'models/retired', 'capabilities/', '']) {
    const route = approvedRoute('#/settings/' + path);
    assert.equal(route.page, '管理中心', path);
    assert.equal(route.query.get('section'), '服务地址', path);
    assert.equal(approvedRoute(route.redirectHash).query.get('section'), '服务地址', path);
  }
});

test('中文设置分区保持可用，错误编码与禁用路由仍被拒绝', () => {
  const sections = ['服务地址', '模型用途', '聊天回复', '看图理解', '图片（ComfyUI）',
    '能力插件', '角色自主与主动联系', '安静时段', '通知', '作息与后台节奏', '资源与预算', '高级'];
  for (const section of sections) {
    assert.equal(normalizeSettingsSection(section), section);
    assert.equal(approvedRoute('#/settings/' + encodeURIComponent(section)).query.get('section'), section);
  }
  assert.equal(approvedRoute('#/settings/%E0%A4').notFound, true);
  assert.equal(approvedRoute('#/settings/capabilities?legacy=1').removed, true);
  assert.equal(approvedRoute('#/settings').redirectHash, undefined);
});

test('页面分区选择也兜底未知值，避免查询参数或恢复状态造成空面板', () => {
  for (const section of [undefined, null, '', '能力与权限', '所有者与会话', 'private-key'])
    assert.equal(normalizeSettingsSection(section), '服务地址');
});
