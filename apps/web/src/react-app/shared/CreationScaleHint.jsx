import { CircleHelp } from "lucide-react";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";

// These are qualitative descriptions of existing 0–1 fields, not promises
// that every future message will follow a particular numerical probability.
export const creationScaleHints = {
  openness: ["偏稳妥熟悉", "愿意探索新事物"],
  conscientiousness: ["偏随性", "重视计划与责任"],
  extraversion: ["偏内敛", "偏外向"],
  agreeableness: ["重视独立立场", "偏温和合作"],
  neuroticism: ["情绪较稳定", "感受较敏锐"],
  curiosity: ["偏熟悉事物", "好奇心较强"],
  independence: ["重视共同决定", "偏独立自主"],
  patience: ["偏及时行动", "比较耐心"],
  empathy: ["重视事实判断", "比较细腻体贴"],
  assertiveness: ["偏含蓄表达", "明确表达立场"],
  humor: ["表达较认真", "表达较幽默"],
  sociability: ["偏少而深入的交往", "比较乐于交往"],
  risk_tolerance: ["偏谨慎", "比较愿意尝试"],
  emoji_frequency: ["很少使用表情", "更常使用表情"],
  sarcasm_tendency: ["很少调侃", "更倾向调侃"],
  directness: ["表达较委婉", "表达更直接"],
  initiative: ["更多回应对方", "更主动互动"],
  topic_initiation: ["较少开启话题", "更常开启话题"],
  silence_tolerance: ["较难接受沉默", "更能接受沉默"],
  emotional_expression: ["表达较克制", "情绪表达更明显"],
  importance: ["不太重要", "非常重要"],
  urgency: ["不太紧迫", "非常紧迫"],
  confidence: ["把握较小", "把握较大"],
  familiarity: ["不太熟悉", "非常熟悉"],
  trust: ["信任较少", "信任较多"],
  affection: ["好感较少", "好感较多"],
};

export default function CreationScaleHint({ field, label }) {
  const ends = creationScaleHints[field];
  if (!ends) return null;
  return <TooltipProvider delayDuration={250}>
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" className="creation-scale-info" aria-label={`了解${label}的数值含义`}>
          <CircleHelp size={15} aria-hidden="true" />
        </button>
      </TooltipTrigger>
      <TooltipContent className="max-w-80 break-words">
        0：{ends[0]}；1：{ends[1]}。中间值表示程度介于两者之间，实际表现会随情境变化。
      </TooltipContent>
    </Tooltip>
  </TooltipProvider>;
}

export function CreationScaleEnds({ field }) {
  const ends = creationScaleHints[field];
  return ends ? <div className="creation-scale-ends"><span>0 · {ends[0]}</span><span>1 · {ends[1]}</span></div> : null;
}
