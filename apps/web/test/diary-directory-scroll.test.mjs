import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {JSDOM} from 'jsdom';

const css = await readFile(new URL('../src/styles/replica-life.css', import.meta.url), 'utf8');
const dom = new JSDOM(`<style>${css}</style>`);
const media = [...dom.window.document.styleSheets[0].cssRules].filter(rule => rule.cssRules);
const directoryRule = query => media.filter(rule => rule.conditionText === query).flatMap(rule => [...rule.cssRules])
  .find(rule => rule.selectorText === '.replica-diary .lp-diary-index').style;
test.after(() => dom.window.close());

test('桌面日记目录在共用页面内吸顶，避开页头且长目录独立滚动', () => {
  const style = directoryRule('(min-width:601px)');
  assert.equal(style.getPropertyValue('position'), 'sticky');
  assert.equal(style.getPropertyValue('align-self'), 'start', '防止目录被 grid 拉伸到正文高度而无法吸顶');
  assert.equal(style.getPropertyValue('top'), '96px');
  assert.equal(style.getPropertyValue('max-height'), 'calc(100dvh - 62px - 96px - var(--replica-page-top,32px) - var(--replica-page-bottom,48px))');
  assert.equal(style.getPropertyValue('overflow-y'), 'auto', '所有日期及加载更早按钮仍可滚动访问');
  assert.equal(style.getPropertyValue('overscroll-behavior-y'), 'contain');
});

test('手机日记目录继续使用横向滚动，不继承桌面吸顶或限高', () => {
  const style = directoryRule('(max-width:600px)');
  assert.equal(style.getPropertyValue('display'), 'flex');
  assert.equal(style.getPropertyValue('overflow'), 'auto');
  for (const property of ['position', 'top', 'max-height']) assert.equal(style.getPropertyValue(property), '');
  const unscoped = [...dom.window.document.styleSheets[0].cssRules]
    .filter(rule => rule.selectorText?.includes('lp-diary-index'));
  for (const rule of unscoped) {
    assert.equal(rule.style.getPropertyValue('position'), '');
    assert.equal(rule.style.getPropertyValue('max-height'), '');
  }
});
