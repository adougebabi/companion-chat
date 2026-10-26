import { bookmarkedPath } from './approved-ui/routing.mjs';
import { createApp } from 'vue';
import App from './App.vue';
import './styles/approved-ui.css';
const bookmarkedPage = bookmarkedPath(location.pathname, location.search);
if (!location.hash && bookmarkedPage) history.replaceState(null, '', '/#' + bookmarkedPage);
createApp(App).mount('#app');
