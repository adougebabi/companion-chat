// Render only the confirmed wardrobe projection shared by chat and life.
export default function WearingSummary({character,error='',loading=false}:{character:any;error?:string;loading?:boolean}){
 const worn=character._wardrobe?.wearing?.worn_items;
 const text=Array.isArray(worn)?worn.map((item:any)=>item.description||character.garments?.find((garment:any)=>garment.id===(item.item_id||item.id))?.name||'衣物名称待读取').join('、'):null;
 return <div data-testid="current-outfit">{error&&<p role="status">穿着读取未完成，保留上次确认的记录。</p>}{loading&&<p role="status">正在读取实际穿着…</p>}<p>{text===null?'实际穿着尚未读到。':text||'还没有登记实际穿着。'}</p></div>;
}
