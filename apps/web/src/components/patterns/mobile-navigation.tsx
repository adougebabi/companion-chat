import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';

export type MobileNavigationItem = { value: string; label: string; icon: ReactNode; disabled?: boolean };
/** Phone destinations. The shell reserves its height; workflow pages omit it. */
export function MobileNavigation({ items, value, onSelect }: {
  items: MobileNavigationItem[]; value: string; onSelect: (value: string) => void;
}) {
  return <nav className="mobile-bottom-nav" aria-label="手机主导航">
    {items.map(item => <Button key={item.value} type="button" variant="ghost"
      className={value === item.value ? 'active' : undefined} disabled={item.disabled}
      aria-current={value === item.value ? 'page' : undefined}
      aria-label={item.value === 'more' ? '打开全部功能' : `${item.label}首页`}
      onClick={() => onSelect(item.value)}>
      <span className="mobile-nav-symbol">{item.icon}</span><span>{item.label}</span>
    </Button>)}
  </nav>;
}
