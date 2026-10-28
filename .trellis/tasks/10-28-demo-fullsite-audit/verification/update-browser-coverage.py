import json
from pathlib import Path
root=Path('.trellis/tasks/10-28-demo-fullsite-audit')
c=json.loads((root/'coverage.json').read_text());rows={r['id']:r for r in c['rows']}
# Record exact cases without promoting unexecuted states or all-page completion.
def add(ids,dimension,state,status,evidence,notes=''):
 for id in ids:
  r=rows[id];r.setdefault('state_verification',{}).setdefault(dimension,{})[state]={'status':status,'evidence':evidence,'notes':notes}
  for p in evidence:
   if p not in r['evidence'][dimension]:r['evidence'][dimension].append(p)
visual={'library':['library-hero','library-cards','library-search-view'],'chat':['chat-room','chat-send','chat-message-cards','chat-now-preview'],'now':['life-now-current','life-now-schedule'],'schedule':['life-schedule-timeline'],'wardrobe':['resources.wardrobe.garments'],'goals':['life-goals-list'],'diary':['life-diary-read'],'profile':['profile-hero','profile-overview'],'world':['resources.world.directory'],'people':['people-directory'],'feed':['feed-filter-list'],'media':['resources.media.gallery'],'studio':['resources.studio.directory'],'inbox':['inbox-list-read','inbox-pending'],'creation':['creation-world-existing'],'memory':['life-memory-list'],'settings':['management-navigation','management-model-services']}
for page,ids in visual.items():
 phase='final-pass94' if page=='media' else 'final-pass99' if page in ['inbox','memory','studio'] else 'final-pass85' if page!='creation' else 'normal-pass4'
 for size in ['desktop','mobile']:
  evidence=[f'evidence/visual/{page}/{phase}/reference-{size}.png',f'evidence/visual/{page}/{phase}/actual-{size}.png']
  add(ids,'visual','正常-'+size,'发现问题',evidence,'最新对应视口截图；标题坐标已核对，共享修复及正式契约/密度差异见fixes.md、accepted-differences.md。仍有未逐张完验细节，未将整行全部状态标通过。')
interaction={
 'library-search-view-back':['library-search-view'],
 'shell-keyboard-navigation':['shell-jump'],
 'chat-input-image-history':['chat-send','chat-history','chat-attachment','chat-now-preview'],
 'chat-image-error-recovery':['chat-attachment'],
 'shell-mobile-drawer':['shell-sidebar'],
 'profile-edit-cancel':['profile-edit-basic'],
 'schedule-detail-date':['life-schedule-timeline','life-schedule-detail'],
 'goals-search-edit':['life-goals-list','life-goals-edit'],
 'wardrobe-search-detail':['resources.wardrobe.garments','resources.wardrobe.garment.editor','resources.wardrobe.outfits'],
 'diary-read-annotation':['life-diary-read','life-diary-annotation','life-diary-shared'],
 'memory-source-detail':['life-memory-detail','life-reflection-list','life-self-list'],
 'world-search-detail':['resources.world.directory','resources.world.venues'],
 'people-about-detail':['people-directory','people-detail','people-about-display'],
 'feed-read-detail':['feed-detail-comments'],
 'media-read-detail':['resources.media.detail'],
 'studio-resource-editor':['resources.studio.editor.world'],
 'inbox-read-sync':['inbox-list-read','inbox-pending']}
files=['evidence/interaction/workspace-ui.json','evidence/interaction/domain-ui.json','evidence/interaction/domain-ui-schedule-detail-date-wardrobe-search-detail-world-search-detail-studio-resource-editor.json','evidence/interaction/domain-ui-wardrobe-search-detail.json']
latest={}
for file in files:
 for result in json.loads((root/file).read_text()):latest[result['name']]=(file,result)
for name,(file,result) in latest.items():
 if name not in interaction:continue
 add(interaction[name],'interaction',name,'已验证' if result['status']=='passed' else '发现问题',[file],'; '.join(result['checks'])+'。仅限该脚本明确执行的状态，受控HTTP，不是实时Core。')
state_rows={'now':['life-now-current','life-now-schedule'],'wardrobe':['resources.wardrobe.garments'],'profile':['profile-hero','profile-overview'],'settings':['management-navigation','management-model-services'],'chat':['chat-room','chat-message-cards'],'schedule':['life-schedule-timeline'],'goals':['life-goals-list'],'diary':['life-diary-read'],'memory':['life-memory-list'],'world':['resources.world.directory'],'people':['people-directory'],'feed':['feed-filter-list'],'media':['resources.media.gallery'],'studio':['resources.studio.directory'],'inbox':['inbox-list-read']}
state_file='evidence/states/read-states.json'
states={(r['page'],r['state']):(state_file,r) for r in json.loads((root/state_file).read_text())}
for extra in sorted((root/'evidence/states').glob('read-states-*.json'),key=lambda p:p.stat().st_mtime):
 if extra.name=='read-states-consolidated.json':continue
 for result in json.loads(extra.read_text()):states[(result['page'],result['state'])]=(str(extra.relative_to(root)),result)
