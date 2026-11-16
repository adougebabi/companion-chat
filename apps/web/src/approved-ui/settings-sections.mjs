const settingsSections = [
  '服务地址', '模型用途', '聊天回复', '看图理解', '图片（ComfyUI）',
  '能力插件', '角色自主与主动联系', '安静时段', '通知',
  '作息与后台节奏', '资源与预算', '高级',
];

export function normalizeSettingsSection(section) {
  return settingsSections.includes(section) ? section : '服务地址';
}

export function legacySettingsSection(path) {
  const section = decodeURIComponent(path.slice('/settings/'.length));
  const aliases = {
    models: '服务地址', 'reply-vision': '聊天回复',
    'media-workflows': '图片（ComfyUI）', runtime: '角色自主与主动联系',
    outreach: '角色自主与主动联系', notifications: '通知',
    capabilities: '能力插件', kev: '高级',
  };
  return normalizeSettingsSection(aliases[section] || section);
}
