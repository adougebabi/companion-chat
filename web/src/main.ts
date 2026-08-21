import {createPinia} from 'pinia';
import {createApp} from 'vue';
import App from './app/App.vue';
import './styles/app.css';

createApp(App).use(createPinia()).mount('#app');
