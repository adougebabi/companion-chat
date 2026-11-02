// Render only the confirmed wardrobe projection shared by chat and life.
export default function WearingSummary({character,error='',loading=false}:{character:any;error?:string;loading?:boolean}){
 const worn=character._wardrobe?.wearing?.worn_items;
 return <div data-testid="current-outfit">{error&&<p role="status">穿着读取未完成，保留上次确认的记录。</p>}{loading&&<p role="status">正在读取实际穿着…</p>}{Array.isArray(worn)&&worn.length?<ul className="grid gap-1" aria-label="当前穿着的衣物">{worn.map((item:any,index:number)=><li key={item.item_id||item.id||index}>{item.description||character.garments?.find((garment:any)=>garment.id===(item.item_id||item.id))?.name||'衣物名称待读取'}</li>)}</ul>:<p>{Array.isArray(worn)?'还没有登记实际穿着。':'实际穿着尚未读到。'}</p>}</div>;
}
