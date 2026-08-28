import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const navigationSource = await readFile(new URL("../src/app/navigation.ts", import.meta.url), "utf8");
const stylesSource = await readFile(new URL("../src/styles/app.css", import.meta.url), "utf8");
const shellSource = await readFile(new URL("../src/components/layout/AppShell.vue", import.meta.url), "utf8");
const appSource = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
const instancesSource = await readFile(new URL("../src/views/InstancesView.vue", import.meta.url), "utf8");
const bottomNavSource = await readFile(new URL("../src/components/layout/BottomNav.vue", import.meta.url), "utf8");
const desktopContextSource = await readFile(new URL("../src/components/layout/DesktopContextPanel.vue", import.meta.url), "utf8");
const settingsSource = await readFile(new URL("../src/views/SettingsView.vue", import.meta.url), "utf8");
const statusSource = await readFile(new URL("../src/lib/fluctlight-status.ts", import.meta.url), "utf8");
const viteSource = await readFile(new URL("../vite.config.ts", import.meta.url), "utf8");
const diagnosticsSource = await readFile(new URL("../src/views/DiagnosticsView.vue", import.meta.url), "utf8");
const controlCenterSource = await readFile(new URL("../src/stores/control-center.ts", import.meta.url), "utf8");

test("Telegram-style workspace keeps four primary tabs", () => {
  assert.match(navigationSource, /id: "instances"/);
  assert.match(navigationSource, /id: "moments"/);
  assert.match(navigationSource, /id: "settings"/);
  assert.match(navigationSource, /id: "diagnostics"/);
  assert.doesNotMatch(navigationSource, /id: "chat"/);
  assert.match(navigationSource, /primaryNavigation[\s\S]*id: "instances"[\s\S]*id: "moments"[\s\S]*id: "settings"[\s\S]*id: "diagnostics"/);
  assert.match(shellSource, /activeView !== 'chat'/);
  assert.match(appSource, /correlation_id/);
  assert.match(bottomNavSource, /activeNavigationView = computed<WorkspaceView>/);
  assert.doesNotMatch(bottomNavSource, /props\.activeView === "diagnostics" \? "settings"/);
  assert.match(desktopContextSource, /:active-view="props\.activeView"/);
  assert.doesNotMatch(instancesSource, /mobile-instance-search|mobile-archived-row|instance-state/);
  assert.match(instancesSource, /fluctlightStatusLabel\(fluctlight\.status\)/);
  assert.match(statusSource, /active: "运行中"/);
  assert.match(statusSource, /paused: "已暂停"/);
  assert.match(statusSource, /retired: "已归档"/);
  assert.doesNotMatch(instancesSource, /全部实例/);
});

test("settings configuration is progressively disclosed by closed drawers", () => {
  assert.equal((settingsSource.match(/<AccordionItem[^>]*class="settings-section settings-drawer"/g) ?? []).length, 6);
  assert.equal((settingsSource.match(/<AccordionTrigger[^>]*settings-drawer-summary/g) ?? []).length, 6);
  assert.doesNotMatch(settingsSource, /<Accordion[^>]*defaultValue/);
  assert.match(settingsSource, /id="endpoint-secret"/);
  assert.match(settingsSource, /id="owner-password"/);
  assert.match(settingsSource, /服务接入/);
  assert.match(settingsSource, /模型参数/);
});

test("web controls are backed by the local shadcn-vue component layer", () => {
  assert.match(viteSource, /@tailwindcss\/vite/);
  assert.match(viteSource, /alias:[\s\S]*\"@\"/);
  assert.match(bottomNavSource, /@\/components\/ui\/button\/Button\.vue/);
  assert.match(settingsSource, /<Accordion/);
  assert.match(settingsSource, /<Tabs/);
  assert.match(settingsSource, /<Select/);
  assert.match(settingsSource, /<Textarea/);
});

test("diagnostics is a quiet, recent-only disclosure surface", () => {
  assert.doesNotMatch(diagnosticsSource, /返回设置|刷新|导出|清空|Correlation ID|按 Correlation ID 过滤|筛选|诊断记录已刷新/);
  assert.match(diagnosticsSource, /<Accordion/);
  assert.match(diagnosticsSource, /value="model-runs"/);
  assert.match(controlCenterSource, /client\.diagnostics\(\{ limit: 20/);
  assert.match(controlCenterSource, /client\.diagnosticModelRuns\(\{ limit: 20/);
});

test("chat owns a fixed-height work surface with an internal message scroller", () => {
  assert.match(stylesSource, /\.app-shell\.chat-shell[^}]*height: calc\(100dvh - 32px\)/);
  assert.match(stylesSource, /\.app-shell\.chat-shell[^}]*padding-bottom: 0/);
  assert.match(stylesSource, /\.chat-page[^}]*height: 100%[^}]*min-height: 0/);
  assert.match(stylesSource, /\.message-timeline[^}]*min-height: 0[^}]*overflow-y: auto/);
});

test("mobile surfaces keep diagnostics, navigation, and empty states shrink-safe", () => {
  assert.match(stylesSource, /\.diagnostics-page \.page-header\s*\{[\s\S]*display: grid/);
  assert.match(stylesSource, /\.diagnostics-page \.header-actions\s*\{[\s\S]*position: static/);
  assert.match(stylesSource, /\.diagnostics-page \.diagnostic-meta\s*\{[\s\S]*grid-template-columns: minmax\(0, 1fr\) auto/);
  assert.match(stylesSource, /\.bottom-nav-item\s*\{[\s\S]*display: grid/);
  assert.match(stylesSource, /\.empty-state\s*\{[\s\S]*place-items: center/);
  assert.match(stylesSource, /\.field-note\s*\{[\s\S]*line-height: 1\.5/);
  assert.match(stylesSource, /\.back-link\s*\{[\s\S]*border: 0/);
});
