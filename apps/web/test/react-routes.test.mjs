import test from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import {
  approvedRoute,
  approvedHref,
  workspacePages,
} from "../src/approved-ui/routing.mjs";
test("every current workspace route resolves while all legacy=1 variants are rejected", () => {
  for (const page of workspacePages) {
    const href = approvedHref(page, "A"),
      resolved = approvedRoute(href);
    assert.equal(resolved.page, page);
    for (const suffix of ["legacy=1", "legacy=0&legacy=1", "%6cegacy=1"]) {
      const route = approvedRoute(
        href + (href.includes("?") ? "&" : "?") + suffix,
      );
      assert.equal(route.removed, true);
      assert.equal(route.page, "页面未找到");
    }
  }
});
test("current React detail and editor routes have explicit owners instead of an old-page fallback", () => {
  for (const [url, page] of [
    ["#/auth", "登录"],
    ["#/studio/edit/new?kind=world", "创作中心"],
    ["#/diag/request/R", "管理中心"],
    ["#/diag/task/T", "管理中心"],
    ["#/diag/record/model-runs/R", "管理中心"],
    ["#/c/A/media/avatar", "媒体"],
    ["#/c/A/media/identity", "媒体"],
    ["#/c/A/governance?tab=capabilities", "角色详情"],
    ["#/c/A/growth/memory/M", "记忆"],
    ["#/c/A/worlds/W/edit", "世界"],
  ]) {
    assert.equal(approvedRoute(url).page, page, url);
    assert.equal(approvedRoute(url).legacy, undefined);
  }
  for (const path of ["#/removed-route", "#/c/A/old-page", "#/%malformed"])
    assert.equal(approvedRoute(path).notFound, true);
});
test("React entry and all source files have no retired Vue or legacy page mount", async () => {
  const root = new URL("../src/", import.meta.url);
  let files = 0;
  async function walk(folder) {
    for (const entry of await readdir(folder, { withFileTypes: true })) {
      const url = new URL(
        entry.name + (entry.isDirectory() ? "/" : ""),
        folder,
      );
      if (entry.isDirectory()) await walk(url);
      else if (/\.(vue|tsx?|jsx?|mjs)$/.test(entry.name)) {
        assert.ok(!entry.name.endsWith(".vue"), url.href);
        const source = await readFile(url, "utf8");
        assert.doesNotMatch(source, /from\s*["'](?:vue|pinia|reka-ui)["']/);
        if (url.pathname.includes("/react-app/"))
          assert.doesNotMatch(source, /mountProduct|LegacySurface|legacy=1/);
        files++;
      }
    }
  }
  await walk(root);
  assert.ok(files > 50);
  const entry = await readFile(new URL("main.tsx", root), "utf8");
  assert.match(entry, /createRoot/);
  assert.doesNotMatch(entry, /createApp|reference-fixture|VITE_PRODUCT_MODE/);
});
