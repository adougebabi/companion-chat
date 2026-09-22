import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const acceptanceDir = path.dirname(fileURLToPath(import.meta.url));
const repositoryRoot = path.resolve(acceptanceDir, "../..");
const runner = path.join(acceptanceDir, "run-go-live-provider-smoke.sh");

function run(args, env = {}) {
  return spawnSync(runner, args, {
    cwd: repositoryRoot,
    env: { ...process.env, ...env },
    encoding: "utf8",
  });
}

function fixedHereDocRows(source, functionName) {
  const match = source.match(new RegExp(`${functionName}\\(\\) \\{\\n  cat <<'EOF'\\n([\\s\\S]*?)\\nEOF\\n\\}`));
  assert.ok(match, `${functionName} fixed list is missing`);
  return match[1].split("\n");
}

test("help preserves the ordinary smoke and documents strict suites", () => {
  const result = run(["--help"]);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /--suite smoke\|tools\|agents\|all/);
  assert.match(result.stdout, /ordinary Provider smoke/);
  assert.match(result.stdout, /-p 1 -parallel 1/);
  assert.match(result.stdout, /fixed 18 product Tool rows/);
  assert.match(result.stdout, /FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE/);
});

test("unknown formal Tool fails before environment or Provider access", () => {
  const result = run(["--suite", "tools", "--tool", "not.a.product.tool"]);
  assert.equal(result.status, 2);
  assert.match(result.stderr, /unknown formal Tool name/);
});

test("active cross-process lock rejects a second runner", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "fluctlight-runner-lock-"));
  const lockDir = path.join(root, "lock");
  fs.mkdirSync(lockDir);
  fs.writeFileSync(path.join(lockDir, "pid"), `${process.pid}\n`);
  const result = run(["--suite", "tools", "--tool", "memory_event"], {
    FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR: lockDir,
  });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /another live Provider runner holds/);
});

test("full Agent suite requires visual dependencies before Provider access", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "fluctlight-runner-agent-"));
  const runDir = path.join(root, "evidence");
  const result = run(["--suite", "agents", "--run-dir", runDir], {
    GO_CORE_TEST_DATABASE_URL: "postgres://private.invalid/test",
    FLUCTLIGHT_LIVE_PROVIDER_URL: "http://provider.invalid/v1",
    FLUCTLIGHT_LIVE_PROVIDER_MODEL: "test-model",
    FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR: path.join(root, "lock"),
  });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /required environment variable is missing: FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE/);
  assert.doesNotMatch(result.stdout + result.stderr, /Provider connectivity/);
  assert.match(fs.readFileSync(path.join(runDir, "run.meta"), "utf8"), /overall_status=FAIL/);
});

test("the three Visual Identity Tools and complete Agent have fixed runner mappings", () => {
  const source = fs.readFileSync(runner, "utf8");
  assert.equal(fixedHereDocRows(source, "tool_ids").length, 18);
  assert.equal(fixedHereDocRows(source, "agent_ids").length, 17);
  assert.match(source, /visual_identity\.generate_candidate\) printf .*TestVisualIdentityGenerateCandidateToolCommitsDurableIntentAndReplays/);
  assert.match(source, /visual_identity\.commit_review\) printf .*TestVisualIdentityCommitReviewToolPreservesRejectedAssetAndCreatesNextAttempt/);
  assert.match(source, /visual_identity\.finalize\) printf .*TestVisualIdentityFinalizeToolCommitsCanonicalCharacterSheetAndCompletion/);
  assert.match(source, /printf '\^TestFormalAgentE2E\/%s\$\\n'/);
  assert.match(source, /\nvisual_identity\nconversation_summary\n/);
  assert.match(source, /TestRunADKLoopPreservesSameRoundMultipleCallAssociation/);
  assert.match(source, /TestAgentRunRecordRetainsCommittedToolAfterCancellationAndPreventsReplay/);
  assert.match(source, /TestLiveStreamTurnFormalAgentNDJSON/);
});

