import { createApp } from "vue";
import { createPinia } from "pinia";

import App from "./App.vue";
import "./styles/v3-proto.css";

createApp(App).use(createPinia()).mount("#app");
