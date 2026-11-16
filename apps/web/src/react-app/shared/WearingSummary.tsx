// Render only the confirmed wardrobe projection shared by chat and life.
// A failed read only claims to keep a previous record when one was actually confirmed.
export default function WearingSummary({character,error='',loading=false}:{character:any;error?:string;loading?:boolean}){
 const worn=character._wardrobe?.wearing?.worn_items,confirmed=Array.isArray(worn);
 if(error&&!confirmed)return <div data-testid="current-outfit"><p role="status">暂未读取到穿着</p></div>;
 return <div data-testid="current-outfit">{error&&<p role="status">穿着读取未完成，保留上次确认的记录。</p>}{loading&&<p role="status">正在读取实际穿着…</p>}{confirmed&&worn.length?<ul className="grid gap-1" aria-label="当前穿着的衣物">{worn.map((item:any,index:number)=><li key={item.item_id||item.id||index}>{item.description||character.garments?.find((garment:any)=>garment.id===(item.item_id||item.id))?.name||'衣物名称待读取'}</li>)}</ul>:<p>{confirmed?'还没有登记实际穿着。':'实际穿着尚未读到。'}</p>}</div>;
}
