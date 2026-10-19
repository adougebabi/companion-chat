import { acceptancePath } from './v3/mock-navigation.mjs';
import { createApp } from 'vue';
import App from './App.vue';
import './styles/v3-proto.css';
const bookmarkedPage = acceptancePath(location.pathname, location.search);
if (!location.hash && bookmarkedPage) history.replaceState(null, '', '/#' + bookmarkedPage);
createApp(App).mount('#app');
