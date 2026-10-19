import { isMock, get } from './store.js';
export { configurationSections, diagnosticSections } from './service-metadata.js';
const services = isMock ? await Promise.all([import('./mock/client.js'),import('./mock/world.js'),import('./mock/operations.js'),import('./mock/governance.js')]) : null;
const domains = new Map();
/** Real adapters register explicit domain methods; never infer commands from draft mutations. */
export function configureDomainService(domain, implementation) { domains.set(domain, implementation); }
function realService(domain) { return new Proxy({}, { get: (_, action) => (...args) => {
  const implementation = domains.get(domain), operation = implementation?.[action];
  if (typeof operation !== 'function') throw new Error(`此操作尚未连接真实服务：${domain}.${String(action)}。未保存任何修改。`);
  return operation.apply(implementation, args);
} }); }
export const mockClient = services?.[0].mockClient || realService('npc');
export const mockWorld = services?.[1].mockWorld || realService('world');
export const mockOperations = services?.[2].mockOperations || realService('operations');
export const mockGovernance = services?.[3].mockGovernance || realService('governance');
export const sceneOptions = services?.[1].sceneOptions || [{ value: '', label: '未配置场景素材' }, ...['cafe','oldstreet','flowershop','seaside','apartment','shop'].map(name => ({value:`/v3/assets/scenes/${name}.svg`,label:name}))];
export function belongsToWorld(characterId,npc) { return services ? services[0].belongsToWorld(characterId,npc) : npc?.charId === characterId && npc?.worldId === get('characters',characterId)?.worldId; }
export function assertWritableCharacter(id) { if (services) return services[0].assertWritableCharacter(id); throw new Error('此操作尚未连接真实命令'); }

/** Explicit life domain commands; legacy fixture pages continue using their isolated store. */
export const lifeClient = realService('life');
export const memoryClient = realService('memory');
export const reflectionClient = realService('reflection');
export const operationsClient = realService('operations');
export const autonomyClient = realService('autonomy');
export const diaryClient = realService('diary');
export const governanceClient = realService('governance');
export const visualClient = realService('visual');
export const inventoryClient = realService('inventory');
export const selfClient = realService('self');

export const studioClient = realService('studio');
export const mediaClient = realService('media');
export const momentsClient = realService('moments');
export const configurationClient = realService('configuration');

export const diagnosticsClient = realService('diagnostics');

export const dataToolsClient = realService('dataTools');
