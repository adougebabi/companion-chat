import {configurationClient} from '../services.js';
import {persistentCommand} from '../../providers/persistent-command.mjs';
import {studioDraftStorage} from './studio-draft-storage.js';

async function fingerprint(salt,value){const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(salt+'\u001f'+value));return [...new Uint8Array(digest)].map(b=>b.toString(16).padStart(2,'0')).join('');}
// Only command identity, ordinary config and salted credential fingerprints
// survive refresh. Credential plaintext stays in the current input element.
export function configurationCommand({slot,alive,readSecret}){
 const command=persistentCommand({key:slot+':command',storage:studioDraftStorage,execute:async(value,key)=>{
  if(!alive())throw new Error('会话已切换，请回到原配置确认提交');
  try{let receipt=await configurationClient.command(key);if(!alive())throw new Error('会话已切换');if(receipt.wake_rearm_required)receipt=await configurationClient.recover(key);return {command:receipt,recovered:true,secretProofs:value.secretProofs,submittedBody:value.body};}catch(error){if(error.status!==404)throw error;}
  const body=structuredClone(value.body),secrets={};
  for(const [purpose,proof]of Object.entries(value.secretProofs||{})){const plain=readSecret(purpose);if(!plain||await fingerprint(proof.salt,plain)!==proof.hash)throw new Error('请重填原提交的访问凭据，再重试原提交；凭据没有保存在浏览器中。');secrets[purpose]=plain;}
  if(!alive())throw new Error('会话已切换，未发送配置');
  if(Object.keys(secrets).length)body.secrets=secrets;
  body.idempotencyKey=key;
  let response;if(value.operation==='endpoint')response=await configurationClient.endpoint(body);
  else if(value.operation==='binding')response=await configurationClient.binding(body);
  else response=await configurationClient.save(body);
  return {response,secretProofs:value.secretProofs,submittedBody:value.body};
 }});
 return {...command,async matchesSecret(proof,plain){return !!proof&&await fingerprint(proof.salt,plain)===proof.hash;},async submit(operation,body,secrets={}){
  const secretProofs={};for(const [purpose,plain]of Object.entries(secrets)){if(!plain)continue;const salt=crypto.randomUUID();secretProofs[purpose]={salt,hash:await fingerprint(salt,plain)};}
  return command.submit({operation,body:structuredClone(body),secretProofs});
 }};
}
