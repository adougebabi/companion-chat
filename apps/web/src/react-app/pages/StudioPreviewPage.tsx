import {Button} from '@/components/ui/button';
import {Card,CardContent,CardHeader,CardTitle} from '@/components/ui/card';
import {useWorkspace} from '../page';
import {useCursorPage} from '../hooks/use-cursor-page';
import CursorControls from '../shared/CursorControls';
import StudioEditorPage from './StudioEditorPage';
export default function StudioPreviewPage({route}:{route:any}){const id=route.query.get('res');return id?<StudioEditorPage resourceId={id} jobId={route.query.get('job')||''} sandbox/>:<ResourcePicker/>;}
function ResourcePicker(){const w=useWorkspace(),owner=w.state.session.actorId,page=useCursorPage(cursor=>w.services.studioClient.list({cursor}),JSON.stringify([owner,'studio-preview-picker']));return <Card><CardHeader><CardTitle>选择创作资源</CardTitle></CardHeader><CardContent className="grid gap-4"><p className="text-sm text-muted-foreground">预览使用同一份工作草稿，不进入角色生活。</p>{page.items.map((resource:any)=><Button key={resource.id} type="button" variant="outline" className="justify-start" onClick={()=>w.navigatePath('/studio/preview?res='+encodeURIComponent(resource.id))}>{resource.name}</Button>)}{!page.items.length&&!page.loading&&<p>还没有可用于预览的创作资源。</p>}<CursorControls page={page} label="创作资源"/></CardContent></Card>;}
