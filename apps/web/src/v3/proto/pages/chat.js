import { isMock } from '../store.js';
import { providerPage } from './provider-page.js';
const mock = isMock ? await import('../mock/chat-page.js') : null;
export { GeneratedImage as genImg } from '../components/chat.js';
export const renderChat = (el,ctx) => mock ? mock.renderChat(el,ctx) : providerPage(() => import('./real-chat.js'), 'renderChat',el,ctx);
export const refreshStream = (...args) => mock?.refreshStream(...args);
export const actionDrawer = (...args) => mock ? mock.actionDrawer(...args) : import('./real-chat.js').then(module => module.actionDrawer(...args));
export const openCorrection = (...args) => mock ? mock.openCorrection(...args) : import('./real-chat.js').then(module => module.openCorrection(...args));
