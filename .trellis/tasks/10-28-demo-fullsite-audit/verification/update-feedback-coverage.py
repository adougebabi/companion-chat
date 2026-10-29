"""Add exact feedback observations; never promote a whole row from one case."""
import json
from pathlib import Path

task = Path(__file__).resolve().parents[1]

def read(rel):
    return json.loads((task / rel).read_text())

def merge_reports(paths, key, output, expected, accepted):
    merged = {}
    for path in paths:
        for index, source in enumerate(read(path)):
            if source['status'] not in accepted:
                continue
            record = dict(source)
            record['source_report'] = path
            record['source_record_index'] = index
            merged[tuple(record.get(part) for part in key)] = record
    assert len(merged) == expected, (output, len(merged), expected)
    target = task / output
    target.parent.mkdir(parents=True, exist_ok=True)
    records = list(merged.values())
    target.write_text(json.dumps(records, ensure_ascii=False, indent=2) + '\n')
    return records

reads = {}
for version in ['before', 'after']:
    reads[version] = merge_reports([
        f'evidence/feedback-read-{version}-feedback2/report.json',
        f'evidence/feedback-read-{version}-feedback3/report.json',
    ], ['page', 'state', 'size'], f'evidence/feedback-read-final/{version}.json', 120,
        {'passed'} if version == 'after' else {'captured'})

configs = {}
for version in ['before', 'after']:
    paths = [f'evidence/feedback-config-{version}-feedback2/report.json',
             f'evidence/feedback-config-{version}-feedback3/report.json']
    if version == 'after':
        paths.append('evidence/feedback-config-after-feedback5/report.json')
    configs[version] = merge_reports(paths, ['name', 'size'],
        f'evidence/feedback-config-final/{version}.json', 10,
        {'passed'} if version == 'after' else {'captured'})

entry = read('evidence/feedback-entry-after-feedback3/report.json') + read('evidence/feedback-entry-after-feedback4/report.json')
management = read('evidence/management-state-edges-feedback-after3/report.json')
assert len(entry) == 28 and all(r['status'] == 'passed' for r in entry)
assert len(management) == 14 and all(r['status'] == 'passed' for r in management)
deep = {}
for group, expected, phases in [('people-state-edges', 10, ['feedback-after1', 'feedback-after2']),
                               ('feed-state-edges', 8, ['feedback-after2']),
                               ('shell-chat-inbox-states', 12, ['feedback-after1', 'feedback-after2', 'feedback-after3']),
                               ('wear-request-states', 10, ['feedback-after1'])]:
    deep[group] = merge_reports([f'evidence/{group}-{phase}/report.json' for phase in phases],
        ['name', 'size'], f'evidence/feedback-deep-final/{group}-after.json', expected, {'passed'})
    merge_reports([f'evidence/{group}-feedback-before1/report.json'], ['name', 'size'],
        f'evidence/feedback-deep-final/{group}-before.json', expected, {'passed'})

coverage = read('coverage.json')
original_ids = [r['id'] for r in coverage['rows']]
rows = {r['id']: r for r in coverage['rows']}

def add(row_ids, scenario, record, report, index, label):
    for row_id in row_ids:
        feedback = rows[row_id]['feedback_ux']
        observation = dict(status='已验证', evidence=report, record_index=index,
            case=record.get('name', record.get('page')), state=record.get('state'),
            size=record['size'], checks=record['checks'], screenshots=record['screenshots'],
            boundary=record['boundary'], notes=label + '。只证明所列断言；该行八类状态和逐张视觉验收不自动通过。')
        key = '-'.join(str(v) for v in [observation['case'], observation['state'], record['size']] if v)
        feedback['scenarios'][scenario].setdefault('observations', {})[key] = observation
        if report not in feedback['scenarios'][scenario]['evidence']:
            feedback['scenarios'][scenario]['evidence'].append(report)

reader_rows = dict(now='life-now-current', wardrobe='resources.wardrobe.garments',
    profile='profile-hero', settings='management-navigation', chat='chat-room',
    schedule='life-schedule-timeline', goals='life-goals-list', diary='life-diary-read',
    memory='life-memory-list', world='resources.world.directory', people='people-directory',
    feed='feed-filter-list', media='resources.media.gallery', studio='resources.studio.directory', inbox='inbox-list-read')
scenarios = dict(normal='normal', empty='empty_first', loading='async_progress', failure='read_write_failure')
for index, record in enumerate(reads['after']):
    add([reader_rows[record['page']]], scenarios[record['state']], record,
        'evidence/feedback-read-final/after.json', index, '主页面实际读取状态及同端点恢复；不代表其编辑/后台任务子流程')

