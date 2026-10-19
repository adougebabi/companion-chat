/** Resolve existing bookmarked URLs to the v3 Mock page without real API boot. */
export function acceptancePath(pathname, search = '', characterId = 'c1') {
  const query = new URLSearchParams(search), section = query.get('section');
  const path = pathname.replace(/\/$/, '') || '/';
  if (path === '/') return null;
  if (path === '/instances') return '/library';
  if (path === '/moments') return '/feed';
  if (path === '/settings/diagnostics' || path === '/diagnostics') return `/diag${section ? '/' + (section === 'kev-decisions' ? 'kev' : section) : ''}`;
  if (path === '/settings') return `/settings${section ? '/' + section : ''}`;
  if (path === '/chat') return `/c/${characterId}/chat`;
  if (path === '/world') return `/c/${characterId}/world`;
  if (path === '/people') return `/c/${characterId}/people`;
  if (path.startsWith('/c/') || ['/library', '/feed', '/studio', '/auth', '/inbox', '/data', '/diag'].some(prefix => path === prefix || path.startsWith(prefix + '/'))) return path + search;
  return null;
}
