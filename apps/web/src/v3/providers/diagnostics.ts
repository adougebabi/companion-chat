import type {BrowserClient,BrowserDiagnosticClearBody,BrowserWorkflowControlBody,BrowserDiagnosticMediaRecoveryBody} from '@fluctlight/browser-client';
// @ts-expect-error explicit v3 domain facade
import {configureDomainService} from '../proto/services.js';
export function installDiagnosticsProvider(client:BrowserClient){const diagnostics={
 workflow:(id:string)=>client.ownerWorkflowInspection(id),
 history:(id:string,run:string,cursor='')=>client.ownerWorkflowHistoryPage(id,run,cursor),
 control:(id:string,action:'pause'|'resume'|'cancel'|'restart'|'reset',body:BrowserWorkflowControlBody)=>client.ownerWorkflowControl(id,action,body),
 retryControl:(id:string,command:string)=>client.retryOwnerWorkflowCommand(id,command),
 harm:()=>client.ownerHarmSummary(),
 page:(kind:string,options:Record<string,string|number>={})=>client.ownerDiagnosticRecords(kind,options),
 record:(kind:string,ref:string)=>client.ownerDiagnosticRecord(kind,ref),
 export:(options:Record<string,string>={})=>client.streamDiagnosticExport(options),
 clear:(body:BrowserDiagnosticClearBody)=>client.clearOwnerDiagnosticRecords(body),
 mediaRetry:(id:string,body:BrowserDiagnosticMediaRecoveryBody)=>client.retryOwnerDiagnosticMedia(id,body),
};configureDomainService('diagnostics',diagnostics);return diagnostics;}
