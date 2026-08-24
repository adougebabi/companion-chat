import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const root = new URL("../", import.meta.url);
const schema = JSON.parse(await readFile(new URL("openapi.json", root), "utf8"));
const operations = Object.keys(schema.paths).sort().join(",");
const source = `// Generated from packages/core-client/openapi.json. Do not edit by hand.\nexport const coreOperations = ${JSON.stringify(operations.split(","))} as const;\n\nexport type CoreHealth = { status: string; role: string };\n\nexport class CoreClient {\n  constructor(private readonly baseUrl: string, private readonly serviceKey: string, private readonly fetcher: typeof fetch = fetch) {}\n  async health(path: \"/health/live\" | \"/health/ready\"): Promise<CoreHealth> {\n    const response = await this.fetcher(new URL(path, this.baseUrl), { headers: { \"x-fluctlight-service-key\": this.serviceKey } });\n    if (!response.ok) throw new Error(\`Core health request failed: \${response.status}\`);\n    return response.json() as Promise<CoreHealth>;\n  }\n  async ping(): Promise<CoreHealth> {\n    const response = await this.fetcher(new URL(\"/internal/platform/ping\", this.baseUrl), { headers: { \"x-fluctlight-service-key\": this.serviceKey } });\n    if (!response.ok) throw new Error(\`Core ping failed: \${response.status}\`);\n    return response.json() as Promise<CoreHealth>;\n  }\n}\n`;
await writeFile(fileURLToPath(new URL("src/index.ts", root)), source);
