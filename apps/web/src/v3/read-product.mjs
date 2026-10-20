// Each independent read retains its own failure/unavailable state. Authentication
// and the owned character directory are required and never degrade to demo data.
export async function readProduct(client, {initialOnly=false}={}) {
  const session = await client.session();
  if (!session.authenticated) return { session, fluctlights: [] };
  const bundle = { session, owner: { id: session.actorId }, fluctlights: await client.listFluctlights(), surfaces: {} };
  const sections = { details: 'detail', wardrobes: 'wardrobe', sidebars: 'sidebar', media: 'listCharacterMedia', diaries: 'listDiary', nearby: 'nearby', worlds: 'world', templates: 'worldTemplates', desires: 'desires', reflections: 'listReflections', autonomy: 'autonomyPolicy', conversations: 'directConversation', goals: 'goals' };
  const globals = { groups: 'listActorGroups', notifications: 'listNotifications', unread: 'unreadNotificationCount', pending: 'pending', pendingCount: 'pendingCount', resources: 'listStudioResources', previews: 'listStudioPreviews', settings: 'settings', backups: 'listBackups' };
  for(const name of Object.keys(sections)){bundle[name]={};for(const item of bundle.fluctlights)bundle.surfaces[`${item.id}:${name}`]={status:'loading',message:'此项资料正在读取，可继续操作'};}
  for(const name of Object.keys(globals))bundle.surfaces[`global:${name}`]={status:'loading',message:'此项资料正在读取，可继续操作'};
  if(initialOnly)return bundle;
  async function read(scope, name, call, target = bundle, field = name) {
    const stateKey = `${scope}:${name}`;
    try {
      const value = await call();
      target[field] = value;
      bundle.surfaces[stateKey] = value?.available === false || value?.status === 'not_implemented'
        ? { status: 'unavailable', code: value.status, message: '此功能尚未实现' }
        : { status: 'ready' };
    } catch (error) {
      // A server authorization/route/transport error is evidence, not empty data.
      bundle.surfaces[stateKey] = { status: error.status === 403 ? 'forbidden' : error.status === 401 ? 'unauthenticated' : 'failed', code: error.code, httpStatus: error.status, message: error.message || '读取失败' };
    }
  }
  await Promise.all(Object.entries(globals).map(([name, method]) => read('global', name, () => client[method]())));
  // Bound fan-out per character. A failure in one optional module cannot erase
  // the directory or another character's successful reads.
  for (const item of bundle.fluctlights) {
    await Promise.all(Object.entries(sections).map(([name, method]) => read(item.id, name, () => client[method](item.id), bundle[name], item.id)));
  }
  await read('global','moments',async()=> (await client.momentFeed({limit:30})).items);
  return bundle;
}
