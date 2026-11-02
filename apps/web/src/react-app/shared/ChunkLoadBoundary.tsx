import { Component, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Feedback } from './Presentation';

export const chunkReloadKey = 'fluctlight:chunk-reload-attempted';

export function isChunkLoadError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false;
  const { name, message } = error as { name?: string; message?: string };
  return name === 'ChunkLoadError' || /failed to fetch dynamically imported module|importing a module script failed|error loading dynamically imported module|failed to load module script|loading (?:css )?chunk .*failed|unable to preload css/i.test(message || '');
}

export function reloadChunkOnce(error: unknown, reload: () => void): boolean {
  if (!isChunkLoadError(error)) return false;
  try {
    // Keep the guard for this tab's session, including across a failed reload.
    // If storage is unavailable, offer a manual reload instead of risking a loop.
    if (sessionStorage.getItem(chunkReloadKey)) return false;
    sessionStorage.setItem(chunkReloadKey, '1');
    reload();
    return true;
  } catch {
    return false;
  }
}

const reloadPage = () => window.location.reload();

export default class ChunkLoadBoundary extends Component<{
  children: ReactNode;
  reload?: () => void;
}, { error: unknown }> {
  state: { error: unknown } = { error: null };

  static getDerivedStateFromError(error: unknown) { return { error }; }

  componentDidCatch(error: unknown) {
    reloadChunkOnce(error, this.props.reload || reloadPage);
  }

  render() {
    if (!this.state.error) return this.props.children;
    const staleChunk = isChunkLoadError(this.state.error);
    return <main className="react-workspace replica-workspace p-6">
      <Feedback kind="error" title={staleChunk ? '页面资源未能加载' : '页面暂时无法显示'}
        actions={<Button onClick={this.props.reload || reloadPage}>重新加载页面</Button>}>
        {staleChunk ? '网站可能已更新，或网络暂时中断。若自动刷新后仍未恢复，请检查网络后重新加载页面。' : '请重新加载页面后继续。'}
      </Feedback>
    </main>;
  }
}
