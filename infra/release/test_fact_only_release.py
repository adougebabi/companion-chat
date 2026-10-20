import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from argparse import Namespace
from unittest.mock import patch

import switch_images


spec = importlib.util.spec_from_file_location("release", Path(__file__).with_name("fact_only_release.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseTest(unittest.TestCase):
    def records(self, folder, source_ref=release.SOURCE_REF):
        for component, image in release.COMPONENTS.items():
            value = release.image_record({"RELEASE_COMPONENT": component, "RELEASE_IMAGE": "example/" + image,
                "RELEASE_DIGEST": "sha256:" + ("a" if component == "core" else "b") * 64,
                "GITHUB_SHA": "c" * 40, "GITHUB_REF": source_ref,
                "GITHUB_RUN_ID": "42", "GITHUB_RUN_ATTEMPT": "2",
                "GITHUB_SERVER_URL": "https://github.com", "GITHUB_REPOSITORY": "example/project"})
            release.write_json(Path(folder) / (component + ".json"), value)
        return release.complete_manifest(folder)

    def test_incomplete_or_mixed_set_cannot_promote(self):
        with tempfile.TemporaryDirectory() as folder:
            self.records(folder)
            web = Path(folder) / "web.json"
            value = json.loads(web.read_text())
            value["source_sha"] = "d" * 40
            value["tag"] = "fact-only-" + "d" * 40 + "-42-2"
            release.write_json(web, value)
            with self.assertRaisesRegex(ValueError, "identities differ"):
                release.complete_manifest(folder)
            web.unlink()
            with self.assertRaises(FileNotFoundError):
                release.complete_manifest(folder)

    def test_current_head_and_verified_complete_promotion(self):
        with tempfile.TemporaryDirectory() as folder:
            manifest = self.records(folder)
            commands = []
            def run(args):
                commands.append(args)
                if "inspect" in args:
                    char = "a" if "core-go" in args[4] else "b"
                    return json.dumps({"digest": "sha256:" + char * 64})
                return ""
            result = release.promote(manifest, Path(folder) / "result.json", lambda: "c" * 40, run)
            self.assertEqual(result["status"], "published")
            self.assertEqual(len(result["promoted"]), 2)
            self.assertTrue(all(":latest" not in " ".join(args) for args in commands))
            self.assertTrue(all("fact-only-latest" in " ".join(args) for args in commands))

    def test_stale_run_and_partial_failure_are_not_ready(self):
        with tempfile.TemporaryDirectory() as folder:
            manifest = self.records(folder)
            output = Path(folder) / "result.json"
            calls = []
            with self.assertRaisesRegex(ValueError, "stale build"):
                release.promote(manifest, output, lambda: "d" * 40, lambda args: calls.append(args))
            self.assertEqual(calls, [])
            heads = iter(["c" * 40, "d" * 40])
            def run(args):
                return json.dumps({"digest": "sha256:" + "a" * 64}) if "inspect" in args else ""
            with self.assertRaisesRegex(ValueError, "stale build"):
                release.promote(self.records(folder), output, lambda: next(heads), run)
            result = json.loads(output.read_text())
            self.assertEqual(result["status"], "promotion_failed")
            self.assertEqual(len(result["promoted"]), 1)

    def test_functional_recovery_publishes_only_its_own_tags(self):
        with tempfile.TemporaryDirectory() as folder:
            manifest = self.records(folder, "refs/heads/codex/fluctlight-functional-recovery")
            self.assertEqual(manifest["channel"], "functional-recovery")
            self.assertTrue(all(item["tag"] == "functional-recovery-" + "c" * 40 + "-42-2" for item in manifest["images"]))
            commands = []
            def run(args):
                commands.append(args)
                if "inspect" in args:
                    return json.dumps({"digest": "sha256:" + ("a" if "core-go" in args[4] else "b") * 64})
                return ""
            with patch.object(release, "current_head", return_value="c" * 40) as head:
                result = release.promote(manifest, Path(folder) / "result.json", run=run)
            self.assertEqual(result["status"], "published")
            self.assertEqual(head.call_count, 2)
            head.assert_called_with("refs/heads/codex/fluctlight-functional-recovery")
            self.assertTrue(all("functional-recovery-latest" in " ".join(args) for args in commands))
            self.assertTrue(all("fact-only-latest" not in " ".join(args) and ":latest" not in " ".join(args) for args in commands))
            args = Namespace(manifest=str(Path(folder) / "result.json"))
            selection = switch_images.selection(args)
            self.assertEqual(selection["TEMPORAL_WORKER_BUILD_ID"], "functional-recovery-" + "c" * 40 + "-42-2")
            self.assertEqual(selection["CORE_GO_IMAGE"], "example/fluctlight-core-go@sha256:" + "a" * 64)
            self.assertEqual(selection["WEB_IMAGE"], "example/fluctlight-web@sha256:" + "b" * 64)

    def test_cross_channel_and_unapproved_sources_are_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            manifest = self.records(folder)
            manifest["channel"] = "functional-recovery"
            with self.assertRaisesRegex(ValueError, "channel does not match"):
                release.promote(manifest, Path(folder) / "result.json")
            for source_ref in ["refs/heads/master", "refs/pull/1/merge", "refs/tags/v3", "refs/heads/other"]:
                with self.assertRaisesRegex(ValueError, "explicitly allowed"):
                    release.validate_image({**manifest["images"][0], "source_ref": source_ref})
            mixed = self.records(folder)
            recovery = self.records(folder, "refs/heads/codex/fluctlight-functional-recovery")
            release.write_json(Path(folder) / "core.json", mixed["images"][0])
            with self.assertRaisesRegex(ValueError, "identities differ"):
                release.complete_manifest(folder)
            with self.assertRaisesRegex(ValueError, "immutable tag"):
                release.validate_image({**recovery["images"][0], "tag": mixed["images"][0]["tag"]})


if __name__ == "__main__":
    unittest.main()
