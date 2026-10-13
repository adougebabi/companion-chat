import { isCustomLabel, enumLabel, formatDisplayValue } from "./fluctlight-display";
function provided(value: unknown): boolean { return value !== null && value !== undefined && value !== "" && (!Array.isArray(value) || value.length > 0); }
function simple(value: unknown): boolean { return typeof value !== "object" || Array.isArray(value) && value.every(item=>typeof item!=='object'); }
export function summaryFields(record: Record<string,unknown>, keys?: string[]): Record<string,unknown> {
  return Object.fromEntries(Object.entries(record).filter(([key,value])=>provided(value)&&simple(value)&&(keys ? keys.includes(key) : !isCustomLabel(key))));
}
export function humanValue(value: unknown): string {
  if (typeof value === "string") {
    const aliases: Record<string,string> = {black:"黑色",brown:"棕色",blonde:"金色",shoulder_length:"齐肩",long:"长发",short_hair:"短发",natural:"自然"};
    if (aliases[value]) return aliases[value];
    const translated=enumLabel(value);if(translated!=='未分类')return translated;
  }
  return formatDisplayValue(value);
}
