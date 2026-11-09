# Standalone Agent prompt document review

The Markdown contains each of the 30 registered Agent IDs and names, their role and trigger, fixed prompt text, conditional fragments, dynamic message placeholders, and native request structures. Shared rules are reproduced within the document; source references are optional maintenance details.

Independent read-only reviewers identified omissions in model-visible tool descriptions, Owner media source semantics, Reflection conditional evidence semantics, and the Owner life request field allowlist. The document now includes those items. It also contains all 63 built-in/native/wrapper tool definitions in their model-visible strict form, 51 catalog purpose strings, and 33 response-format/schema snapshots, with runtime selection and dynamic cases labeled.

The fixed prompt excerpts were decoded from current Go source. The initialization array guide was produced from the current schema helpers. Static checks compared all 30 registered IDs/names, selected complete prompt literals and conditional replacements, the initialization guide, local and internal links, balanced fences, all 63 tool wire definitions, all 51 catalog purposes and all 33 response schema snapshots. All checks passed. No runtime prompt code was changed and no tests were run.