for (page,state),(file,result) in states.items():
 add(state_rows[page],'interaction','窄屏-'+state,'已验证' if result['status']=='passed' else '发现问题',[file],'; '.join(result['checks']))
 add(state_rows[page],'visual','窄屏-'+state,'待检查',[file],'实际状态截图已生成，无横向溢出；逐张人工视觉复核仍未全部完成，不由脚本结果自动标视觉通过。')
# Deeper actual-browser cases: each entry is scoped to its own recorded assertions.
extra_maps={
 'evidence/interaction/settings-studio-ui.json':lambda name:['management-navigation','management-model-services','management-model-roles','management-conversation','management-media','management-autonomy','management-quiet','management-runtime','management-advanced'] if name.startswith('settings-') else ['resources.studio.editor.'+name.removeprefix('studio-editor-')] if name.startswith('studio-editor-') else ['shell-sidebar'],
 'evidence/interaction/settings-studio-ui-auth-validation-failure.json':lambda name:['auth-formal-init'],
 'evidence/creation/creation-ui.json':lambda name:['creation-blank','creation-person-input'] if name.startswith('source-blank-') else ['creation-person-input','creation-'+({'json':'json-import'}.get(name.split('-')[1],name.split('-')[1])),'creation-analysis','creation-review-identity','creation-review-personality','creation-review-life','creation-review-relations','creation-review-json'] if name.startswith('source-') else ['creation-analysis'],
 'evidence/creation/creation-ui-preview-trait-edit-preservation.json':lambda name:['creation-review-personality','creation-review-json'],
 'evidence/preparation/preparation-ui.json':lambda name:['creation-seven-stages','creation-activation']+(['creation-inventory-recovery'] if name in ['wardrobe-failure','recovery-unknown'] else []),
 'evidence/form-groups/report-after.json':lambda name:['feed-publish','feed-suggest'] if name=='feed-compose' else ['people-create'] if name=='people-create' else ['people-formal-management','people-detail'],
 'evidence/interaction/visual-ui.json':lambda name:['resources.media.visual.brief'] if name.startswith('visual-definition-') else ['resources.media.visual.candidate','resources.media.visual.review'] if name.startswith('candidate-review-') else ['resources.media.avatar'] if name.startswith('avatar-') else ['resources.media.generation'] if name.startswith('generation-') else ['resources.media.gallery']}
