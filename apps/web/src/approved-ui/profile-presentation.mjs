const object=value=>value&&typeof value==='object'&&!Array.isArray(value);
const equal=(a,b)=>JSON.stringify(a)===JSON.stringify(b);
// Evolution retains flat traits alongside their canonical carriers. Display an
// exact alias once, but retain conflicting values and unknown extensions.
export function semanticEntries(value){
  if(!object(value))return [];
  return Object.entries(value).filter(([key,item])=>!['traits','expression'].some(carrier=>object(value[carrier])&&Object.hasOwn(value[carrier],key)&&equal(value[carrier][key],item)));
}
function different(value,shared){
  if(equal(value,shared))return undefined;
  if(object(value)&&object(shared)){
    const entries=Object.entries(value).map(([key,item])=>[key,different(item,shared[key])]).filter(([,item])=>item!==undefined);
    return entries.length?Object.fromEntries(entries):undefined;
  }
  return value;
}
export function personaPresentation(profile,common){
  const result={...profile};
  for(const key of ['identity','personality','behavioral_policy','life_profile']){
    if(!Object.hasOwn(profile,key))continue;
    const value=different(profile[key],common[key]);
    if(value===undefined)delete result[key];else result[key]=value;
  }
  return result;
}
export function workingPersonaPresentation(value,depth=0){
  if(!object(value)||!object(value.working_persona)||depth>=7)return value;
  const {working_persona,...metadata}=value,body=workingPersonaPresentation(working_persona,depth+1);
  if(Object.keys(body).some(key=>Object.hasOwn(metadata,key)&&!equal(metadata[key],body[key])))return value;
  return {...metadata,...body};
}
const portraitKeys=new Set(['openness','conscientiousness','extraversion','agreeableness','neuroticism','curiosity','independence','patience','empathy','assertiveness','humor','sociability','risk_tolerance','message_length','emoji_frequency','sarcasm_tendency','directness','initiative','topic_initiation','silence_tolerance','response_delay','emotional_expression','response_style','punctuation_style','humor_style','conflict_style','refusal_style','intimacy_expression']);
// Blank portraits persist Core's `key: value` lines. Interpret only declared
// scalar keys, leaving model/user prose intact and never changing stored text.
// Optional counts describe equal rows within this one text, never shared sources.
export function portraitPresentation(text,{includeOccurrences=false}={}){
  const seen=new Map();
  return String(text).split('\n').flatMap(line=>{
    const match=/^([a-z_]+): (.+)$/.exec(line);
    if(!match||!portraitKeys.has(match[1]))return [{text:line}];
    const raw=match[2],value=/^\d+(?:\.\d+)?$/.test(raw)?Number(raw):raw;
    const key=JSON.stringify([match[1],value]);
    if(seen.has(key)){if(includeOccurrences)seen.get(key).occurrences++;return [];}
    const row=includeOccurrences?{field:match[1],value,occurrences:1}:{field:match[1],value};
    seen.set(key,row);return [row];
  });
}
