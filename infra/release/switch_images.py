#!/usr/bin/env python3
"""Select explicit Core/Web image references. Preview by default; --apply deploys."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
COMPOSE = ROOT / "infra/compose/fluctlight.compose.yml"


def image_ref(value, component):
    if not re.fullmatch(r"(?:docker\.io/)?[a-z0-9][a-z0-9_.-]*/fluctlight-" + component +
                        r"(?:@[a-z0-9]+:[0-9a-f]{64}|:[a-zA-Z0-9_.-]+)", value):
        raise ValueError("explicit image reference required for " + component)
    return value


def selection(args):
    if args.manifest:
        manifest = json.loads(Path(args.manifest).read_text())
        if manifest.get("status") != "published" or manifest.get("channel") != "fact-only":
            raise ValueError("test manifest is not a complete published release")
        images = {item["component"]: item for item in manifest["images"]}
        if set(images) != {"core", "web"} or len(manifest["images"]) != 2:
            raise ValueError("manifest must contain exactly Core and Web")
        refs = {key: images[key]["image"] + "@" + images[key]["digest"] for key in images}
        build_id = "fact-only-" + manifest["source_sha"] + "-" + str(manifest["run_id"]) + "-" + str(manifest["attempt"])
    else:
        if not args.core_image or not args.web_image or not args.worker_build_id:
            raise ValueError("explicit Core/Web references and worker build ID are required")
        refs = {"core": args.core_image, "web": args.web_image}
        build_id = args.worker_build_id
    return {"CORE_GO_IMAGE": image_ref(refs["core"], "core-go"),
            "WEB_IMAGE": image_ref(refs["web"], "web"), "TEMPORAL_WORKER_BUILD_ID": build_id}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env-file", required=True)
    parser.add_argument("--manifest")
    parser.add_argument("--core-image")
    parser.add_argument("--web-image")
    parser.add_argument("--worker-build-id")
    parser.add_argument("--migrate", action="store_true", help="Run the new image migration before starting services; omit on rollback")
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    if not Path(args.env_file).is_file():
        parser.error("private deployment env file does not exist")
    selected = selection(args)
    # Process env overrides --env-file; this really switches the full image
    # variables already consumed by Compose, rather than an unused tag setting.
    environment = {**os.environ, **selected}
    compose = ["docker", "compose", "--env-file", str(Path(args.env_file).resolve()), "-f", str(COMPOSE)]
    print(json.dumps({"apply": args.apply, "images": selected}, indent=2))
    if not args.apply:
        print("Preview only. Use an isolated test namespace/data environment; back up settings and drain new reply drafts before rollback.")
        return
    subprocess.run(compose + ["pull", "core", "worker", "web", "migrate"], env=environment, check=True)
    if args.migrate:
        subprocess.run(compose + ["run", "--rm", "--no-deps", "migrate"], env=environment, check=True)
    subprocess.run(compose + ["up", "--detach", "--no-build", "--no-deps", "--force-recreate", "--wait", "core", "worker", "web"],
                   env=environment, check=True)
    for service, variable in [("core", "CORE_GO_IMAGE"), ("worker", "CORE_GO_IMAGE"), ("web", "WEB_IMAGE")]:
        container_id = subprocess.check_output(compose + ["ps", "--quiet", service], env=environment, text=True).strip()
        actual = json.loads(subprocess.check_output(["docker", "inspect", container_id], text=True))[0]
        if actual["Config"]["Image"] != selected[variable]:
            raise ValueError("running " + service + " has the wrong configured image")
        image = json.loads(subprocess.check_output(["docker", "image", "inspect", actual["Image"]], text=True))[0]
        print(json.dumps({"service": service, "configured_image": selected[variable], "image_id": actual["Image"],
                          "repo_digests": image.get("RepoDigests", []), "state": actual["State"].get("Status"),
                          "health": actual["State"].get("Health", {}).get("Status")}))


if __name__ == "__main__":
    main()
