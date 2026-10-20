export function normalizeRuntimeCatalog(value){
 return {...value,agents:Array.isArray(value.agents)?value.agents:[],capabilities:(Array.isArray(value.capabilities)?value.capabilities:[]).map(c=>({...c,name:c.name??c.Name,version:c.version??c.Version,type:c.type??c.Type,description:c.description??c.Description,sourceContract:c}))};
}
