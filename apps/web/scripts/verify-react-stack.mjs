import { readFile, readdir, stat } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
import ts from "typescript";
const root = fileURLToPath(new URL("../src/", import.meta.url)),
  web = path.dirname(root.replace(/\/$/, ""));
const retired =
  /(?:^|\/)(?:approved-ui\/(?:pages|renderers|shared)|v3\/proto\/(?:pages|modules)|v3\/proto\/(?:app|demo)\.js)(?:\/|$)/;
const banned =
  /^(?:vue(?:\/|$)|pinia(?:\/|$)|reka-ui(?:\/|$)|@vueuse\/|@lucide\/vue|shadcn-vue(?:\/|$))/;
const packageFile = JSON.parse(
  await readFile(path.join(web, "package.json"), "utf8"),
);
for (const name of Object.keys({
  ...packageFile.dependencies,
  ...packageFile.devDependencies,
}))
  if (
    banned.test(name) ||
    ["vue-tsc", "@vitejs/plugin-vue", "vue-component-type-helpers"].includes(
      name,
    )
  )
    throw Error("Retired Vue dependency: " + name);
async function walk(folder) {
  for (const entry of await readdir(folder, { withFileTypes: true })) {
    const file = path.join(folder, entry.name);
    if (entry.isDirectory()) await walk(file);
    else if (entry.name.endsWith(".vue"))
      throw Error("Retired SFC remains: " + file);
  }
}
await walk(root);
const entryHtml = await readFile(path.join(web, "index.html"), "utf8");
const scripts = [
  ...entryHtml.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/gi),
];
for (const [, attrs, body] of scripts) {
  const source = attrs.match(/\bsrc=["']([^"']+)["']/)?.[1];
  if (body.trim() || !["/runtime-config.js", "/src/main.tsx"].includes(source))
    throw Error("Unexpected HTML script entry: " + String(source));
}
if (scripts.filter(([, attrs]) => attrs.includes("/src/main.tsx")).length !== 1)
  throw Error("Exactly one React module entry is required");
async function noExtraHtml(folder) {
  for (const entry of await readdir(folder, { withFileTypes: true })) {
    const file = path.join(folder, entry.name);
    if (entry.isDirectory()) await noExtraHtml(file);
    else if (/\.html?$/i.test(entry.name))
      throw Error("Unexpected served HTML entry: " + file);
  }
}
await noExtraHtml(path.join(web, "public"));
for (const entry of await readdir(web))
  if (/\.html?$/i.test(entry) && entry !== "index.html")
    throw Error("Unexpected Vite HTML entry: " + entry);

const seen = new Set();
async function resolve(specifier, from) {
  const base = specifier.startsWith("@/")
    ? path.join(root, specifier.slice(2))
    : path.resolve(path.dirname(from), specifier);
  for (const suffix of [
    "",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".mjs",
    "/index.ts",
    "/index.tsx",
    "/index.js",
  ]) {
    const file = base + suffix;
    try {
      if ((await stat(file)).isFile()) return file;
    } catch {}
  }
  throw Error("Unresolved product import: " + specifier + " from " + from);
}
async function visit(file) {
  if (seen.has(file)) return;
  seen.add(file);
  const relative = path.relative(root, file).replaceAll(path.sep, "/");
  if (retired.test(relative))
    throw Error("Retired UI is reachable: " + relative);
  if (!/\.[cm]?[jt]sx?$/.test(file)) return;
  const source = await readFile(file, "utf8");
  if (/\b(?:mountProduct|LegacySurface)\b/.test(source))
    throw Error("Retired page mount in " + file);
  const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true);
  const imports = [];
  const inspect = (node) => {
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier &&
      ts.isStringLiteral(node.moduleSpecifier)
    )
      imports.push(node.moduleSpecifier.text);
    if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword
    ) {
      const arg = node.arguments[0];
      if (ts.isStringLiteral(arg) || ts.isNoSubstitutionTemplateLiteral(arg))
        imports.push(arg.text);
      else throw Error("Computed product import must be reviewed: " + file);
    }
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      node.expression.name.text === "glob"
    )
      throw Error("Glob product entry must be reviewed: " + file);
    ts.forEachChild(node, inspect);
  };
  inspect(ast);
  for (const raw of imports) {
    const specifier = raw.split("?")[0];
    if (banned.test(specifier) || specifier.endsWith(".vue"))
      throw Error("Vue import in product: " + file);
    if (specifier.startsWith(".") || specifier.startsWith("@/"))
      await visit(await resolve(specifier, file));
  }
}
await visit(path.join(root, "main.tsx"));
console.log(
  `Verified React-only production graph (${seen.size} local modules): no Vue, Re ka, legacy pages or old router.`.replace(
    "Re ka",
    "Reka",
  ),
);
