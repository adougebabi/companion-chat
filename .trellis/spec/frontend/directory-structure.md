# Frontend Directory And Structure

```text
apps/web/index.html                 Sole static product HTML entry
apps/web/src/main.tsx               React root and bookmark normalization
apps/web/src/react-app/App.tsx      Shell, role navigation and explicit page dispatch
apps/web/src/react-app/pages/       React page/controller ownership
apps/web/src/react-app/views/       Presentational React views with named models
apps/web/src/react-app/shared/      Domain composition (fields, preparation, semantic trees)
apps/web/src/react-app/components/  Focused shared React interactions
apps/web/src/components/ui/*.tsx    Official shadcn Radix source, locally themed
apps/web/src/state/                 Framework-independent workspace/conversation/creation state
apps/web/src/approved-ui/*.mjs      Retained pure routing, validation, projections and recovery helpers
apps/web/src/v3/providers/          Existing typed BrowserClient domain adapters
apps/web/src/v3/proto/              Retained service/snapshot facade and shared helpers; NO pages/modules/app
apps/web/src/lib/                   Shared validation and presentation utilities
apps/web/src/styles/react*.css      Tailwind, tokens and approved scoped layout
apps/web/test/                      React DOM, domain and provider tests
apps/web/dist/                      Vite production output served by Nginx
```

Keep API parsing at the generated browser-client boundary. State controllers own
request identity and asynchronous authority; components express user intent.
The historical Vue views/components/stores, compiled renderers and legacy DOM
page router have been removed. Never recreate them as a compatibility entry.
A new page requires an explicit route and real React component; unknown routes
render an unavailable page. Static QA HTML must not create another product entry.
