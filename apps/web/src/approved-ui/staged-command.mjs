/** Durable immutable stage requests. Results always come from the server. */
export function stagedCommand({ storage, key, live = () => true }) {
  function read() {
    const raw = storage.getItem(key);
    if (!raw) return {};
    const data = JSON.parse(raw);
    if (!data || data.version !== 1 || !data.requests)
      throw Error("扩写提交记录无法读取，请查看原创作任务");
    return data.requests;
  }
  return {
    async stage(name, create, execute) {
      if (!live()) throw Error("账号或角色已变化，请回到原范围继续");
      const requests = read();
      let request = requests[name];
      if (!request) {
        request = {
          body: structuredClone(await create()),
          idempotencyKey: key + ":" + name,
        };
        if (!live()) throw Error("账号或角色已变化");
        requests[name] = request;
        storage.setItem(key, JSON.stringify({ version: 1, requests }));
      }
      if (!live()) throw Error("账号或角色已变化");
      const result = await execute(
        structuredClone(request.body),
        request.idempotencyKey,
      );
      if (!live()) throw Error("账号或角色已变化，请回到原范围核对结果");
      return result;
    },
    finish() {
      storage.removeItem(key);
    },
  };
}
