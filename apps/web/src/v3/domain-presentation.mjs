import {enumLabel} from '../lib/fluctlight-display.ts';
const states={started:'已开始',running:'执行中',queued:'排队中',ready:'已就绪',failed:'执行失败',no_op:'本次无新增操作',completed_noop:'本轮结束，无新增操作',awaiting_review:'等待人工评审',awaiting_trigger:'等待触发条件',retry_wait:'等待重试',blocked:'暂时受阻',skipped:'已跳过，未完成验证',succeeded:'已完成',processing:'处理中',due:'待执行',in_progress:'正在执行',qualified:'已具备执行条件',not_required:'本次无需执行',vision_disabled:'视觉检验未启用',goal_created:'目标已创建','goal created':'目标已创建',goal_updated:'目标已更新','goal updated':'目标已更新',source_review:'初始化来源审查',reviewed:'已审查',retained:'已保留',rejected:'未采纳',adopted:'已采纳',accepted:'已接受',pending:'等待处理'};
const kinds={initialization:'人物初始化',visual_identity:'形象准备',schedule:'日程安排',wake_up:'日常唤醒',reflection:'回顾与自我认识',goal:'目标评估',goal_planner:'目标规划',media:'媒体生成',conversation:'交流处理',conversation_reply:'交流表达',generic_llm:'认知与通用分析',embedding:'记忆向量',vision_understanding:'视觉理解'};
const names={'schedule.current_day':'今日日程准备','wake_up.current':'日常唤醒','reflection.run':'回顾与自我认识','goal.evaluate':'目标进展评估','goal.plan':'独立目标规划','visual_identity.initialize':'人物形象准备','media.generate':'媒体生成'};
const phases={dispatched:'已交给执行器',activity_started:'开始执行',completed:'本轮已完成',completed_noop:'本轮结束，无新增操作',failed:'执行失败',cancelled:'已取消',retry_scheduled:'已安排重试',started:'已开始',queued:'排队中',continued:'继续处理',scheduled:'已安排',paused:'已暂停',resumed:'已恢复'};
export function stateLabel(value,fallback='状态待确认'){
 if(!value)return fallback;
 if(states[value])return states[value];
 const common=enumLabel(value);if(common!=='未分类')return common;
 return /[\u3400-\u9fff]/u.test(String(value))?String(value):fallback;
}
export function domainName(value,fallback='运行记录'){
 if(!value)return fallback;
 if(names[value])return names[value];if(kinds[value])return kinds[value];
 if(states[value])return states[value];
 const parts=String(value).replace(/^lifecycle\./,'').split('.');
 if(kinds[parts[0]])return [kinds[parts[0]],phases[parts.slice(1).join('.')]||phases[parts.at(-1)]].filter(Boolean).join(' · ');
 return /[\u3400-\u9fff]/u.test(String(value))?String(value):fallback;
}
