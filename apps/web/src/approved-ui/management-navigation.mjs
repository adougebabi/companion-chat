import {Server,MessageSquare,ImageIcon,Leaf,Bell,Activity,Database,SlidersHorizontal} from 'lucide-react';

export const managementCategories = [
    {id:'models',label:'模型服务与用途',icon:Server,sections:['服务地址','模型用途']},
    {id:'conversation',label:'聊天与视觉',icon:MessageSquare,sections:['聊天回复','看图理解']},
    {id:'media',label:'图片生成',icon:ImageIcon,sections:['图片（ComfyUI）']},
    {id:'plugins',label:'能力插件',icon:SlidersHorizontal,sections:['能力插件']},
    {id:'autonomy',label:'自主性与安静时段',icon:Leaf,sections:['角色自主与主动联系','安静时段']},
    {id:'runtime',label:'通知与后台',icon:Bell,sections:['通知','作息与后台节奏','资源与预算']},
    {id:'diagnostics',label:'诊断与任务',icon:Activity,sections:[]},
    {id:'data',label:'数据管理',icon:Database,sections:[]},
    {id:'advanced',label:'高级设置',icon:SlidersHorizontal,sections:['高级']},
];

export function managementCategoryHref(item) {
  if (item.id === 'data') return '#/data';
  if (item.id === 'diagnostics') return '#/diag?view=tasks';
  return '#/settings?section=' + encodeURIComponent(item.sections[0]);
}
