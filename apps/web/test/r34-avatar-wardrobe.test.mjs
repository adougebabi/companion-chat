import test from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { act, installDOM, mountReact } from "./helpers/react-dom.mjs";
import { avatarInitial, avatarSource } from "../src/lib/avatar-presentation.mjs";
import { Avatar } from "../src/v3/proto/components/avatar.js";
import { profileSnapshotFromDetail } from "../src/v3/proto/data/profile.js";

const env = installDOM();
const vite = await createServer({ root: fileURLToPath(new URL("../", import.meta.url)), appType: "custom", logLevel: "silent", server: { middlewareMode: true } });
test.after(async () => { await vite.close(); env.restore(); });
const { default: CharacterAvatar } = await vite.ssrLoadModule("/src/react-app/shared/CharacterAvatar.tsx");
const urlForAsset = id => "https://media.example.test/api/media/" + encodeURIComponent(id);
const { default: WardrobeItemArt } = await vite.ssrLoadModule("/src/react-app/shared/WardrobeItemArt.tsx");

test("avatar initials strip only the leading test prefix and consume string or object sources", async () => {
  assert.equal(avatarInitial("FT-青岚", "F"), "青");
  assert.equal(avatarInitial(" ft- 青岚 "), "青");
  assert.equal(avatarInitial("青岚FT-"), "青");
  assert.equal(avatarInitial("", "光"), "光");
  assert.equal(avatarInitial("青岚", "岚"), "岚");
  assert.equal(avatarInitial("创建角色", "+"), "+");
  assert.equal(avatarInitial("青岚", "FT-青"), "青");
  assert.equal(avatarSource("/api/media/portrait"), "/api/media/portrait");
  assert.equal(avatarSource({ url: "/api/media/portrait" }), "/api/media/portrait");
  assert.match(Avatar({ name: "FT-青岚" }), /avatar-fallback[^>]*>青</);
  const v = await mountReact(vite, CharacterAvatar, { props: { name: "FT-青岚", initial: "F" } });
  try { assert.equal(v.host.querySelector('[data-slot="avatar-fallback"]').textContent, "青"); }
  finally { await v.dispose(); }
});

test("profile uses a normalized directory image only while detail has no avatar field", () => {
  assert.equal(profileSnapshotFromDetail({}, { avatar: "/api/media/selected" }).avatar, "/api/media/selected");
  assert.equal(profileSnapshotFromDetail({ avatar: { url: "/api/media/new" } }, { avatar: "/api/media/old" }).avatar, "/api/media/new");
  assert.equal(profileSnapshotFromDetail({ avatar: null }, { avatar: "/api/media/old" }).avatar, "");
});

test("wardrobe distinguishes absent associations, real image fetches, and failed fetches per item", async () => {
  const v = await mountReact(vite, WardrobeItemArt, { props: { urlForAsset, type: "upper_body", name: "蓝色衬衫", media: [], showStatus: true, allowRetry: true } });
  try {
    assert.equal(v.host.querySelector("img"), null);
    assert.match(v.host.textContent, /尚无关联图片 · 类别示意/);
    await v.update({ urlForAsset, type: "upper_body", name: "蓝色衬衫", showStatus: true });
    assert.match(v.host.textContent, /图片资料待读取 · 类别示意/);
    assert.doesNotMatch(v.host.textContent, /尚无关联图片/);
    await v.update({ urlForAsset, type: "upper_body", name: "蓝色衬衫", media: [{ asset_id: "old", created_at: "2026-10-01" }, { asset_id: "own/photo", created_at: "2026-11-02" }], showStatus: true, allowRetry: true });
    const image = v.host.querySelector("img");
    assert.equal(image.getAttribute("src"), "https://media.example.test/api/media/own%2Fphoto");
    assert.equal(image.alt, "蓝色衬衫的关联图片");
    assert.doesNotMatch(v.host.textContent, /尚无关联图片/);
    await act(async () => image.dispatchEvent(new window.Event("error")));
    assert.equal(v.host.querySelector("img"), null);
    assert.match(v.host.textContent, /图片加载失败 · 类别示意/);
    await v.click([...v.host.querySelectorAll("button")].find(button => button.textContent === "重新加载图片"));
    assert.equal(v.host.querySelector("img").getAttribute("src"), "https://media.example.test/api/media/own%2Fphoto");
    await v.update({ urlForAsset, type: "outer", name: "黑色夹克", media: [{ asset_id: "coat-photo" }], showStatus: true, allowRetry: true });
    assert.equal(v.host.querySelector("img").getAttribute("src"), "https://media.example.test/api/media/coat-photo");
    assert.doesNotMatch(v.host.textContent, /图片加载失败/);
  } finally { await v.dispose(); }
});
