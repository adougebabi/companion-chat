import {useState} from 'react';
import {EarthIcon} from 'lucide-react';
import {AspectRatio} from '@/components/ui/aspect-ratio';
export default function WorldScenePreview({world}:any){
 const place=(world.sites||[]).flatMap((site:any)=>site.places||[]).find((place:any)=>typeof place.scene==='string'&&place.scene.trim());
 const src=place?.scene||'', [failed,setFailed]=useState('');
 if(!src||failed===src)return <div className="flex h-20 items-center justify-center gap-3 bg-muted text-sm text-muted-foreground"><EarthIcon className="size-5"/><span>{src?'场景暂不可用':'生活世界'}</span></div>;
 return <AspectRatio ratio={16/9} className="overflow-hidden bg-muted"><div className="flex h-full w-full items-center justify-center">{src&&failed!==src?<img src={src} alt={(place.alias||place.name||'地点')+'的场景'} className="h-full w-full object-cover" loading="lazy" onError={()=>setFailed(src)}/>:<div className="grid justify-items-center gap-2 text-muted-foreground"><EarthIcon className="size-8" aria-hidden="true"/><span className="text-xs">{src?'地点场景暂不可用':'尚未配置场景图片'}</span></div>}</div></AspectRatio>;
}
