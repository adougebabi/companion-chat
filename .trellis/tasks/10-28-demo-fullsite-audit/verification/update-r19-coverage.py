"""Merge exact r19 passes without accepting entire fullsite rows or scenarios."""
import json
from pathlib import Path

task = Path(__file__).resolve().parents[1]
reports = [
    'evidence/r19/r19-reviewed-after/report.json',
    'evidence/r19/r19-targeted-after/report.json',
    'evidence/r19/r19-preview-final-after/report.json',
]
records = {}
for path in reports:
    for index, record in enumerate(json.loads((task / path).read_text())):
        if record['status'] == 'passed':
            records[record['name'], record['size']] = {
                **record, 'source_report': path, 'source_record_index': index,
            }
assert len(records) == 33
(task / 'evidence/r19-browser-index.json').write_text(
    json.dumps(list(records.values()), ensure_ascii=False, indent=2) + '\n')

mapping = {
    'settings-entry': ['shell-navigation', 'management-navigation', 'management-diagnostic-tasks', 'management-model-calls'],
    'read-states': ['management-model-services', 'management-navigation'],
    'settings-fields': ['management-model-roles', 'management-conversation', 'management-runtime', 'management-advanced'],
    'data-layout': ['management-data-overview'],
    'world-studio': ['resources.world.directory', 'resources.studio.directory'],
    'studio-form': ['resources.studio.editor.world'],
    'studio-sandbox': ['resources.studio.sandbox'],
    'media-empty': ['resources.media.gallery', 'resources.media.visual.brief'],
    'command-actions': ['feed-publish', 'feed-detail-comments'],
    'blank-initialization': ['creation-world-daily', 'creation-blank', 'creation-activation', 'creation-inventory-recovery', 'management-diagnostic-detail'],
    'preview-chinese': ['creation-json-import', 'creation-review-life', 'creation-review-relations', 'creation-review-json'],
}
coverage = json.loads((task / 'coverage.json').read_text())
ids = {row['id'] for row in coverage['rows']}
assert len(ids) == 177
assert all(value in ids for values in mapping.values() for value in values), set(sum(mapping.values(), [])) - ids
observed = set()
for row in coverage['rows']:
    observations = []
    for (name, size), record in records.items():
        if row['id'] not in mapping[name]:
            continue
        observed.add(row['id'])
        observations.append({
            'key': f'r19:{name}:{size}', 'status': '已观察并定向验证',
            'source_report': record['source_report'],
            'source_record_index': record['source_record_index'],
            'checks': record['checks'],
            'boundary': '仅该受控HTTP/正式构建案例；原三维状态及八场景状态保持，不代表真实Core/模型或整行验收。',
        })
    if observations:
        row['feedback_ux']['r19_observations'] = observations
coverage['r19_increment'] = {
    'issue_report': 'r19-acceptance.md', 'issue_records': 'r19-issues.json',
    'browser_index': 'evidence/r19-browser-index.json', 'observed_rows': len(observed),
    'note': '只追加精确观察；保留177行历史、三维状态和八场景状态。图片仅在仓库外。',
}
(task / 'coverage.json').write_text(json.dumps(coverage, ensure_ascii=False, indent=2) + '\n')
print(f'33 r19 cases, {len(observed)} exact matrix rows; original statuses retained')
