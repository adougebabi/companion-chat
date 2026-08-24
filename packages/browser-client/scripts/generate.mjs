import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const schema = JSON.parse(await readFile(new URL("openapi.json", root), "utf8"));
const paths = Object.keys(schema.paths).sort();
const source = `// Generated from packages/browser-client/openapi.json. Do not edit by hand.\nexport const browserOperations = ${JSON.stringify(paths)} as const;\nexport type BrowserHealth = { status: string; role: string };\n`;
await writeFile(fileURLToPath(new URL("src/index.ts", root)), source);
