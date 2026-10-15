import { createApp } from "vue";
import { createPinia } from "pinia";

import App from "./App.vue";
import "./styles/globals.css";
import "./styles/app.css";
import "./styles/product.css";
import "./styles/sage.css";

createApp(App).use(createPinia()).mount("#app");
