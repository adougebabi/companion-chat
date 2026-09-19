import assert from "node:assert/strict";
import test from "node:test";

import { runtimeConfigSource } from "../scripts/runtime-config.mjs";

test("runtime API configuration is emitted at container startup", () => {
  assert.equal(
    runtimeConfigSource("http://100.80.75.9:13000"),
    'window.__FLUCTLIGHT_RUNTIME_CONFIG__ = Object.freeze({ apiOrigin: "http://100.80.75.9:13000/" });\n',
  );
});

test("runtime API configuration rejects non-HTTP URLs", () => {
  assert.throws(() => runtimeConfigSource("ftp://api.invalid"), /must use http or https/);
});