test("selected DB-only Tool uses fixed inventory, serial flags, and strict event verification", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "fluctlight-runner-tool-"));
  const fakeBin = path.join(root, "bin");
  const runDir = path.join(root, "evidence");
  fs.mkdirSync(fakeBin);
  const fakeGo = path.join(fakeBin, "go");
  fs.writeFileSync(
    fakeGo,
    `#!/usr/bin/env bash
set -eu
args="$*"
pkg="github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
if [[ "$args" == *"TestIndependentToolE2EFixedProductInventory"* ]]; then
  name=TestIndependentToolE2EFixedProductInventory
elif [[ "$args" == *"TestFormalToolEinoAdapterE2E"* ]]; then
  printf '{"Action":"run","Package":"%s","Test":"TestFormalToolEinoAdapterE2E"}\\n' "$pkg"
  printf '{"Action":"run","Package":"%s","Test":"TestFormalToolEinoAdapterE2E/memory_event"}\\n' "$pkg"
  printf '{"Action":"pass","Package":"%s","Test":"TestFormalToolEinoAdapterE2E/memory_event","Elapsed":0.01}\\n' "$pkg"
  name=TestFormalToolEinoAdapterE2E
else
  name=TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay
fi
printf '{"Action":"run","Package":"%s","Test":"%s"}\\n' "$pkg" "$name"
printf '{"Action":"pass","Package":"%s","Test":"%s","Elapsed":0.01}\\n' "$pkg" "$name"
printf '{"Action":"pass","Package":"%s","Elapsed":0.01}\\n' "$pkg"
`,
  );
  fs.chmodSync(fakeGo, 0o755);

  const result = run(["--suite", "tools", "--tool", "memory_event", "--run-dir", runDir], {
    GO_CORE_TEST_DATABASE_URL: "postgres://private.invalid/test",
    FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR: path.join(root, "lock"),
    PATH: `${fakeBin}:${process.env.PATH}`,
  });
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
  assert.match(result.stdout, /\[tool-e2e:memory_event\] PASS/);
  assert.doesNotMatch(result.stdout, /\[dual-e2e\] PASS/);
  const commands = fs.readFileSync(path.join(runDir, "commands.tsv"), "utf8");
  assert.match(commands, /tool-fixed-inventory\tfixed-product-tool-inventory\t0\t0\tPASS/);
  assert.match(commands, /tool-memory-event-eino-adapter\tcontrolled Tool memory_event formal Eino adapter\t0\t0\tPASS/);
  assert.match(commands, /tool-memory-event\tTool memory_event\t0\t0\tPASS/);
  const context = fs.readFileSync(path.join(runDir, "commands", "tool-memory-event.context.txt"), "utf8");
  assert.match(context, /-p 1 -parallel 1/);
  assert.match(context, /GO_CORE_TEST_DATABASE_URL=present/);
  assert.doesNotMatch(context, /private\.invalid/);
  assert.ok(fs.statSync(path.join(runDir, "commands", "tool-memory-event.sources.sha256")).size > 0);
});

test("a skipped required Tool row remains a strict failure", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "fluctlight-runner-skip-"));
  const fakeBin = path.join(root, "bin");
  const runDir = path.join(root, "evidence");
  fs.mkdirSync(fakeBin);
  const fakeGo = path.join(fakeBin, "go");
  fs.writeFileSync(
    fakeGo,
    `#!/usr/bin/env bash
set -eu
args="$*"
pkg="github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
if [[ "$args" == *"TestIndependentToolE2EFixedProductInventory"* ]]; then
  name=TestIndependentToolE2EFixedProductInventory
  action=pass
elif [[ "$args" == *"TestFormalToolEinoAdapterE2E"* ]]; then
  printf '{"Action":"run","Package":"%s","Test":"TestFormalToolEinoAdapterE2E"}\\n' "$pkg"
  printf '{"Action":"run","Package":"%s","Test":"TestFormalToolEinoAdapterE2E/memory_event"}\\n' "$pkg"
  printf '{"Action":"pass","Package":"%s","Test":"TestFormalToolEinoAdapterE2E/memory_event","Elapsed":0.01}\\n' "$pkg"
  name=TestFormalToolEinoAdapterE2E
  action=pass
else
  name=TestDirectToolExecutionMemoryEventAndRecallOwnsCommitAndOperationReplay
  action=skip
fi
printf '{"Action":"run","Package":"%s","Test":"%s"}\\n' "$pkg" "$name"
printf '{"Action":"%s","Package":"%s","Test":"%s","Elapsed":0.01}\\n' "$action" "$pkg" "$name"
printf '{"Action":"pass","Package":"%s","Elapsed":0.01}\\n' "$pkg"
`,
  );
  fs.chmodSync(fakeGo, 0o755);

  const result = run(["--suite", "tools", "--tool", "memory_event", "--run-dir", runDir], {
    GO_CORE_TEST_DATABASE_URL: "postgres://private.invalid/test",
    FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR: path.join(root, "lock"),
    PATH: `${fakeBin}:${process.env.PATH}`,
  });
  assert.equal(result.status, 1, `${result.stdout}\n${result.stderr}`);
  assert.match(fs.readFileSync(path.join(runDir, "commands.tsv"), "utf8"), /tool-memory-event\tTool memory_event\t0\t1\tFAIL/);
  assert.match(fs.readFileSync(path.join(runDir, "run.meta"), "utf8"), /overall_status=FAIL/);
});

