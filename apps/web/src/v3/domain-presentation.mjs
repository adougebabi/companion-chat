import {enumLabel} from '../lib/fluctlight-display.ts';
const states={reviewing:'审核中',fulfilled:'已交付',missing:'尚未建立',not_configured:'尚未配置',source_unavailable:'来源暂不可用',available:'可使用',initial_saved:'首次保存',idle:'等待新的经历',dead_letter:'重试已用尽，需要处理',retry:'等待重试',planned:'计划',scheduled:'已安排',started:'已开始',running:'执行中',queued:'排队中',ready:'已就绪',failed:'执行失败',no_op:'本次无新增操作',completed_noop:'本轮结束，无新增操作',awaiting_review:'等待人工评审',awaiting_trigger:'等待触发条件',retry_wait:'等待重试',blocked:'暂时受阻',skipped:'已跳过，未完成验证',succeeded:'已完成',processing:'处理中',due:'待执行',in_progress:'正在执行',qualified:'已具备执行条件',not_required:'本次无需执行',vision_disabled:'视觉检验未启用',goal_created:'目标已创建','goal created':'目标已创建',goal_updated:'目标已更新','goal updated':'目标已更新',source_review:'初始化来源审查',reviewed:'已审查',retained:'已保留',rejected:'未采纳',adopted:'已采纳',accepted:'已接受',pending:'等待处理'};
const kinds={cognition:'回应处理',wardrobe:"穿搭准备",cognitive_assessment:'认知分析',media_prompt:'图像提示整理',takeover_judge:'人格接管判断',visual_identity_patch:'视觉身份修正',initialization:'人物初始化',visual_identity:'形象准备',schedule:'日程安排',wake_up:'日常唤醒',reflection:'回顾与自我认识',goal:'目标评估',goal_planner:'目标规划',media:'媒体生成',conversation:'交流处理',conversation_reply:'交流表达',generic_llm:'认知与通用分析',embedding:'记忆向量',vision_understanding:'视觉理解'};
const names={'schedule.owner':'日程安排请求','wardrobe.initialize':'初始穿搭准备',moment_suggestion:'动态发布建议',media_generate:'图片生成请求',item_photo:'物品图片请求',wardrobe_wear:'换装请求',item_use:'物品使用请求',schedule_item_suggestion:'日程建议',promise:'约定提议',promise_reschedule:'约定改期',inventory_borrow:'物品借入请求',inventory_return:'物品归还请求',inventory_remove:'脱下衣物请求',inventory_outfit:'穿搭应用请求','schedule.current_day':'今日日程准备','wake_up.current':'日常唤醒','reflection.run':'回顾与自我认识','goal.evaluate':'目标进展评估','goal.plan':'独立目标规划','visual_identity.initialize':'人物形象准备','media.generate':'媒体生成'};
const phases={dispatched:'已交给执行器',activity_started:'开始执行',completed:'本轮已完成',completed_noop:'本轮结束，无新增操作',failed:'执行失败',cancelled:'已取消',retry_scheduled:'已安排重试',started:'已开始',queued:'排队中',continued:'继续处理',scheduled:'已安排',paused:'已暂停',resumed:'已恢复'};
export function stateLabel(value,fallback='状态待确认'){
 if(!value)return fallback;
 if(states[value])return states[value];
 if(typeof value==='string'&&states[value.toLowerCase()])return states[value.toLowerCase()];
 const common=enumLabel(value);if(common!=='未分类')return common;
 if(typeof value==='string'){const normalized=enumLabel(value.toLowerCase());if(normalized!=='未分类')return normalized;}
 return /[\u3400-\u9fff]/u.test(String(value))?String(value):fallback;
}
export function domainName(value,fallback='运行记录'){
 if(!value)return fallback;
 if(names[value])return names[value];if(kinds[value])return kinds[value];
 if(states[value])return states[value];
 if(typeof value==='string'&&states[value.toLowerCase()])return states[value.toLowerCase()];
 const parts=String(value).replace(/^lifecycle\./,'').split('.');
 if(kinds[parts[0]])return [kinds[parts[0]],phases[parts.slice(1).join('.')]||phases[parts.at(-1)]].filter(Boolean).join(' · ');
 return /[\u3400-\u9fff]/u.test(String(value))?String(value):fallback;
}

const visualStages={session_created:'生成任务已登记',seed_requested:'正在整理外观设定',seed_ready:'外观设定已整理',image_requested:'正在生成候选图',image_ready:'候选图已生成',vision_requested:'正在检查形象一致性',vision_ready:'形象检查已返回',patch_requested:'正在整理修正',patch_ready:'修正内容已准备',regenerate:'正在重新生成',accepted:'规范参考图已接受',character_sheet_requested:'正在生成角色设定图',character_sheet_ready:'角色设定图已生成',completed:'视觉准备已完成',failed:'本次生成失败'};
export function visualStageLabel(value,fallback='生成阶段记录'){return visualStages[value]||fallback;}

// A user-facing summary only. Callers retain the original value in technical details.
export function errorMessage(error, fallback='操作未完成，请稍后重试。') {
 const text=String(error?.message ?? error ?? ''), code=String(error?.code ?? '');
 const source=code+' '+text;
 if(error?.status===401||/\b401\b|invalid_password|invalid_credentials/.test(source))return '登录未成功，请检查密码并重新登录。';
 if(error?.status===403||/\b403\b|forbidden/.test(source))return '当前账户没有执行此操作的权限。';
 if(error?.status===404||/resource not found|could not be read|not_found/.test(source))return '没有找到这条记录，请返回列表核对。';
 if(error?.status===409||/current_facts_stale|actor_context_stale|version_conflict|revision_conflict|resource revision conflict/.test(source))return '资料版本已变化，草稿已保留，请重新读取后核对。';
 if(/prompt_(?:required_|context_|tool_result_)?budget_exceeded/.test(source))return '要读取的资料超过模型输入上限，请在模型设置核对可用上下文。';
 if(/adk_final_(?:output|repair)|structured_response_invalid/.test(source))return '模型返回的内容格式不完整，请检查模型服务后重试。';
 if(/context canceled|context_cancelled|request_cancelled/.test(source))return '本次处理已中断，请刷新进度确认结果。';
 if(/request_timeout|context deadline exceeded|timed? ?out|TimeoutError/.test(source))return '模型服务响应超时。原消息和提交记录已保留，请刷新进度或检查模型服务。';
 if(/request_failed|Failed to fetch|NetworkError|fetch failed|unavailable/.test(source))return '服务暂时无法连接，请稍后重新读取。';
 if(/[\u3400-\u9fff]/u.test(text))return text.replace(/\b[a-z][a-z0-9]*_[a-z0-9_]+\b/gi,'（详情见技术记录）');
 return fallback;
}
