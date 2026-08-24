// Generated from packages/core-client/openapi.json. Do not edit by hand.
export const coreOperations = ["/health/live","/health/ready","/internal/platform/ping"] as const;

export type CoreHealth = { status: string; role: string };

export class CoreClient {
  constructor(private readonly baseUrl: string, private readonly serviceKey: string, private readonly fetcher: typeof fetch = fetch) {}
  async health(path: "/health/live" | "/health/ready"): Promise<CoreHealth> {
    const response = await this.fetcher(new URL(path, this.baseUrl), { headers: { "x-fluctlight-service-key": this.serviceKey } });
    if (!response.ok) throw new Error(`Core health request failed: ${response.status}`);
    return response.json() as Promise<CoreHealth>;
  }
  async ping(): Promise<CoreHealth> {
    const response = await this.fetcher(new URL("/internal/platform/ping", this.baseUrl), { headers: { "x-fluctlight-service-key": this.serviceKey } });
    if (!response.ok) throw new Error(`Core ping failed: ${response.status}`);
    return response.json() as Promise<CoreHealth>;
  }
}
