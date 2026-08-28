# Frontend Directory And Structure

```text
apps/web/index.html         Static shell and loading skeleton
apps/web/src/main.ts        Vue + Pinia entry point
apps/web/src/app/*          Application shell and view routing
apps/web/src/api/*          Typed HTTP/NDJSON clients and normalizers
apps/web/src/stores/*       Shared server/app state
apps/web/src/composables/*  Chat, history, activity, composer and dialog effects
apps/web/src/components/*  Feature components and accessible interactions
apps/web/src/styles/*       Migrated visual tokens and responsive layout
apps/web/dist/              Vite production output, served by Nginx
```

Keep API parsing at the generated browser-client boundary, shared state in Pinia stores, and DOM side
effects in composables. Components should emit intent to their parent or store;
they should not perform raw fetches. Use kebab-case CSS classes and stable
dimensions for message/media surfaces.
