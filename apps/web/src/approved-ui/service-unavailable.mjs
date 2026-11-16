// User-facing explanation for a failed read. Raw codes and text are only used for
// classification here; nothing returned from this module echoes them back.

/** @param {any} error */
export function readErrorCause(error) {
  return {
    code: String(error?.code ?? error?.error_code ?? ''),
    status: Number(error?.status ?? error?.statusCode ?? 0) || 0,
    text: String(error?.message ?? error ?? '').slice(0, 500),
  };
}

const modelPattern = /model|role_unconfigured|provider.role|llm|conversation_reply|generic_llm|模型/i;
const mediaPattern = /(?:^|[^a-z])(?:media|image|vision|renderer)(?:[^a-z]|$)|图片|视觉/i;
const networkPattern = /Failed to fetch|NetworkError|fetch failed|ECONNREFUSED|无法连接/i;

/**
 * @param {{code?:string,status?:number,text?:string}|undefined|null} cause
 * @param {string} [message] already localized message
 * @returns {{kind:'model'|'media'|'service'|'network'|'unknown',service:string,reason:string,configSection:string}}
 */
export function serviceUnavailableNotice(cause, message = '') {
  const source = [cause?.code, cause?.text, message].filter(Boolean).join(' ');
  const status = Number(cause?.status) || 0;
  // Image providers often mention both “provider” and “model”; prefer the
  // more specific service before classifying general model failures.
  if (mediaPattern.test(source))
    return { kind: 'media', service: '图片服务', reason: '所需的图片服务尚未配置或暂时不可用。请到管理中心核对图片服务设置。', configSection: '模型用途' };
  if (modelPattern.test(source))
    return { kind: 'model', service: '模型服务', reason: '所需的模型服务尚未配置或暂时不可用。请到管理中心的「模型用途」核对服务和模型分配。', configSection: '模型用途' };
  if (status === 503 || /unavailable|service_unavailable|503/i.test(source))
    return { kind: 'service', service: '后台服务', reason: '后台服务暂时不可用，暂时无法确认具体原因。请到管理中心检查服务状态后重新读取。', configSection: '模型用途' };
  if (networkPattern.test(source))
    return { kind: 'network', service: '服务连接', reason: '暂时无法连接到服务，请检查服务地址或稍后重新读取。', configSection: '服务地址' };
  return { kind: 'unknown', service: '服务', reason: '衣柜服务暂时无法完成读取，暂时无法确认具体原因。请到管理中心检查服务状态，或稍后重新读取。', configSection: '服务地址' };
}

export function settingsHref(section) {
  return '#/settings?section=' + encodeURIComponent(section || '模型用途');
}
