# Design

Fix behavior at its existing authority: persona preparation state and conversation admission; per-character workflow dispatch lanes; studio domain attempt/retry settlement; creation name projection; typed lifecycle reason production; visual failure presentation and centralized provider prompt construction.

Retain existing APIs, database schema, transactional writes, leases, stable workflow/provider IDs and Temporal history compatibility. Separate slow lanes only where frozen state and transactional/CAS checks make it safe. Prefer existing recovery endpoint and UI controller rather than a new API. Each defect receives focused regression coverage; full suites verify cross-layer behavior.

No DB migration or config edits are planned; document any requirement discovered during investigation. Preserve existing assertions unless explicitly superseded by the requested behavior.