test("a selected Tool fails when its required adapter child event is missing", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "fluctlight-runner-adapter-missing-"));
  const fakeBin = path.join(root, "bin");
  const runDir = path.join(root, "evidence");
  fs.mkdirSync(fakeBin);
  const fakeGo = path.join(fakeBin, "go");
  fs.writeFileSync(
    fakeGo,
    `#!/usr/bin/env bash
set -eu
args="$*"
pkg="github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
if [[ "$args" == *"TestIndependentToolE2EFixedProductInventory"* ]]; then
  name=TestIndependentToolE2EFixedProductInventory
else
  name=TestFormalToolEinoAdapterE2E
fi
printf '{"Action":"run","Package":"%s","Test":"%s"}\\n' "$pkg" "$name"
printf '{"Action":"pass","Package":"%s","Test":"%s","Elapsed":0.01}\\n' "$pkg" "$name"
printf '{"Action":"pass","Package":"%s","Elapsed":0.01}\\n' "$pkg"
`,
  );
  fs.chmodSync(fakeGo, 0o755);

  const result = run(["--suite", "tools", "--tool", "memory_event", "--run-dir", runDir], {
    GO_CORE_TEST_DATABASE_URL: "postgres://private.invalid/test",
    FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR: path.join(root, "lock"),
    PATH: `${fakeBin}:${process.env.PATH}`,
  });
  assert.equal(result.status, 1, `${result.stdout}\n${result.stderr}`);
  assert.match(result.stderr, /missing required passing test event.*TestFormalToolEinoAdapterE2E\/memory_event/);
  assert.match(
    fs.readFileSync(path.join(runDir, "commands.tsv"), "utf8"),
    /tool-memory-event-eino-adapter\tcontrolled Tool memory_event formal Eino adapter\t0\t1\tFAIL/,
  );
  assert.doesNotMatch(fs.readFileSync(path.join(runDir, "commands.tsv"), "utf8"), /\tTool memory_event\t/);
});