for index, record in enumerate(entry):
    name = record['name']
    report = 'evidence/feedback-entry-after-feedback4/report.json' if name == 'awaiting-review' else 'evidence/feedback-entry-after-feedback3/report.json'
    source_index = index - 26 if name == 'awaiting-review' else index
    scenario = 'unconfigured' if name == 'unconfigured' else 'failed_unknown_recovery' if name in ['unknown-activation', 'awaiting-review'] else 'draft_restore'
    row_ids = ['library-hero', 'library-empty-errors']
    if name == 'unknown-activation': row_ids += ['creation-activation']
    if name == 'awaiting-review': row_ids += ['creation-seven-stages', 'creation-visual-review']
    if name == 'unconfigured': row_ids += ['creation-blank']
    add(row_ids, scenario, record, report, source_index, '精确本机记录分类、真实操作、原记录保留/原激活一次')

for index, record in enumerate(configs['after']):
    target = {'stale': 'management-model-services', 'workflow': 'management-media',
              'policy': 'management-quiet', 'policy-restored': 'management-quiet',
              'storage-runtime': 'management-navigation'}[record['name']]
    scenario = 'draft_restore' if record['name'] == 'stale' else 'read_write_failure' if record['name'] in ['workflow', 'storage-runtime'] else 'failed_unknown_recovery'
    add([target], scenario, record, 'evidence/feedback-config-final/after.json', index,
        '字段/存储/角色策略精确恢复，成功消息到期与真实原请求身份')

for index, record in enumerate(management):
    name = record['name']
    targets = ['management-vision'] if name.startswith('settings-') else ['management-data-backups', 'formal-backup-isolated-detail'] if name == 'backup-states' else ['management-data-import', 'formal-data-import-pack']
    if name == 'settings-partial': targets += ['management-model-services', 'management-model-roles']
    add(targets, 'async_progress' if name in ['settings-busy', 'backup-states'] else 'failed_unknown_recovery',
        record, 'evidence/management-state-edges-feedback-after3/report.json', index,
        '配置或数据工具原提交、实际回读与同位置反馈；受控HTTP不认证真实配置/维护执行')

deep_targets = {
 'people-state-edges': {
  'incomplete': (['people-directory'], ['read_write_failure']),
  'foreign-world': (['people-directory'], ['read_write_failure']),
  'foreign-place': (['people-directory'], ['read_write_failure']),
  'read-only': (['people-detail', 'people-about-display'], ['normal']),
  'about-conflict': (['people-about-edit'], ['draft_restore', 'read_write_failure', 'failed_unknown_recovery'])},
 'feed-state-edges': {
  'suggestion-states': (['feed-suggest'], ['async_progress', 'failed_unknown_recovery']),
  'suggestion-readback': (['feed-suggest'], ['read_write_failure', 'failed_unknown_recovery']),
  'npc-states': (['feed-formal-npc'], ['normal']),
  'comment-read-failure': (['feed-detail-comments'], ['read_write_failure'])},
 'shell-chat-inbox-states': {
  'logout-failure': (['shell-account'], ['read_write_failure']),
  'quickjump-empty': (['shell-jump'], ['empty_first']),
  'inbox-bulk': (['inbox-list-read'], ['async_progress']),
  'inbox-no-role': (['inbox-back-chat'], ['empty_first']),
  'chat-composition': (['chat-send'], ['normal']),
  'chat-states': (['chat-turn-recovery'], ['async_progress', 'failed_unknown_recovery'])},
 'wear-request-states': {name: (['resources.wardrobe.wear'], ['failed_unknown_recovery'])
    for name in ['failed', 'denied', 'no-change', 'unknown', 'readback-failure']}}
for group, records in deep.items():
    for index, record in enumerate(records):
        targets, checked_scenarios = deep_targets[group][record['name']]
        for scenario in checked_scenarios:
            add(targets, scenario, record, f'evidence/feedback-deep-final/{group}-after.json', index,
                '保留原交互断言并新增各截图的反馈DOM语义；尚需逐实例文案/位置/重复/视觉人工签收')

assert len(coverage['rows']) == 177 and [r['id'] for r in coverage['rows']] == original_ids
(task / 'coverage.json').write_text(json.dumps(coverage, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(dict(read_cases=120, entry_cases=28, management_cases=14,
    config_cases=10, deep_cases=40, rows_with_observations=sum(any(s.get('observations') for s in r['feedback_ux']['scenarios'].values()) for r in coverage['rows']),
    matrix_rows=177, whole_row_feedback_status='未批量提升'), ensure_ascii=False))
