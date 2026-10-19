#!/usr/bin/env python3
"""Build the deployment's pinned official MinIO sources; never use a mirror binary."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

SOURCES = {
    'minio': ('RELEASE.2025-04-22T22-12-26Z', '0d7408fc9969caf07de6a8c3a84f9fbb10a6739e', 'server'),
    'mc': ('RELEASE.2025-04-16T18-13-26Z', 'b00526b153a31b36767991a4f5ce2cced435ee8e', 'client'),
}

def run(args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)

def build(component, root, architecture):
    tag, revision, dockerfile = SOURCES[component]
    source = root / (component + '-source')
    if not source.exists():
        run(['git', 'clone', '--depth', '1', '--branch', tag, 'https://github.com/minio/' + component + '.git', str(source)])
    actual = run(['git', '-C', str(source), 'rev-parse', 'HEAD'], capture_output=True, text=True).stdout.strip()
    if actual != revision:
        raise RuntimeError('pinned_source_revision_mismatch: ' + component)
    if run(['git', '-C', str(source), 'status', '--porcelain'], capture_output=True, text=True).stdout:
        raise RuntimeError('source_checkout_has_changes: ' + component)
    context = root / (component + '-image')
    context.mkdir(exist_ok=True)
    binary = context / component
    environment = dict(os.environ, GOOS='linux', GOARCH=architecture, CGO_ENABLED='0')
    run(['go', 'build', '-trimpath', '-o', str(binary), '.'], cwd=source, env=environment)
    for name in ('LICENSE', 'CREDITS'):
        shutil.copyfile(source / name, context / name)
    image = 'fluctlight-' + component + '-source:' + tag
    run(['docker', 'build', '--build-arg', 'SOURCE_REVISION=' + revision, '--tag', image,
         '--file', str(Path(__file__).parent / ('Dockerfile.' + dockerfile)), str(context)])
    return {'component': component, 'release': tag, 'revision': revision, 'image': image, 'architecture': architecture}

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--workdir', type=Path)
    parser.add_argument('--component', choices=['minio', 'mc', 'all'], default='all')
    args = parser.parse_args()
    root = (args.workdir or Path(tempfile.mkdtemp(prefix='fluctlight-minio-build-'))).resolve()
    root.mkdir(mode=0o700, parents=True, exist_ok=True)
    architecture = run(['docker', 'version', '--format', '{{.Server.Arch}}'], capture_output=True, text=True).stdout.strip()
    if architecture not in ('amd64', 'arm64'):
        raise RuntimeError('unsupported_container_architecture')
    result = [build(name, root, architecture) for name in SOURCES if args.component in ('all', name)]
    print(json.dumps({'source_builds': result, 'workdir': str(root)}, indent=2))

if __name__ == '__main__':
    main()