test("a selected Tool fails when its formal adapter child fails", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "fluctlight-runner-adapter-fail-"));
  const fakeBin = path.join(root, "bin");
  const runDir = path.join(root, "evidence");
  fs.mkdirSync(fakeBin);
  const fakeGo = path.join(fakeBin, "go");
  fs.writeFileSync(
    fakeGo,
    `#!/usr/bin/env bash
set -eu
args="$*"
pkg="github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
if [[ "$args" == *"TestIndependentToolE2EFixedProductInventory"* ]]; then
  name=TestIndependentToolE2EFixedProductInventory
  printf '{"Action":"run","Package":"%s","Test":"%s"}\\n' "$pkg" "$name"
  printf '{"Action":"pass","Package":"%s","Test":"%s","Elapsed":0.01}\\n' "$pkg" "$name"
  printf '{"Action":"pass","Package":"%s","Elapsed":0.01}\\n' "$pkg"
  exit 0
fi
printf '{"Action":"run","Package":"%s","Test":"TestFormalToolEinoAdapterE2E"}\\n' "$pkg"
printf '{"Action":"run","Package":"%s","Test":"TestFormalToolEinoAdapterE2E/memory_event"}\\n' "$pkg"
printf '{"Action":"fail","Package":"%s","Test":"TestFormalToolEinoAdapterE2E/memory_event","Elapsed":0.01}\\n' "$pkg"
printf '{"Action":"fail","Package":"%s","Test":"TestFormalToolEinoAdapterE2E","Elapsed":0.01}\\n' "$pkg"
printf '{"Action":"fail","Package":"%s","Elapsed":0.01}\\n' "$pkg"
exit 1
`,
  );
  fs.chmodSync(fakeGo, 0o755);

  const result = run(["--suite", "tools", "--tool", "memory_event", "--run-dir", runDir], {
    GO_CORE_TEST_DATABASE_URL: "postgres://private.invalid/test",
    FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR: path.join(root, "lock"),
    PATH: `${fakeBin}:${process.env.PATH}`,
  });
  assert.equal(result.status, 1, `${result.stdout}\n${result.stderr}`);
  assert.match(
    fs.readFileSync(path.join(runDir, "commands.tsv"), "utf8"),
    /tool-memory-event-eino-adapter\tcontrolled Tool memory_event formal Eino adapter\t1\t1\tFAIL/,
  );
});

test("the full Agent suite fails when a controlled cross-case event is missing", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "fluctlight-runner-agent-gate-missing-"));
  const fakeBin = path.join(root, "bin");
  const runDir = path.join(root, "evidence");
  const visualConfig = path.join(root, "visual.json");
  fs.mkdirSync(fakeBin);
  fs.writeFileSync(
    visualConfig,
    JSON.stringify({
      media_comfyui: { baseUrl: "http://comfy.invalid", workflow: { prompt: "{{prompt}}" } },
      s3: { endpoint: "http://s3.invalid", access_key: "test", secret_key: "test", bucket_prefix: "runner" },
    }),
  );
  const fakeGo = path.join(fakeBin, "go");
  fs.writeFileSync(
    fakeGo,
    `#!/usr/bin/env bash
set -eu
args="$*"
pkg="github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
if [[ "$args" == *"TestVisualIdentityLiveE2EPreflight"* ]]; then
  name=TestVisualIdentityLiveE2EPreflight
else
  name=TestFormalAgentE2ERejectsBrokenToolResultFeedback
fi
printf '{"Action":"run","Package":"%s","Test":"%s"}\\n' "$pkg" "$name"
printf '{"Action":"pass","Package":"%s","Test":"%s","Elapsed":0.01}\\n' "$pkg" "$name"
printf '{"Action":"pass","Package":"%s","Elapsed":0.01}\\n' "$pkg"
`,
  );
  fs.chmodSync(fakeGo, 0o755);
  const fakeCurl = path.join(fakeBin, "curl");
  fs.writeFileSync(fakeCurl, "#!/usr/bin/env bash\nexit 0\n");
  fs.chmodSync(fakeCurl, 0o755);

  const result = run(["--suite", "agents", "--run-dir", runDir], {
    GO_CORE_TEST_DATABASE_URL: "postgres://private.invalid/test",
    FLUCTLIGHT_LIVE_PROVIDER_URL: "http://provider.invalid/v1",
    FLUCTLIGHT_LIVE_PROVIDER_MODEL: "test-model",
    FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE: visualConfig,
    FLUCTLIGHT_LIVE_PROVIDER_LOCK_DIR: path.join(root, "lock"),
    PATH: `${fakeBin}:${process.env.PATH}`,
  });
  assert.equal(result.status, 1, `${result.stdout}\n${result.stderr}`);
  assert.match(result.stderr, /missing required passing test event.*TestRunADKLoopPreservesSameRoundMultipleCallAssociation/);
  assert.match(
    fs.readFileSync(path.join(runDir, "commands.tsv"), "utf8"),
    /agent-controlled-cross-cases\tcontrolled-agent-cross-cases\t0\t1\tFAIL/,
  );
  assert.doesNotMatch(fs.readFileSync(path.join(runDir, "commands.tsv"), "utf8"), /agent-live-production-stream/);
});
