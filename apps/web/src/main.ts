import { createApp } from "vue";
import { createPinia } from "pinia";

import App from "./App.vue";
import "./styles/globals.css";
import "./styles/app.css";

createApp(App).use(createPinia()).mount("#app");
