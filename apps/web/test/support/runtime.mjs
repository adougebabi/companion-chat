export const memoryStorage=()=>{const values=new Map();return {getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,String(v)),removeItem:k=>values.delete(k),values};};
export const tick=()=>new Promise(resolve=>setImmediate(resolve));
