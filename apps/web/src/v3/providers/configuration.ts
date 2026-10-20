import type {BrowserClient,BrowserConfigurationWrite} from '@fluctlight/browser-client';
import {parseWorkflowTemplate} from '../../lib/configuration-forms';
// @ts-expect-error shared browser delivery observes real notification records
import {installBrowserNotifications,browserNotificationState} from './browser-notifications.mjs';
// @ts-expect-error v3 domain facade
import {configureDomainService} from '../proto/services.js';
// @ts-expect-error shared runtime catalog normalization
import {normalizeRuntimeCatalog} from '../runtime-catalog.mjs';
export function installConfigurationProvider(client:BrowserClient,refresh:()=>Promise<unknown>){const configuration={
 notificationState:browserNotificationState,
 read:()=>client.configurationCatalog(),
 settings:()=>client.settings(),
 command:(key:string)=>client.configurationCommandByKey(key),
 recover:(key:string)=>client.recoverConfigurationCommand(key),
 cancelUnaccepted:(key:string)=>client.cancelUnacceptedConfigurationCommand(key),
 save:(body:BrowserConfigurationWrite)=>client.updateSettings(body),
 endpoints:()=>client.providerEndpoints(),
 bindings:()=>client.providerBindings(),
 endpoint:(body:Parameters<BrowserClient['configureProviderEndpoint']>[0])=>client.configureProviderEndpoint(body),
 binding:(body:Parameters<BrowserClient['configureModelRole']>[0])=>client.configureModelRole(body),
 models:(id:string)=>client.providerEndpointModels(id),
 catalog:async()=>normalizeRuntimeCatalog(await client.runtimeCapabilityCatalog()),
 test:(kind:'kev'|'vision'|'media')=>kind==='kev'?client.testKevConnection():kind==='vision'?client.testVisionUnderstanding():client.testSavedMediaConnection(),
 parseWorkflow:parseWorkflowTemplate,
 async password(value:string){await client.changePassword(value);await refresh();},
 async logout(){await client.logout();await refresh();},
 async revoke(){await client.revokeAllSessions();await refresh();},
};configureDomainService('configuration',configuration);installBrowserNotifications(client);return configuration;}
