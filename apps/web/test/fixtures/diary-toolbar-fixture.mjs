// Read-only fixture shared by DOM and isolated browser layout checks.
export function diaryToolbarWorkspace() {
  const calls = [];
  const w = {
    calls,
    state: {session: {actorId: 'diary-reader'}},
    current: {value: {id: 'A', name: '苏晴', timezone: 'Asia/Shanghai', status: 'active'}},
    isCurrent: (owner, fid) => owner === 'diary-reader' && fid === 'A',
    subscribe: () => () => {},
    client: {
      listDiary: async (_fid, _cursor, options) => {
        calls.push({kind: 'diary', favorite: options.favorite});
        return {items: [], next_cursor: ''};
      },
      sharedExperiences: async () => {
        calls.push({kind: 'shared'});
        return {items: [], next_cursor: ''};
      },
    },
  };
  return w;
}
