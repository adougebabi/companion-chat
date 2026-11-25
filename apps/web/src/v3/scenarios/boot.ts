// Imported first by main.tsx. BrowserClient binds `fetch` when it is
// constructed, so the scenario interceptor must be installed synchronously
// before the application graph evaluates. The env check is a build-time
// constant: production and normal dev builds drop the whole scenario module.
import {installApiScenarios} from './install';

if(import.meta.env.VITE_API_SCENARIOS==='1')installApiScenarios();