extra_maps.update({
 'evidence/memory-deep/report-pass26.json':lambda name:['life-reflection-list','life-reflection-detail'] if name=='reflection-feedback' else ['life-self-list','life-self-detail','formal-self-governance'] if name=='self-governance' else ['life-memory-correct','life-memory-forget','life-memory-restore','life-memory-detail'] if name=='memory-governance' else ['life-memory-list','life-memory-detail'] if name=='filter-sources' else ['life-memory-correct','life-memory-detail'],
 'evidence/memory-deep/report-pass29.json':lambda name:['life-memory-correct','life-memory-detail'],
 'evidence/studio-deep/report-after-pass29.json':lambda name:['resources.studio.editor.'+name,'resources.studio.lifecycle','resources.studio.apply'],
 'evidence/world-deep/report-pass31.json':lambda name:['resources.world.venues','resources.world.place.editor','resources.world.place.history'] if name=='place-history-restore' else ['resources.world.items'] if name=='item-availability' else ['resources.world.events'],
 'evidence/studio-sandbox/report-pass35.json':lambda name:['resources.studio.sandbox'],
 'evidence/diary-deep/report-consolidated.json':lambda name:['life-diary-reply','life-diary-read'] if name=='reply-request' else ['life-diary-generate','life-diary-read'] if name=='past-generation' else ['life-diary-shared'] if name=='shared-source' else ['life-diary-annotation','life-diary-read']+(['formal-diary-favorite'] if name=='annotation-search' else []),
 'evidence/schedule-deep/report-consolidated.json':lambda name:['life-schedule-plan','life-schedule-timeline'] if name.startswith('plan') else ['life-schedule-suggestions','life-schedule-detail','life-schedule-timeline'],
 'evidence/media-deep/report-consolidated.json':lambda name:['resources.media.visual.brief'] if name=='brief-save' else ['resources.media.visual.review','resources.media.visual.candidate']+(['resources.media.visual.sheet'] if name=='review-accept' else []) if name.startswith('review') else ['resources.media.avatar'] if name.startswith('avatar') else ['resources.media.generation','resources.media.detail'],
 'evidence/real-ui-upload/report.json':lambda name:['resources.media.upload','resources.media.detail'],
 'evidence/preparation-read/report-consolidated.json':lambda name:['creation-seven-stages','creation-activation','creation-world-daily','creation-blank']+(['creation-visual-review'] if name=='awaiting-review' else []),
 'evidence/wardrobe-deep/report-pass48.json':lambda name:['resources.wardrobe.outfits','resources.wardrobe.compose','resources.wardrobe.wear'] if name=='outfit-wear' else ['resources.wardrobe.garment.editor','resources.wardrobe.garments'],
 'evidence/admin-deep/report-consolidated.json':lambda name:['management-diagnostic-tasks','management-diagnostic-detail'] if name=='task-source' or name.startswith('task-read-') else ['management-diagnostic-detail','management-diagnostic-action-form'] if name.startswith('task-') else ['management-model-calls','management-diagnostic-health'] if name=='calls-and-health' else ['management-data-overview','management-data-import','formal-data-import-pack','resources.formal.studio.packs'] if name=='data-validation-cancel' else ['management-data-backups','management-data-restore','formal-backup-isolated-detail'] if name=='backup-details-cancel' else ['formal-data-revisions','formal-data-rollback-boundary'],
 'evidence/profile-people/report-consolidated.json':lambda name:['profile-personas','profile-life','profile-history-source'] if name=='profile-read' else ['profile-edit-'+name.removeprefix('edit-')] if name.startswith('edit-') else ['profile-switch-persona','profile-personas'] if name=='persona-switch' else ['profile-foundation-proposal'] if name=='foundation-governance' else ['profile-capabilities'] if name=='capability-review' else ['people-about-edit','people-about-display','people-authored-history'] if name=='people-about-save' else ['people-knowledge','people-relation-basis','people-relationship-tab'],
 'evidence/feed-goals/report-consolidated.json':lambda name:['feed-publish','feed-detail-comments'] if name.startswith('feed-publish') else ['feed-suggest'] if name=='feed-suggest' else ['feed-post-actions','feed-detail-comments'] if name=='feed-comment-actions' else ['feed-detail-comments'] if name=='feed-comment-readback' else ['life-goals-detail'] if name=='goal-read' else ['life-goals-state','life-goals-detail'] if name.startswith('goal-state') else ['life-goals-edit','life-goals-detail'],
 'evidence/creation-supplement/report-consolidated.json':lambda name:['creation-world-published'] if name.startswith('published') else ['creation-structured-editor','creation-review-life','creation-review-json'] if name=='structured-editor' else ['creation-world-conflict'] if name=='world-conflict' else ['creation-complete-navigation','creation-start-snapshot'] if name=='snapshot-completion' else ['formal-creation-rejected-copy'] if name=='rejected-copy' else ['resources.studio.creation'],
 'evidence/chat-inbox-avatar/report-consolidated.json':lambda name:['chat-turn-recovery'] if name=='retry-original' or name.startswith('stop-') else ['chat-history-formal','chat-history'] if name=='history-recovery' else ['inbox-detail-source','inbox-task-recover'] if name=='inbox-source' else ['inbox-back-chat'] if name=='inbox-back' else ['inbox-detail-source'] if name=='inbox-source-errors' else ['inbox-formal-groups'] if name=='inbox-groups' else ['avatar-source-all'],
 'evidence/retained-tools/report-consolidated.json':lambda name:['management-vision'] if name=='management-vision' else ['management-notifications'] if name=='management-notifications' else ['formal-config-extra'] if name=='management-extra' else ['life-schedule-habits'] if name.startswith('habits') else ['formal-life-timeline'] if name=='life-timeline' else ['formal-goal-planning'] if name=='goal-planning' else ['resources.world.settings'] if name=='world-settings' else ['resources.wardrobe.tags'],
 'evidence/media-lifecycle/report-consolidated.json':lambda name:['resources.formal.media.lifecycle'],
 'evidence/formal-edges/report-consolidated.json':lambda name:['profile-self-claims'] if name=='profile-self' else ['profile-formal-recovery'] if name=='profile-readback' else ['feed-formal-npc'] if name=='feed-npc' else ['formal-diagnostic-export-clear'],
 'evidence/responsive-layout/report-consolidated.json':lambda name:['management-navigation','management-advanced'] if name=='management' else ['inbox-list-read','inbox-pending'] if name=='inbox' else ['life-diary-read','formal-diary-favorite'] if name=='diary' else ['resources.studio.directory'] if name=='studio' else ['people-directory','people-detail'] if name=='people' else ['life-goals-list','life-goals-detail'] if name=='goals' else ['life-memory-list'] if name=='memory' else ['life-schedule-timeline','life-schedule-detail'],
 'evidence/status-presentation/report-consolidated.json':lambda name:['resources.studio.directory','resources.studio.lifecycle'] if name=='studio-status' else ['inbox-list-read','inbox-formal-groups'],
 'evidence/shell-deep/report-consolidated.json':lambda name:['shell-navigation'] if name=='navigation-pages' else ['shell-switch'] if name=='character-switch' else ['shell-navigation','shell-switch','library-empty-errors'] if name=='empty-directory' else ['shell-sidebar'] if name=='sidebar-focus' else ['chat-say-prefill','chat-send'] if name=='say-existing-draft' else ['shell-account','auth-demo-form'] if name=='account-logout' else ['shell-formal-routing'],
})
for file,ids in extra_maps.items():
 if not (root/file).exists():continue
 for result in json.loads((root/file).read_text()):
  name=result.get('name') or result.get('scenario');result.setdefault('checks',['rendered group fills form width','20px flex stack gap','cancel without a write','no horizontal overflow'] if file=='evidence/form-groups/report-after.json' else []);targets=[id for id in ids(name) if id in rows]
  state=name+('-'+result['size'] if isinstance(result.get('size'),str) else '')
  add(targets,'interaction',state,'已验证' if result['status']=='passed' else '发现问题',[file],'; '.join(result['checks'])+'。只证明记录的断言，不代表该矩阵整行所有状态已完成。')
