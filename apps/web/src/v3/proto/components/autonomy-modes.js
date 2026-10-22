export const autonomyModes=[{value:'active',label:'允许自治行动'},{value:'paused',label:'暂停新自治行动'},{value:'muted',label:'屏蔽自治外发'}];
export const autonomyModeLabel=value=>autonomyModes.find(item=>item.value===value)?.label||'自治状态待确认';
