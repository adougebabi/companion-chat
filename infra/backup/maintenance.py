#!/usr/bin/env python3
"""Execute an Owner-created backup/isolated-restore maintenance job.

No shell evaluates user input. Docker/DB credentials remain in private files.
The API/Worker are never started by this tool; no model task runs on restore.
"""
import argparse
import fcntl
import datetime
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import secrets
import shutil
import subprocess
import sys
import tarfile
import time

VOLUMES = ('fluctlight_postgres', 'fluctlight_minio', 'fluctlight_redis', 'fluctlight_kev_core', 'fluctlight_kev_worker')
SCHEMA = 'fluctlight.business-backup.v1'

class OperatorError(Exception):
    pass

def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()

def digest(path):
    value = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            value.update(block)
    return value.hexdigest()

def private_json(path, value):
    temporary = Path(str(path) + '.writing-' + secrets.token_hex(8))
    with temporary.open('x', encoding='utf-8') as stream:
        json.dump(value, stream, ensure_ascii=False, indent=2)
        stream.flush()
        os.fsync(stream.fileno())
    os.chmod(temporary, 0o600)
    temporary.replace(path)
    directory_fd = os.open(Path(path).parent, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(directory_fd)
    finally:
        os.close(directory_fd)

def safe_env(path, values):
    for key, value in values.items():
        if not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*', key) or '\n' in str(value) or '\r' in str(value):
            raise OperatorError('deployment_environment_not_portable')
    path.write_text(''.join(f'{key}={value}\n' for key, value in values.items()), encoding='utf-8')
    os.chmod(path, 0o600)

def inspect_tar(path):
    files = total = 0
    with tarfile.open(path, 'r:') as archive:
        for member in archive:
            name = PurePosixPath(member.name)
            if name.is_absolute() or '..' in name.parts or not (member.isfile() or member.isdir()):
                raise OperatorError('unsupported_or_unsafe_volume_entry')
            if member.isfile():
                files += 1
                total += member.size
    return {'files': files, 'logical_bytes': total}

def host_lock(scope):
    daemon = subprocess.run(['docker','info','--format','{{.ID}}'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=True).stdout.decode().strip()
    name = hashlib.sha256((daemon+':'+scope).encode()).hexdigest()
    path = Path('/tmp').resolve() / ('fluctlight-maintenance-'+name+'.lock')
    fd = os.open(path,os.O_CREAT|os.O_RDWR|os.O_NOFOLLOW,0o600)
    stream = os.fdopen(fd,'a+')
    try:
        fcntl.flock(stream.fileno(),fcntl.LOCK_EX|fcntl.LOCK_NB)
    except BlockingIOError:
        stream.close()
        raise OperatorError('deployment_maintenance_already_running')
    return stream

class Operator:
    def __init__(self, args):
        self.args = args
        self.compose_file = Path(args.compose).resolve(strict=True)
        self.env_file = Path(args.env_file).resolve(strict=True)
        self.deployment_env_bytes = self.env_file.read_bytes()
        if not self.env_file.is_file() or self.env_file.stat().st_mode & 0o077:
            raise OperatorError('private_env_file_permissions_required')
        self.root = Path(args.workdir).absolute()
        if ',' in str(self.root) or self.root.is_symlink():
            raise OperatorError('private_backup_directory_required')
        self.root.mkdir(mode=0o700, parents=True, exist_ok=True)
        if self.root.stat().st_mode & 0o077:
            raise OperatorError('private_backup_directory_permissions_required')
        self.journal = self.root / 'operator-journal.jsonl'
        self.compose = ['docker', 'compose', '--env-file', str(self.env_file), '-f', str(self.compose_file)]
        self.config = json.loads(self.command(self.compose + ['config', '--format', 'json'], 'read_topology'))
        self.project = self.config['name']
        self.deployment_lock = host_lock('deployment:'+self.project)
        self.runtime = self.config['services']['core']['environment']
        from urllib.parse import urlsplit
        if urlsplit(self.runtime['CORE_GO_DATABASE_URL']).hostname != 'postgres' or urlsplit(self.runtime['S3_ENDPOINT']).hostname != 'minio':
            raise OperatorError('local_compose_storage_topology_required')
        minio_values = self.config['services']['minio']['environment']
        if minio_values['MINIO_ROOT_USER'] != self.runtime['S3_ACCESS_KEY'] or minio_values['MINIO_ROOT_PASSWORD'] != self.runtime['S3_SECRET_KEY']:
            raise OperatorError('compose_media_credentials_topology_mismatch')
        self.runtime_file = self.root / 'operator-runtime.env'
        safe_env(self.runtime_file, self.runtime)
        self.network = self.config['networks']['platform']['name']
        self.containers = self.container_inventory()
        core_containers = [item for item in self.containers if item['service'] == 'core']
        if len(core_containers) != 1:
            raise OperatorError('one_existing_core_container_required')
        self.image = core_containers[0]['image']
        self.restore_image = args.core_image or self.image
        self.command(['docker','image','inspect',self.restore_image],'restore_core_image_missing')
        if any(core_containers[0]['environment'].get(key) != str(value) for key, value in self.runtime.items()):
            raise OperatorError('deployment_env_differs_from_actual_core')
        self.bound_volumes = {}
        for logical,service,destination in [('fluctlight_postgres','postgres','/var/lib/postgresql/data'),('fluctlight_minio','minio','/data'),('fluctlight_redis','redis','/data'),('fluctlight_kev_core','core','/var/lib/fluctlight/kev'),('fluctlight_kev_worker','worker','/var/lib/fluctlight/kev')]:
            containers = [c for c in self.containers if c['service']==service]
            if len(containers)!=1 or self.network not in containers[0]['networks']:
                raise OperatorError('actual_storage_container_topology_mismatch')
            mounts=[m for m in containers[0]['mounts'] if m['Destination']==destination]
            if len(mounts)!=1 or mounts[0]['Type']!='volume' or mounts[0].get('Name')!=self.config['volumes'][logical]['name']:
                raise OperatorError('actual_storage_mount_differs_from_compose')
            self.bound_volumes[logical]=mounts[0]['Name']
        self.utility = args.utility_image
        self.private = self.root / (args.job + '.execution.json')
        if self.private.exists():
            saved = json.loads(self.private.read_text())
            if saved.get('job') != args.job or saved.get('project') != self.project:
                raise OperatorError('execution_scope_conflict')
            self.token = saved['token']
            if not any(c['running'] and c['service'] == 'postgres' for c in self.containers):
                self.source_up_for_receipt()
        else:
            self.token = secrets.token_urlsafe(48)
            # Identity reaches disk before the DB command; a lost claim response
            # cannot leave an unrecoverable, ownerless maintenance execution.
            private_json(self.private, {'job': args.job, 'project': self.project, 'token': self.token})
        self.claim = self.ctl('claim', {'token': self.token})
        self.output = self.root / f"{args.job}-attempt-{self.claim['attempt']}"
        self.output.mkdir(mode=0o700, exist_ok=True)
        self.running_before = [item['service'] for item in self.containers if item['running']]
        self.stopped_source = False
        self.target_containers = []

    def log(self, stage, **values):
        with self.journal.open('a', encoding='utf-8') as stream:
            stream.write(json.dumps({'at': now(), 'stage': stage, **values}, ensure_ascii=False) + '\n')
        print(stage, flush=True)

    def command(self, args, stage, *, body=None, stdout=None, okay_missing=False, timeout=7200):
        result = subprocess.run(args, input=body, stdout=stdout if stdout is not None else subprocess.PIPE,
                                stderr=subprocess.PIPE, timeout=timeout, check=False)
        if result.returncode and not okay_missing:
            # Never print subprocess stderr: tooling may echo a deployment URL.
            raise OperatorError(stage)
        return result.stdout if stdout is None else b''

    def container_inventory(self):
        ids = self.command(self.compose + ['ps', '-a', '-q'], 'inspect_source_containers').decode().split()
        if not ids:
            raise OperatorError('existing_source_deployment_required')
        values = json.loads(self.command(['docker', 'inspect', *ids], 'inspect_source_containers'))
        result = []
        for item in values:
            labels = item['Config'].get('Labels') or {}
            if labels.get('com.docker.compose.project') != self.project:
                raise OperatorError('source_project_mismatch')
            result.append({'id': item['Id'], 'service': labels['com.docker.compose.service'],
                           'running': item['State']['Running'], 'image': item['Image'], 'environment':dict(v.split('=',1) for v in item['Config'].get('Env',[]) if '=' in v), 'mounts':item.get('Mounts',[]), 'networks':item.get('NetworkSettings',{}).get('Networks',{})})
        return result

    def ctl(self, command, value=None, *, network=None, environment=None, output=None):
        args = ['docker', 'run', '--rm', '--network', network or self.network, '--env-file', str(environment or self.runtime_file), '-i', self.restore_image if network else self.image,
                '/usr/local/bin/fluctlight-backupctl-go', command]
        if command in ('claim', 'record', 'audit-restore'):
            args += ['--job', self.args.job]
        raw = self.command(args, 'operator_' + command, body=json.dumps(value).encode() if value is not None else None, stdout=output)
        return json.loads(raw) if output is None else None

    def record(self, stage, status='running', result=None, private=None):
        self.log(stage, status=status)
        return self.ctl('record', {'token': self.token, 'stage': stage, 'status': status, 'attempt': self.claim['attempt'],
                                   'result': result or {}, 'private': private or {}})

    def probe(self, path, *, network=None, environment=None):
        with path.open('wb') as stream:
            self.ctl('probe', network=network, environment=environment, output=stream)
        header = footer = None
        with path.open(encoding='utf-8') as stream:
            for line in stream:
                value = json.loads(line)
                if header is None:
                    if value.get('type') != 'manifest':
                        raise OperatorError('probe_manifest_missing')
                    header = value
                footer = value
        if not header or not footer or footer.get('type') != 'end' or footer.get('complete') is not True:
            raise OperatorError('probe_incomplete')
        return header, footer

    def source_up_for_receipt(self):
        # Only the database is brought back for durable reporting. This cannot
        # dispatch a Temporal workflow, media request or model call.
        self.command(self.compose + ['start', 'postgres'], 'restart_postgres_for_receipt', timeout=120)
        for _ in range(60):
            result = subprocess.run(self.compose + ['exec', '-T', 'postgres', 'sh', '-c',
                                    'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"'],
                                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
            if result.returncode == 0:
                return
            time.sleep(1)
        raise OperatorError('source_postgres_not_ready')

    def backup(self):
        manifest_path = self.output / 'manifest.json'
        if manifest_path.exists():
            # Final receipt was lost after all artifacts completed. Verify the
            # original bundle, never overwrite it with another source snapshot.
            manifest = json.loads(manifest_path.read_text())
            self.verify_bundle(self.output, manifest)
            self.source_up_for_receipt()
            return self.complete_backup(manifest_path, manifest)
        if any(self.output.iterdir()):
            raise OperatorError('partial_backup_requires_failed_attempt_review')
        self.record('quiescing_source')
        self.command(self.compose + ['stop', '--timeout', '60', 'web', 'core', 'worker', 'temporal', 'redis'], 'quiesce_source', timeout=180)
        self.stopped_source = True
        required = {'web', 'core', 'worker', 'temporal', 'redis'}
        if any(c['running'] and c['service'] in required for c in self.container_inventory()):
            raise OperatorError('source_writers_still_running')
        header, footer = self.probe(self.output / 'source-probe.jsonl')
        self.log('source_probe_verified')
        self.command(self.compose + ['stop', '--timeout', '60'], 'clean_stop_source', timeout=180)
        if any(c['running'] for c in self.container_inventory()):
            raise OperatorError('source_deployment_not_stopped')
        artifacts = []
        for logical in VOLUMES:
            source = self.bound_volumes[logical]
            self.command(['docker', 'volume', 'inspect', source], 'source_volume_missing')
            name = logical + '.tar'
            self.log('archive_volume', volume=logical)
            self.command(['docker', 'run', '--rm', '--network', 'none', '--user', '0:0',
                          '--mount', f'type=volume,src={source},dst=/source,readonly',
                          '--mount', f'type=bind,src={self.output},dst=/backup', self.utility,
                          'tar', '-cpf', '/backup/' + name, '-C', '/source', '.'], 'archive_volume')
            archive = self.output / name
            artifacts.append({'name': name, 'logical_volume': logical, 'sha256': digest(archive),
                              'bytes': archive.stat().st_size, **inspect_tar(archive)})
        (self.output / 'deployment.env').write_bytes(self.deployment_env_bytes)
        shutil.copyfile(self.runtime_file, self.output / 'runtime.env')
        utility_id = json.loads(self.command(['docker', 'image', 'inspect', self.utility], 'inspect_utility_image'))[0]['Id']
        image_ids = sorted({c['image'] for c in self.containers} | {utility_id})
        self.log('archive_deployment_images')
        image_archive = self.output / 'deployment-images.tar'
        self.command(['docker', 'image', 'save', '--output', str(image_archive), *image_ids], 'archive_deployment_images')
        for name in ('deployment.env', 'runtime.env', 'source-probe.jsonl', 'deployment-images.tar'):
            os.chmod(self.output / name, 0o600)
            artifacts.append({'name': name, 'sha256': digest(self.output / name), 'bytes': (self.output / name).stat().st_size})
        images = {c['service']: c['image'] for c in self.containers}
        manifest = {'schema': SCHEMA, 'job_id': self.args.job, 'attempt': self.claim['attempt'], 'created_at': now(),
                    'source_project': self.project, 'source_quiesced': True, 'strategy': 'clean_stopped_volume_snapshot',
                    'migration_head': header['migration_head'], 'images': images, 'utility_image': utility_id,
                    'artifacts': artifacts, 'media_checked': footer['media_checked'], 'legacy_media_versions': footer['legacy_media_versions'],
                    'foreign_keys_validated': header['unvalidated_foreign_keys'] == 0,
                    'active_workflow_count': len(header['active_workflows']), 'models_started': False,
                    'private_environment_fields': sorted(self.runtime),
                    'external_provider_state': 'not_snapshotted; original intents, request IDs and provider job IDs retained'}
        private_json(manifest_path, manifest)
        self.verify_bundle(self.output, manifest)
        self.source_up_for_receipt()
        self.complete_backup(manifest_path, manifest)

    def verify_bundle(self, root, manifest):
        if manifest.get('schema') != SCHEMA or manifest.get('source_quiesced') is not True:
            raise OperatorError('unsupported_or_incomplete_backup')
        names = [a.get('name') for a in manifest.get('artifacts', [])]
        required = [v + '.tar' for v in VOLUMES] + ['deployment.env', 'runtime.env', 'source-probe.jsonl', 'deployment-images.tar']
        if sorted(names) != sorted(required):
            raise OperatorError('backup_artifact_set_incomplete')
        for artifact in manifest['artifacts']:
            path = root / artifact['name']
            if path.is_symlink() or not path.is_file() or path.stat().st_size != artifact['bytes'] or digest(path) != artifact['sha256']:
                raise OperatorError('backup_artifact_digest_mismatch')
            if artifact.get('logical_volume'):
                check = inspect_tar(path)
                if check['files'] != artifact['files'] or check['logical_bytes'] != artifact['logical_bytes']:
                    raise OperatorError('volume_archive_manifest_mismatch')

    def complete_backup(self, path, manifest):
        result = {'restorable': True, 'source_inventory_verified': True, 'restore_comparison_verified': False, 'artifacts_verified': True, 'database_counts_verified': True, 'media_verified': True,
                  'migration_head': manifest['migration_head'], 'media_count': manifest['media_checked'],
                  'legacy_media_versions': manifest['legacy_media_versions'], 'foreign_keys_validated': manifest['foreign_keys_validated'],
                  'manifest_digest': digest(path), 'artifact_count': len(manifest['artifacts']), 'models_started': False,
                  'external_provider_state': manifest['external_provider_state']}
        self.record('backup_verified', 'completed', result, {'artifact_dir': str(self.output), 'manifest_digest': digest(path)})
        print('manifest_sha256=' + digest(path), flush=True)

    def restore(self):
        name = self.claim.get('target_name')
        if not isinstance(name, str) or not re.fullmatch(r'fluctlight_restore_[a-z0-9]{8,32}', name):
            raise OperatorError('new_isolated_target_required')
        self.target_lock = host_lock('restore-target:'+name)
        self.restore_nonce = secrets.token_hex(24)
        source = self.claim.get('source') or {}
        root = Path(source['artifact_dir']).resolve(strict=True)
        manifest_path = root / 'manifest.json'
        if digest(manifest_path) != source.get('manifest_digest'):
            raise OperatorError('registered_manifest_digest_mismatch')
        manifest = json.loads(manifest_path.read_text())
        self.verify_bundle(root, manifest)
        self.command(['docker', 'image', 'load', '--input', str(root / 'deployment-images.tar')], 'load_archived_images')
        created_volumes = {logical: name + '_' + logical for logical in VOLUMES}
        # Preflight every target before creating any. Existing targets remain
        # untouched; interrupted restores require an explicit new attempt/name.
        for target in [*created_volumes.values()]:
            result = subprocess.run(['docker', 'volume', 'inspect', target], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
            if result.returncode == 0:
                raise OperatorError('restore_target_already_exists')
        result = subprocess.run(['docker', 'network', 'inspect', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
        if result.returncode == 0:
            raise OperatorError('restore_network_already_exists')
        self.record('creating_isolated_restore')
        self.command(['docker', 'network', 'create', '--internal','--label','fluctlight.restore_operation='+self.restore_nonce, name], 'create_isolated_network')
        for logical, target in created_volumes.items():
            self.command(['docker', 'volume', 'create', '--label', 'fluctlight.restore=' + self.args.job,'--label','fluctlight.restore_operation='+self.restore_nonce, target], 'create_isolated_volume')
            actual = json.loads(self.command(['docker','volume','inspect',target],'inspect_new_target_volume'))[0]
            if actual.get('Labels',{}).get('fluctlight.restore_operation')!=self.restore_nonce:
                raise OperatorError('restore_target_not_created_by_this_operation')
            self.command(['docker', 'run', '--rm', '--network', 'none', '--user', '0:0',
                          '--mount', f'type=volume,src={target},dst=/target', '--mount', f'type=bind,src={root},dst=/backup,readonly',
                          manifest['utility_image'], 'tar', '-xpf', '/backup/' + logical + '.tar', '-C', '/target'], 'restore_isolated_volume')
        # Reuse exactly the original startup values and object version storage,
        # but bind all infra hostnames only inside the new internal network.
        runtime = {}
        for line in (root / 'runtime.env').read_text().splitlines():
            key, value = line.split('=', 1)
            runtime[key] = value
        runtime['FLUCTLIGHT_ISOLATED_RESTORE'] = 'true'
        runtime_path = self.output / 'isolated-runtime.env'
        safe_env(runtime_path, runtime)
        from urllib.parse import urlsplit, unquote
        database_url = urlsplit(runtime['CORE_GO_DATABASE_URL'])
        if database_url.hostname != 'postgres':
            raise OperatorError('restore_database_host_not_isolated')
        db_values = {'POSTGRES_USER': unquote(database_url.username or ''), 'POSTGRES_PASSWORD': unquote(database_url.password or ''), 'POSTGRES_DB': unquote(database_url.path.lstrip('/'))}
        postgres_env = self.output / 'isolated-postgres.env'
        safe_env(postgres_env, db_values)
        minio_env = self.output / 'isolated-minio.env'
        safe_env(minio_env, {'MINIO_ROOT_USER': runtime['S3_ACCESS_KEY'], 'MINIO_ROOT_PASSWORD': runtime['S3_SECRET_KEY']})
        pg = name + '_postgres'
        minio = name + '_minio'
        self.target_containers.append(pg)
        self.command(['docker', 'run', '-d', '--label','fluctlight.restore_operation='+self.restore_nonce,'--name', pg, '--network', name, '--network-alias', 'postgres', '--env-file', str(postgres_env),
                      '--mount', f"type=volume,src={created_volumes['fluctlight_postgres']},dst=/var/lib/postgresql/data", manifest['images']['postgres']], 'start_isolated_postgres')
        self.target_containers.append(minio)
        self.command(['docker', 'run', '-d','--label','fluctlight.restore_operation='+self.restore_nonce,'--name', minio, '--network', name, '--network-alias', 'minio', '--env-file', str(minio_env),
                      '--mount', f"type=volume,src={created_volumes['fluctlight_minio']},dst=/data", manifest['images']['minio'], 'server', '/data', '--console-address', ':9001'], 'start_isolated_minio')
        ready = False
        for _ in range(60):
            result = subprocess.run(['docker', 'exec', pg, 'sh', '-c', 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
            storage_ready = subprocess.run(['docker', 'exec', minio, 'curl', '--fail', '--silent', 'http://127.0.0.1:9000/minio/health/live'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
            if result.returncode == 0 and storage_ready.returncode == 0:
                ready = True
                break
            time.sleep(1)
        if not ready:
            raise OperatorError('isolated_postgres_not_ready')
        # S3 readiness is exercised by the actual probe and retained as a
        # failure if unavailable; no empty inventory substitutes for it.
        restored = self.output / 'restored-probe.jsonl'
        header, footer = self.probe(restored, network=name, environment=runtime_path)
        if digest(restored) != digest(root / 'source-probe.jsonl'):
            raise OperatorError('restored_database_or_media_differs')
        upgrade = self.ctl('upgrade', network=name, environment=runtime_path)
        private_json(self.output / 'upgrade-result.json', upgrade)
        result = {'restorable': False, 'source_inventory_verified': True, 'restore_comparison_verified': True, 'artifacts_verified': True, 'database_counts_verified': True, 'media_verified': True,
                  'source_head': header['migration_head'], 'upgrade_head': upgrade['migration_head'], 'media_count': footer['media_checked'],
                  'legacy_media_versions': footer['legacy_media_versions'], 'isolated_target': name,
                  'manifest_digest': digest(manifest_path), 'models_started': False}
        self.ctl('audit-restore', {'source_backup_id': manifest['job_id'], 'target_name': name, 'result': result,
                                   'private': {'artifact_dir': str(self.output), 'source_artifact_dir': str(root)}},
                 network=name, environment=runtime_path)
        private_json(self.output / 'restore-result.json', {'result': result, 'volumes': created_volumes, 'network': name, 'containers': self.target_containers})
        # Verification containers are left stopped. Source services and all
        # restored Temporal/API/Worker/model execution stay untouched.
        self.stop_verifiers()
        self.target_containers = []
        self.record('isolated_restore_verified', 'completed', result, {'artifact_dir': str(self.output), 'isolated_target': name})

    def stop_verifiers(self,best_effort=False):
        for name in self.target_containers:
            result=subprocess.run(['docker','inspect',name],stdout=subprocess.PIPE,stderr=subprocess.PIPE,check=False)
            if result.returncode:
                if best_effort:continue
                # A start response can be lost before the object was created.
                continue
            item=json.loads(result.stdout)[0]
            if item['Config'].get('Labels',{}).get('fluctlight.restore_operation')!=self.restore_nonce:
                if best_effort:continue
                raise OperatorError('verifier_container_ownership_mismatch')
            self.command(['docker','stop',name],'stop_isolated_verifiers',timeout=120,okay_missing=best_effort)

    def run(self):
        try:
            if self.claim['kind'] == 'backup':
                self.backup()
            elif self.claim['kind'] == 'restore':
                self.restore()
            else:
                raise OperatorError('unsupported_maintenance_kind')
        except Exception as error:
            stage = str(error) if isinstance(error, OperatorError) else 'operator_interrupted_or_failed'
            self.log(stage, status='failed')
            try:
                if self.stopped_source:
                    self.source_up_for_receipt()
                self.record(stage[:64], 'failed', {'models_started': False}, {'artifact_dir': str(self.output)})
            except Exception:
                self.log('failure_receipt_unconfirmed', status='unknown')
            raise
        finally:
            if self.target_containers:
                self.stop_verifiers(best_effort=True)


class ColdRestore(Operator):
    def __init__(self, args):
        self.args = args
        root = Path(args.bundle).resolve(strict=True)
        if not re.fullmatch(r'[0-9a-f]{64}', args.manifest_sha256 or '') or digest(root / 'manifest.json') != args.manifest_sha256:
            raise OperatorError('expected_manifest_digest_required')
        if not re.fullmatch(r'fluctlight_restore_[a-z0-9]{8,32}', args.target_name or ''):
            raise OperatorError('new_isolated_target_required')
        self.root = Path(args.workdir).absolute()
        if self.root.is_symlink() or ',' in str(self.root):
            raise OperatorError('private_backup_directory_required')
        self.root.mkdir(mode=0o700, parents=True, exist_ok=True)
        if self.root.stat().st_mode & 0o077:
            raise OperatorError('private_backup_directory_permissions_required')
        self.output = self.root / args.target_name
        self.output.mkdir(mode=0o700, exist_ok=False)
        self.journal = self.output / 'operator-journal.jsonl'
        self.network = 'none'
        self.image = args.core_image
        self.restore_image = args.core_image
        if not self.image:
            raise OperatorError('current_release_core_image_required')
        self.command(['docker', 'image', 'inspect', self.image], 'current_core_image_missing')
        self.runtime_file = root / 'runtime.env'
        self.claim = {'kind': 'restore', 'attempt': 1, 'target_name': args.target_name,
                      'source': {'artifact_dir': str(root), 'manifest_digest': args.manifest_sha256}}
        args.job = 'restore_cold_' + args.target_name.removeprefix('fluctlight_restore_')
        self.stopped_source = False
        self.target_containers = []

    def record(self, stage, status='running', result=None, private=None):
        self.log(stage, status=status)
        private_json(self.output / 'operator-result.json', {'id': self.args.job, 'stage': stage,
                     'status': status, 'result': result or {}, 'private': private or {}})
        return {'recorded_locally': True}


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--job')
    parser.add_argument('--bundle', help='standalone disaster recovery from a private complete artifact directory')
    parser.add_argument('--manifest-sha256')
    parser.add_argument('--target-name')
    parser.add_argument('--core-image', help='locally available current-release Core image for isolated verification and upgrade')
    parser.add_argument('--compose')
    parser.add_argument('--env-file')
    parser.add_argument('--workdir', required=True, help='private operator directory; artifacts and execution identities remain here')
    parser.add_argument('--utility-image', default='alpine:3.22')
    parser.add_argument('--maintenance-confirm', action='store_true', help='authorize stopping source writers for backup; source API/Worker are not automatically restarted')
    args = parser.parse_args()
    if not args.maintenance_confirm or (not args.bundle and (not args.compose or not args.env_file or not re.fullmatch(r'(backup|restore)_[A-Za-z0-9_-]{8,96}', args.job or ''))):
        parser.error('an Owner-created job and explicit --maintenance-confirm are required')
    try:
        operator = ColdRestore(args) if args.bundle else Operator(args)
        operator.run()
        print('maintenance_job_completed; inspect the actual job result; no API/Worker/model process was started')
    except Exception as error:
        print('maintenance_job_not_completed; inspect private operator-journal.jsonl and the original job', file=sys.stderr)
        return 1
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
