// Fixture event log read by Playwright via window.__uiLabEvents.
export type FixtureEvent = { action: string; args: unknown[] };
export const events: FixtureEvent[] = [];
