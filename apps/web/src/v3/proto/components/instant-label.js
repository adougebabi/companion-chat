import {esc} from './controls.js';
import {readingInstant,userInstant} from '../../presentation-time.mjs';

export function InstantLabel(value,zone){
  return `<time${typeof value==='string'&&Number.isFinite(Date.parse(value))?` datetime="${esc(value)}"`:''} title="${esc(userInstant(value,zone))}" aria-label="${esc(userInstant(value,zone))}">${esc(readingInstant(value,zone))}</time>`;
}