real='evidence/real-browser/report.json'
if (root/real).exists():
 report=json.loads((root/real).read_text())
 add(['library-cards','chat-room','life-now-current','resources.studio.directory','inbox-list-read'],'business','真实隔离Core/PG浏览器读取','已验证' if report.get('status')=='passed' else '发现问题',[real],'现有Owner和角色；5页面实际API读取，无聊天发送/模型生成/新激活。本项不证明各页全部写操作。')
upload='evidence/real-ui-upload/report.json'
if (root/upload).exists():
 for result in json.loads((root/upload).read_text()):
  add(['resources.media.upload','resources.media.detail'],'business','真实上传-'+result['size'],'已验证' if result['status']=='passed' else '发现问题',[upload],'; '.join(result['checks'])+'。仅已有隔离Web的上传UI与原始PNG读取；不证明视觉身份/图片生成/模型能力。')
# The supplied Demo-only reset/simulator controls have no production operation.
for id in ['shell-help-reset','management-data-demo-reset','creation-demo-scenarios']:
 r=rows[id]
 for field in ['visual_status','interaction_status','business_status']:r[field]='明确不适用'
 r['not_applicable_reason']='仅Demo的说明/模拟状态/恢复演示种子入口；正式产品不含Demo store或重置生活数据命令，依据当前workspace-replica-contract及accepted-differences，正常截图与生产隔离守卫证明未复制。'
(root/'coverage.json').write_text(json.dumps(c,ensure_ascii=False,indent=2)+'\n')
(root/'evidence/states/read-states-consolidated.json').write_text(json.dumps([r for _,r in states.values()],ensure_ascii=False,indent=2)+'\n')
md=['# 全站覆盖矩阵','',f'初始 {len(rows)} 行。行级视觉、交互、业务状态保持独立；本轮执行细节在每行 state_verification，不以单个正常页或一次测试覆盖整行全部状态。','', '|ID|页面/入口|视觉|交互|业务|本轮已登记具体核验项|','|---|---|---|---|---|---|']
for r in c['rows']:
 count=sum(len(v) for v in r.get('state_verification',{}).values())
 md.append('|'+ '|'.join([r['id'],r.get('demo_entry',r.get('area','')).replace('|','/'),r['visual_status'],r['interaction_status'],r['business_status'],str(count)])+'|')
md+=['','完整字段、状态、调用链、问题及证据路径见 [coverage.json](coverage.json)。']
(root/'coverage.md').write_text('\n'.join(md)+'\n')
print(json.dumps({'rows':len(rows),'rows_with_browser_evidence':sum(bool(r.get('state_verification')) for r in rows.values()),'state_cases':len(states),'state_cases_passed':sum(r['status']=='passed' for _,r in states.values()),'workflow_cases':len(latest),'workflow_cases_passed':sum(r['status']=='passed' for _,r in latest.values())},ensure_ascii=False))
