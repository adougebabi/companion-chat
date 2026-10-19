import type {BrowserClient,BrowserMomentCommentBody,BrowserMomentReactionBody,BrowserMomentStatusBody} from '@fluctlight/browser-client';
// @ts-expect-error explicit v3 domain registration
import {configureDomainService} from '../proto/services.js';
export function installMomentsProvider(client:BrowserClient){const moments={
 list:(options:Parameters<BrowserClient['momentFeed']>[0])=>client.momentFeed(options),
 record:(id:string)=>client.momentRecord(id),
 npcInteractions:(id:string,cursor='')=>client.momentNPCInteractions(id,cursor),
 npcInteraction:(id:string,interaction:string)=>client.momentNPCInteraction(id,interaction),
 retryNPC:(id:string,interaction:string,body:BrowserMomentStatusBody)=>client.retryMomentNPCInteraction(id,interaction,body),
 suggestions:(fid='',cursor='')=>client.momentSuggestions(fid,cursor),
 comments:(id:string,cursor='')=>client.momentComments(id,cursor),
 publish:(body:Parameters<BrowserClient['createMoment']>[0])=>client.createMoment(body),
 suggest:(fid:string,body:Parameters<BrowserClient['suggestMoment']>[1])=>client.suggestMoment(fid,body),
 comment:(id:string,body:BrowserMomentCommentBody)=>client.writeMomentComment(id,body),
 reaction:(id:string,body:BrowserMomentReactionBody)=>client.writeMomentReaction(id,body),
 status:(id:string,action:'hide'|'restore',body:BrowserMomentStatusBody)=>client.writeMomentStatus(id,action,body),
 read:(fid:string)=>client.markMomentsRead(fid),
};configureDomainService('moments',moments);return moments;}
