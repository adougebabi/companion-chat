# Dark mode adaptation

## Goal

Make the active companion UI comfortable and readable in dark system themes
without changing its existing product behavior or persona color data.

## Confirmed Facts

- The active browser entry is `src/index.html`, which loads
  `src/companion-style.css` and `src/companion-main.js`.
- `src/companion-style.css` currently forces `color-scheme: light`; it has no
  theme tokens or dark-mode media query.
- No theme setting, preference persistence, or theme-toggle UI currently
  exists. The JavaScript store is unrelated to themes.
- `src/companion-main.js` has unrelated uncommitted changes. The adaptation can
  be isolated to the stylesheet.
- `src/style.css` and `src/main.js` are not loaded by the active entry and are
  outside this task's scope.

## Requirements

- R1. Add a system-preference dark appearance for the active companion UI.
- R2. Preserve the current light appearance as the fallback when the system is
  not in dark mode.
- R3. Cover the primary application surfaces, navigation, chat, activity
  cards, composer, form controls, dialogs, settings, inspector, floating chat
  header, and their high-value interactive states.
- R4. Preserve dynamic persona/avatar colors and existing behavior; do not add
  a preference setting, persistence, or JavaScript theme logic.
- R5. Keep the change confined to `src/companion-style.css` unless inspection
  proves another active front-end file is strictly necessary.

## Acceptance Criteria

- [ ] AC1. The stylesheet declares support for light and dark browser color
  schemes and uses a dark system-preference override.
- [ ] AC2. In a dark system theme, all primary visible surfaces and text remain
  legible, including dialogs and the mobile/floating chat header.
- [ ] AC3. Inputs, buttons, selected/hover/focus states, disabled send state,
  warnings/errors, media status, and dialog backdrops retain usable contrast.
- [ ] AC4. In a light system theme, the UI continues to render with its current
  visual intent.
- [ ] AC5. No JavaScript, server, persisted-data, or persona-color behavior is
  changed for this work.

## Out of Scope

- A manual theme switch, preference persistence, or a setting-screen control.
- Automatic browser-based visual regression testing infrastructure.
- Restyling the unused legacy `src/style.css` / `src/main.js` UI.

## Theme Decision

Dark mode follows the operating-system preference only, via CSS. There is no
manual in-app switch and no persisted theme preference.

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
