# Implementation
1. Read relevant layer contracts and inspect audit/data via GET.
2. Fix separate owning projections/components; add behavioral regressions without loosening assertions.
3. Run targeted Go with isolated DB schema, JS tests, web lint/typecheck, production build.
4. Serve actual dist with isolated read-only fixtures; Chrome desktop/mobile screenshots in /tmp/r34-shots.
5. Record per-item causes, files, verification and changed assertions in Chinese. Do not commit.
