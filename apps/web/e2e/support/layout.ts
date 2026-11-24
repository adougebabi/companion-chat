import { expect, type Locator, type Page } from '@playwright/test';

export type LayoutDefect = { kind: string; target: string; detail: string };

/**
 * Real-browser layout audit for a container (page region or overlay).
 *
 * Reports:
 * - page-overflow: the document scrolls horizontally.
 * - container-outside-viewport: the container extends past the viewport.
 * - control-outside-container: a visible control sits outside the container and
 *   is not inside a scrollable region that can bring it into view.
 * - control-clipped: an ancestor with overflow hidden/clip cuts the control.
 * - content-overflow: a control's own content is larger than its box (text
 *   spilling out of a fixed-height button, the original schedule-sheet bug).
 * - touch-target: on narrow viewports a control is smaller than `minTarget`.
 */
export async function findLayoutDefects(
  page: Page,
  container: Locator,
  { minTarget = 0 }: { minTarget?: number } = {},
): Promise<LayoutDefect[]> {
  const handle = await container.elementHandle();
  if (!handle) return [{ kind: 'missing-container', target: String(container), detail: 'container not found' }];
  // Measure settled geometry only: wait for enter/exit animations to finish.
  // Infinite animations (skeleton pulse, spinners) never finish; ignore them.
  await page.waitForFunction(el => (el as Element).getAnimations({ subtree: true }).every(a => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity), handle, { timeout: 5000 });
  return handle.evaluate((root: Element, minTarget: number) => {
    const defects: { kind: string; target: string; detail: string }[] = [];
    const vw = document.documentElement.clientWidth, vh = window.innerHeight;
    const name = (el: Element) => {
      const label = (el.getAttribute('aria-label') || el.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 24);
      return `${el.tagName.toLowerCase()}${el.getAttribute('role') ? `[role=${el.getAttribute('role')}]` : ''} "${label}"`;
    };
    const r = (n: number) => Math.round(n);
    if (document.documentElement.scrollWidth > vw + 1)
      defects.push({ kind: 'page-overflow', target: 'document', detail: `scrollWidth ${document.documentElement.scrollWidth} > viewport ${vw}` });
    const box = root.getBoundingClientRect();
    // Vertical extent matters only for fixed overlays; page regions may scroll.
    const fixed = getComputedStyle(root).position === 'fixed';
    if (box.left < -1 || box.right > vw + 1 || (fixed && (box.top < -1 || box.bottom > vh + 1)))
      defects.push({ kind: 'container-outside-viewport', target: name(root), detail: `box ${r(box.left)},${r(box.top)}–${r(box.right)},${r(box.bottom)} viewport ${vw}x${vh}` });
    const visible = (el: Element) => {
      const style = getComputedStyle(el);
      if (style.visibility === 'hidden' || style.display === 'none') return false;
      const rect = el.getBoundingClientRect();
      return rect.width > 0 && rect.height > 0 && !el.closest('[hidden],[aria-hidden="true"],[inert]');
    };
    const controls = [...root.querySelectorAll('button, a[href], input:not([type=hidden]), select, textarea, [role=tab], [role=checkbox], [role=combobox]')].filter(visible);
    for (const el of controls) {
      const rect = el.getBoundingClientRect();
      // Effective box: once a scroll container can bring the control into view
      // on an axis, the control counts as inside that container on that axis.
      const eff = { l: rect.left, r: rect.right, t: rect.top, b: rect.bottom };
      for (let a = el.parentElement; a && a !== root.parentElement; a = a.parentElement) {
        const s = getComputedStyle(a);
        const ar = a.getBoundingClientRect();
        const outsideX = eff.l < ar.left - 1 || eff.r > ar.right + 1;
        const outsideY = eff.t < ar.top - 1 || eff.b > ar.bottom + 1;
        if (outsideX && /(auto|scroll)/.test(s.overflowX) && a.scrollWidth > a.clientWidth + 1) { eff.l = Math.min(Math.max(eff.l, ar.left), ar.right); eff.r = Math.max(Math.min(eff.r, ar.right), ar.left); }
        else if (outsideX && /(hidden|clip)/.test(s.overflowX))
          defects.push({ kind: 'control-clipped', target: name(el), detail: `x ${r(eff.l)}–${r(eff.r)} clipped by ${a.tagName.toLowerCase()} ${r(ar.left)}–${r(ar.right)}` });
        if (outsideY && /(auto|scroll)/.test(s.overflowY) && a.scrollHeight > a.clientHeight + 1) { eff.t = Math.min(Math.max(eff.t, ar.top), ar.bottom); eff.b = Math.max(Math.min(eff.b, ar.bottom), ar.top); }
        else if (outsideY && /(hidden|clip)/.test(s.overflowY))
          defects.push({ kind: 'control-clipped', target: name(el), detail: `y ${r(eff.t)}–${r(eff.b)} clipped by ${a.tagName.toLowerCase()} ${r(ar.top)}–${r(ar.bottom)}` });
      }
      if (eff.l < box.left - 1 || eff.r > box.right + 1)
        defects.push({ kind: 'control-outside-container', target: name(el), detail: `x ${r(eff.l)}–${r(eff.r)} container ${r(box.left)}–${r(box.right)}` });
      // Rendered content (text and descendant boxes, not pseudo-elements) must
      // fit inside the control's own box: catches text spilling out of a
      // fixed-height button and overlapping its neighbours.
      if (!['INPUT', 'SELECT', 'TEXTAREA'].includes(el.tagName) && getComputedStyle(el).overflow === 'visible') {
        // Visually hidden text (sr-only: 1px clipped box) is not rendered content.
        const rects: DOMRect[] = [];
        const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
        for (let node = walker.nextNode(); node; node = walker.nextNode()) {
          const parent = node.parentElement!;
          const pr = parent.getBoundingClientRect();
          const ps = getComputedStyle(parent);
          if ((pr.width <= 1 || pr.height <= 1) && ps.position === 'absolute') continue;
          if (ps.clipPath !== 'none' || (ps.clip && ps.clip !== 'auto')) continue;
          const range = document.createRange();
          range.selectNodeContents(node);
          rects.push(...[...range.getClientRects()].filter(q => q.width > 0 && q.height > 0));
        }
        for (const child of el.querySelectorAll('*')) {
          const cs = getComputedStyle(child);
          if (cs.display === 'none' || cs.position === 'absolute' || cs.position === 'fixed') continue;
          const cr = child.getBoundingClientRect();
          if (cr.width > 0 && cr.height > 0 && child.children.length === 0 && !child.textContent?.trim()) rects.push(cr);
        }
        const spill = rects.find(q => q.left < rect.left - 2 || q.right > rect.right + 2 || q.top < rect.top - 2 || q.bottom > rect.bottom + 2);
        if (spill)
          defects.push({ kind: 'content-overflow', target: name(el), detail: `content ${r(spill.left)},${r(spill.top)}–${r(spill.right)},${r(spill.bottom)} box ${r(rect.left)},${r(rect.top)}–${r(rect.right)},${r(rect.bottom)}` });
      }
      // A control wrapped by its <label> is hit through the whole label row.
      const label = el.closest('label');
      const hitHeight = Math.max(rect.height, label ? label.getBoundingClientRect().height : 0);
      if (minTarget && hitHeight < minTarget - 0.5 && !el.matches('a:not([data-slot=button])'))
        defects.push({ kind: 'touch-target', target: name(el), detail: `height ${r(rect.height)} < ${minTarget}` });
    }
    return defects;
  }, minTarget);
}

export async function expectNoLayoutDefects(page: Page, container: Locator, options?: { minTarget?: number }) {
  const defects = await findLayoutDefects(page, container, options);
  expect(defects, `layout defects:\n${defects.map(d => `- ${d.kind}: ${d.target} (${d.detail})`).join('\n')}`).toEqual([]);
}

/** Every tab in a tab list must be reachable: visible or scrollable into view. */
export async function expectTabsReachable(list: Locator) {
  const tabs = list.getByRole('tab');
  const count = await tabs.count();
  for (let i = 0; i < count; i++) {
    const tab = tabs.nth(i);
    await tab.scrollIntoViewIfNeeded();
    const [t, l] = await Promise.all([tab.boundingBox(), list.boundingBox()]);
    expect(t && l, 'tab and list are rendered').toBeTruthy();
    expect(t!.x, `tab ${i} left edge inside list`).toBeGreaterThanOrEqual(l!.x - 1);
    expect(t!.x + t!.width, `tab ${i} right edge inside list`).toBeLessThanOrEqual(l!.x + l!.width + 1);
  }
}
