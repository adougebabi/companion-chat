# Frontend Directory And Structure

```text
web/index.html              Static shell and loading skeleton
web/src/main.ts             Vue + Pinia entry point
web/src/app/App.vue         Application shell and view routing
web/src/api/*               Typed HTTP/SSE clients and normalizers
web/src/stores/*            Shared server/app state
web/src/composables/*       Chat, history, activity, composer and dialog effects
web/src/components/*        Feature components and accessible interactions
web/src/styles/*            Migrated visual tokens and responsive layout
dist/                       Vite production output, served by Express
```

Keep API parsing in `web/src/api`, shared state in Pinia stores, and DOM side
effects in composables. Components should emit intent to their parent or store;
they should not perform raw fetches. Use kebab-case CSS classes and stable
dimensions for message/media surfaces.
