function hex(value: number): string {
  return value.toString(16).padStart(2, "0");
}

export function randomId(): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }
  if (typeof globalThis.crypto?.getRandomValues !== "function") {
    throw new Error("This browser does not provide secure random ID generation");
  }
  const bytes = globalThis.crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const values = Array.from(bytes, hex);
  return `${values.slice(0, 4).join("")}-${values.slice(4, 6).join("")}-${values.slice(6, 8).join("")}-${values.slice(8, 10).join("")}-${values.slice(10).join("")}`;
}
