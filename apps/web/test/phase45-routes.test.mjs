import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(path, import.meta.url), "utf8");
const app = read("../src/App.vue");
const studio = read("../src/views/StudioView.vue");
const world = read("../src/views/WorldView.vue");
const moments = read("../src/views/MomentsView.vue");
const autonomy = read("../src/views/AutonomyView.vue");
const dataTools = read("../src/views/DataToolsView.vue");
const rollback = read("../src/views/RollbackView.vue");
const pending = read("../src/views/PendingView.vue");
const diagnostics = read("../src/views/DiagnosticsView.vue");
const audit = read("../../../docs/audit-feature-entries.md");
const missing = read("../../../docs/backend-missing-apis.md");
const migration = read("../../../docs/migration-correspondence.md");
const access = read("../../../docs/access-status.md");

test("phase 4-5 routes replace studio and world placeholders and keep diary unwired", () => {
  assert.match(app, /id="v3-root"/);
  assert.match(app, /mountProduct/);
  assert.match(studio, /创作预览·不进入生活/);
  assert.match(rollback, /未提供/);
  assert.doesNotMatch(app, /<GovernanceView|<StudioView|<UnwiredView/);
  assert.doesNotMatch(app, /iframe|prototype-v3/);
});

test("legacy urls replace the address without dropping the resolved view", () => {
  assert.match(app, /path === "\/instances"/);
  assert.match(app, /\/library\$\{search\}/);
  assert.match(app, /path === "\/moments"/);
  assert.match(app, /\/feed\$\{search\}/);
  assert.match(app, /path === "\/settings\/diagnostics"/);
  assert.match(app, /\/diagnostics\$\{search\}/);
  assert.match(app, /history\.replaceState/);
  assert.match(app, /path\.startsWith\("\/moments"\) \|\| path\.startsWith\("\/feed"\)/);
  assert.match(app, /path\.startsWith\("\/settings\/diagnostics"\)/);
});

test("studio preview stays local and world browsing does not move the character", () => {
  assert.match(studio, /创作预览·不进入生活/);
  assert.match(studio, /没有写入角色媒体、日记、记忆或生活事实/);
  assert.match(studio, /ChatPanel/);
  assert.doesNotMatch(studio, /useConversationStore|controlCenter\.|store\.send\(/);
  assert.match(world, /浏览世界不会移动角色/);
  assert.match(world, /地图/);
  assert.match(world, /尚未接入/);
  assert.doesNotMatch(world, /cancelLifeEvent|createLifeEvent|setLifePresence/);
});

test("moments, autonomy, data tools and pending keep honest unwired states", () => {
  assert.match(moments, /生成图·非事实/);
  assert.match(moments, /不能以角色身份代发动态/);
  assert.match(moments, /reactToMoment/);
  assert.doesNotMatch(moments, /createMoment/);
  assert.match(autonomy, /OperationalSettings/);
  assert.match(autonomy, /governAutonomyAction/);
  assert.match(autonomy, /免打扰时段尚未接入/);
  assert.match(autonomy, /花费上限尚未接入/);
  assert.match(autonomy, /主动触达频率尚未接入/);
  assert.match(dataTools, /配置包/);
  assert.match(dataTools, /业务数据备份/);
  assert.match(dataTools, /时间线回档/);
  assert.match(rollback, /未提供/);
  assert.match(rollback, /日记/);
  assert.match(rollback, /记忆/);
  assert.match(rollback, /关系/);
  assert.doesNotMatch(rollback, /<button[^>]*>[^<]*恢复/);
  assert.match(pending, /本页不就地批准/);
  assert.match(pending, /不把失败显示成 0 条/);
  assert.doesNotMatch(pending, /reviewCapabilityRequest/);
});

test("diagnostics keeps filters, export, paging and confirms dangerous workflow commands", () => {
  assert.match(diagnostics, /<FilterPanel/);
  assert.match(diagnostics, /导出当前结果/);
  assert.match(diagnostics, /loadMoreDiagnosticRuns/);
  assert.match(diagnostics, /查看 Model Run/);
  assert.match(diagnostics, /永久伤害事件拦截计数尚未接入/);
  assert.match(diagnostics, /不提供放行/);
  assert.match(diagnostics, /controlCenter\.commandWorkflow\("cancel"\)/);
  assert.match(diagnostics, /controlCenter\.restartWorkflow\(\)/);
  assert.match(diagnostics, /controlCenter\.resetWorkflow\(\)/);
  assert.match(diagnostics, /确认取消/);
  assert.doesNotMatch(diagnostics, /放行按钮|releaseHarm/);
});

test("phase documents cover migration, access and all 53 audit features", () => {
  const ids = [...audit.matchAll(/^\| ((?:SYS|INS|CHAT|MOM|DET|GOV|SET|DIA|GAP)-\d{2}) \|/gm)].map((match) => match[1]);
  assert.equal(new Set(ids).size, 53);
  assert.equal(ids.length, 53);
  assert.match(missing, /B 类补入口/);
  assert.match(missing, /C 类新领域能力/);
  assert.match(missing, /幂等/);
  assert.match(missing, /revision/);
  assert.match(migration, /\/library/);
  assert.match(migration, /GovernanceView/);
  assert.match(access, /尚未接入/);
  assert.match(access, /部分接入/);
  assert.match(access, /已接入/);
});
