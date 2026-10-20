// Compatibility facade: pages compose shared components; state stays in the Mock store.
import * as store from './store.js';
import { confirm } from './components/overlay.js';
export * from './components/controls.js';
export * from './components/overlay.js';
export * from './components/toast.js';
export * from './components/states.js';
export * from './components/sources.js';
export const $ = (selector, root = document) => root.querySelector(selector);
export const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
/** Workspace is navigation context; the authenticated server owns authorization. */
export async function requireOwner() {
  return store.isMock || !!store.get('session')?.actorId;
}
