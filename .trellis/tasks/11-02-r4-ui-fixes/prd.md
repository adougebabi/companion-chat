# R4 deployment and UI fixes

Repair the five requested audit items from `/workspace/ux/audit-20261102-r4`:

- HTML must revalidate while hashed assets cache long term. Stale lazy imports automatically reload at most once per tab session, then expose a manual reload prompt.
- D8 world selection uses shadcn RadioGroup cards. Creating a new world draft has equal visual weight, explains returning to the wizard, and cannot advance with an unfinished choice. Daily confirmation says `将为 TA 准备一个日常生活世界`.
- The library distinguishes a saved model service without a generic purpose binding from an absent service, and links to the matching configuration section.
- World directory empty states distinguish no data from active filters and offer one CTA through shadcn Empty.
- Empty diagnostic tasks link to character creation.

One behavioral regression per item. Do not weaken existing assertions; record all changed assertions and reasons. No `/workspace/accept`, env, image or Git commit changes. Use Node 24.12 from `/tmp`, only targeted tests plus web typecheck, lint and build. Finish within 25 minutes.
