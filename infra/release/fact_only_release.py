#!/usr/bin/env python3
"""Publish complete, branch-scoped test image sets; never touch stable aliases."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess


SOURCE_REF = "refs/heads/codex/fact-only-reply-vision"
CHANNELS = {
    SOURCE_REF: "fact-only",
    "refs/heads/codex/fluctlight-functional-recovery": "functional-recovery",
}
COMPONENTS = {"core": "fluctlight-core-go", "web": "fluctlight-web"}


def channel_for_ref(source_ref):
    if source_ref not in CHANNELS:
        raise ValueError("test channel requires an explicitly allowed branch")
    return CHANNELS[source_ref]


def write_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2) + "\n")
    temporary.replace(path)


def validate_image(value):
    component = value.get("component")
    if component not in COMPONENTS:
        raise ValueError("unknown release component")
    if not re.fullmatch(r"[a-z0-9][a-z0-9_.-]*/" + COMPONENTS[component], value.get("image", "")):
        raise ValueError("unexpected Docker Hub image")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", value.get("digest", "")):
        raise ValueError("invalid image digest")
    if not re.fullmatch(r"[0-9a-f]{40}", value.get("source_sha", "")):
        raise ValueError("invalid source revision")
    channel = channel_for_ref(value.get("source_ref"))
    for field in ("run_id", "attempt"):
        if not re.fullmatch(r"[1-9][0-9]*", str(value.get(field, ""))):
            raise ValueError("invalid build identity")
    expected = channel + "-{source_sha}-{run_id}-{attempt}".format(**value)
    if value.get("tag") != expected:
        raise ValueError("immutable tag does not match the build")
    return value


def image_record(environment):
    value = {
        "component": environment["RELEASE_COMPONENT"],
        "image": environment["RELEASE_IMAGE"],
        "digest": environment["RELEASE_DIGEST"],
        "source_sha": environment["GITHUB_SHA"],
        "source_ref": environment["GITHUB_REF"],
        "run_id": environment["GITHUB_RUN_ID"],
        "attempt": environment["GITHUB_RUN_ATTEMPT"],
        "run_url": "{GITHUB_SERVER_URL}/{GITHUB_REPOSITORY}/actions/runs/{GITHUB_RUN_ID}".format(**environment),
    }
    value["tag"] = channel_for_ref(value["source_ref"]) + "-{source_sha}-{run_id}-{attempt}".format(**value)
    return validate_image(value)


def complete_manifest(directory):
    records = [validate_image(json.loads((Path(directory) / (name + ".json")).read_text())) for name in COMPONENTS]
    identity_fields = ("source_sha", "source_ref", "run_id", "attempt", "run_url")
    first = records[0]
    if [item["component"] for item in records] != list(COMPONENTS):
        raise ValueError("incomplete or duplicate release set")
    if any(any(item.get(key) != first.get(key) for key in identity_fields) for item in records):
        raise ValueError("image source/build identities differ")
    return {"schema_version": 1, "channel": channel_for_ref(first["source_ref"]), "status": "built",
            **{key: first[key] for key in identity_fields}, "images": records, "promoted": []}


def command(arguments):
    return subprocess.check_output(arguments, text=True).strip()


def current_head(source_ref=SOURCE_REF):
    channel_for_ref(source_ref)
    fields = command(["git", "ls-remote", "--exit-code", "origin", source_ref]).split()
    if len(fields) != 2 or fields[1] != source_ref:
        raise ValueError("branch head could not be verified")
    return fields[0]


def image_digest(reference, run=command):
    # Manifest is Buildx's documented inspect template object. Its Descriptor
    # is the registry descriptor, not the local container configuration digest.
    value = json.loads(run(["docker", "buildx", "imagetools", "inspect", reference,
                            "--format", "{{json .Manifest}}"] ))
    digest = value.get("digest")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest or ""):
        raise ValueError("registry returned no valid manifest digest")
    return digest


def promote(manifest, output, head=None, run=command):
    channel = channel_for_ref(manifest["source_ref"])
    if manifest.get("channel") != channel:
        raise ValueError("manifest channel does not match its source branch")
    if head is None:
        head = lambda: current_head(manifest["source_ref"])
    # A registry cannot make two repository aliases atomic. Keep the immutable
    # references and partial evidence even when the second promotion fails.
    write_json(output, manifest)
    try:
        for item in manifest["images"]:
            if head() != manifest["source_sha"]:
                raise ValueError("stale build: branch head has changed")
            reference = item["image"] + "@" + item["digest"]
            alias = item["image"] + ":" + channel + "-latest"
            run(["docker", "buildx", "imagetools", "create", "--prefer-index=false", "--tag", alias, reference])
            actual = image_digest(alias, run)
            if actual != item["digest"]:
                raise ValueError("promoted alias does not match the built digest")
            manifest["promoted"].append({"component": item["component"], "reference": alias, "digest": actual})
            manifest["status"] = "partial"
            write_json(output, manifest)
        manifest["status"] = "published"
        write_json(output, manifest)
    except Exception:
        manifest["status"] = "promotion_failed"
        write_json(output, manifest)
        raise
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["record", "promote"])
    parser.add_argument("--directory")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    if args.action == "record":
        if command(["git", "rev-parse", "HEAD"]) != os.environ["GITHUB_SHA"]:
            raise ValueError("checkout source revision differs from the build identity")
        write_json(args.output, image_record(os.environ))
    else:
        if not args.directory:
            parser.error("promote requires --directory")
        manifest = complete_manifest(args.directory)
        for key, env_name in [("source_sha", "GITHUB_SHA"), ("source_ref", "GITHUB_REF"),
                              ("run_id", "GITHUB_RUN_ID"), ("attempt", "GITHUB_RUN_ATTEMPT")]:
            if manifest[key] != os.environ.get(env_name):
                raise ValueError("artifact identity differs from this publishing run")
        promote(manifest, args.output)


if __name__ == "__main__":
    main()
