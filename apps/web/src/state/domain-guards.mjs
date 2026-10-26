export function readonlyCharacter(workspace,fid){const row=workspace.state.characters.find(value=>value.id===fid);return !row||['已退休','已归档','retired','archived'].includes(row.status)||['retired','archived'].includes(row._directory?.status);}
export function requireWritableCharacter(workspace,fid){if(readonlyCharacter(workspace,fid))throw Error('当前角色不可写入，请回到角色库核对状态。');}
