import {useIsMobile} from '../hooks/use-mobile';
import {ChevronLeft,ChevronRight} from 'lucide-react';
import {modelCallGroups} from '@/approved-ui/task-model-call-presentation.mjs';
import {modelRouteGroups,modelRouteLabel} from '@/approved-ui/model-route-presentation.mjs';
import {Select,SelectTrigger,SelectValue,SelectContent,SelectGroup,SelectLabel,SelectItem} from '@/components/ui/select';
import SettingsSections from '../shared/SettingsSections';
import DiagnosticsWorkspace from "@/react-app/shared/DiagnosticsWorkspace";
import {useRef,useState,useEffect} from 'react';
import WorkspaceDialog from '../shared/WorkspaceDialog';
import WorkspacePageHeader from "@/react-app/shared/WorkspacePageHeader";
import {managementCategories as categories} from '@/approved-ui/management-navigation.mjs';
import {Server} from 'lucide-react';
import RecordView from '@/react-app/shared/RecordView';
import CursorControls from '@/react-app/shared/CursorControls';
import {Field,FieldLabel} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {Switch} from '@/components/ui/switch';
import {Button} from '@/components/ui/button';
import {MoreHorizontal} from 'lucide-react';
import {DropdownMenu,DropdownMenuTrigger,DropdownMenuContent,DropdownMenuItem,DropdownMenuSeparator} from '@/components/ui/dropdown-menu';
import {InlineFeedback as OperationStatus} from "@/react-app/shared/Presentation";
import {Feedback,InlineFeedback} from "@/react-app/shared/Presentation";
import ModelCallDetails from '@/react-app/shared/ModelCallDetails';
import {Collapsible,CollapsibleContent,CollapsibleTrigger} from '@/components/ui/collapsible';
import JsonDetails from '@/react-app/shared/JsonDetails';
import {ScrollArea} from '@/components/ui/scroll-area';
import {ToggleGroup,ToggleGroupItem} from '@/components/ui/toggle-group';
import DiagnosticRecords from '@/react-app/shared/DiagnosticRecords';
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { readValue, elementRef } from "@/state/view-model";
import { cn } from "@/lib/utils";
import {
  ActionButton,
  Surface,
  FieldInput,
  ChoiceSelect,
  Disclosure,
  Badge,
} from "@/react-app/controls";
import {
  displayText,
  eventModifiers,
  listValues,
  Fragment,
  numberInput,
} from "@/react-app/render-helpers";
const Yi = ["周一", "周二", "周三", "周四", "周五", "周六", "周日"];
export function ManagementPageView({ view: view }) {
  const mobile=useIsMobile(), [directory,setDirectory]=useState(() => !new URLSearchParams(location.hash.split('?')[1] || '').has('section'));
  useEffect(() => {const sync=()=>setDirectory(!new URLSearchParams(location.hash.split('?')[1] || '').has('section'));window.addEventListener('hashchange',sync);return()=>window.removeEventListener('hashchange',sync);},[]);
  const recordTrigger=useRef(null),managementNav=useRef(null),[callTechnicalTarget,setCallTechnicalTarget]=useState(null);
  const callGroup=view.selected.value?modelCallGroups(view.rows.value.map(record=>record._record)).find(group=>group.records.some(record=>record.ref===view.selected.value._record?.ref&&record.kind===view.selected.value._record?.kind)):null;
  const callPeers=(callGroup?.records||[]).map(record=>view.rows.value.find(row=>row._record?.ref===record.ref&&row._record?.kind===record.kind)).filter(Boolean);
  const callIndex=callPeers.findIndex(record=>record.id===view.selected.value?.id);
  const selectPeer=offset=>{const record=callPeers[callIndex+offset];if(record)view.selectRecord(record);};
  const category=readValue(view.tab)==='运行诊断'?'diagnostics':categories.find(c=>c.sections.includes(view.currentSection.value))?.id||'models';
  useEffect(()=>{const align=()=>{const nav=managementNav.current,active=nav?.querySelector('[aria-current=page]');if(!nav||!active||nav.scrollWidth<=nav.clientWidth)return;const box=nav.getBoundingClientRect(),item=active.getBoundingClientRect();if(item.left<box.left)nav.scrollLeft+=item.left-box.left;else if(item.right>box.right)nav.scrollLeft+=item.right-box.right;};align();window.addEventListener('resize',align);return()=>window.removeEventListener('resize',align);},[category]);
  const show=title=>categories.find(c=>c.id===category)?.sections.includes(title)&&(category!=='models'||view.currentSection.value===title);
  const choose=id=>{setDirectory(false);const item=categories.find(c=>c.id===id);if(id==='data'){view.openData();return;}if(id==='diagnostics'){view.openDiagnostics();return;}view.tab.value='配置中心';view.openSettings(item.sections[0]);if(item.sections.length)void view.scrollTo(item.sections[0],false).then(()=>{document.querySelector('.replica-management-main')?.scrollTo({top:0,left:0,behavior:'instant'});});};
  return (
    <Tabs className="replica-management" value={view.tab.value} onValueChange={value=>view.tab.value=value}>
      <WorkspacePageHeader eyebrow="PREFERENCES / MAKE IT YOURS" title={readValue(view.tab)==='运行诊断'?'诊断与任务':'管理中心'} description={readValue(view.tab)==='运行诊断'?undefined:'让每一次相处，按你喜欢的方式发生。'} actions={readValue(view.tab)==='运行诊断'?undefined:view.accountActions()}/>
      {mobile && directory && readValue(view.tab)!=='运行诊断' && <div className="mobile-settings-directory">{[['偏好与连接',categories.filter(item => !['diagnostics','data','advanced'].includes(item.id))],['管理与维护',categories.filter(item => ['diagnostics','data','advanced'].includes(item.id))]].map(([label,items]) => <section key={label}><h2>{label}</h2>{items.map(item => <Button key={item.id} variant="ghost" onClick={() => choose(item.id)}><span className="lp-icon-box"><item.icon/></span><span>{item.label}</span><ChevronRight/></Button>)}</section>)}</div>}
      {mobile && !directory && <Button className="mobile-settings-back" variant="ghost" onClick={() => {setDirectory(true);location.hash='#/settings';}}><ChevronLeft/>全部设置</Button>}
      <div className="replica-management-layout" hidden={mobile && directory && readValue(view.tab)!=='运行诊断'}>
        <nav ref={managementNav} className="replica-management-nav" aria-label="管理中心设置目录" data-testid="config-section-nav">{categories.map(item=><ActionButton key={item.id} variant={category===item.id?'secondary':'ghost'} aria-current={category===item.id?'page':undefined} type="button" onClick={()=>choose(item.id)} data-testid={'config-anchor-'+item.sections[0]}><item.icon size={16}/>{item.label}</ActionButton>)}</nav>
        <div className="replica-management-main">
      <TabsContent value={view.tab.value} className="min-w-0">
        {readValue(view.tab) === "配置中心" ? (
          <div key={0} className="replica-configuration-content">
            <form
              ref={elementRef(view.configForm)}
              className="settings-form management-form replica-settings-form"
              onInvalidCapture={event=>{const target=event.target,section=target.closest(".settings-group");if(section?.dataset.section)void view.scrollTo(section.dataset.section,false).then(()=>{target.closest("[data-setting-field]")?.scrollIntoView({block:"center",behavior:"instant"});target.focus({preventScroll:true});});}}
              onSubmit={eventModifiers(view.save, ["prevent"])}
            >
              <div className="replica-management-section-header">
              <div className="replica-settings-head"><div><h2>{categories.find(c=>c.id===category)?.label}</h2><p>{category==='models'?'先连接服务，再为每一种能力选择合适的模型。':'修改正式配置；保存后读取实际回执。'}</p></div>{category==='models'&&<small className="replica-model-summary">{view.catalogReady.value?view.endpoints.value.length+' 个服务 · '+view.bindings.value.filter(binding=>!binding.role.startsWith('agent:')).length+' 个默认用途':'正在读取服务与用途'}</small>}{categories.filter(item=>item.id===category&&category!=='models').map(item=><item.icon key={item.id} size={25} className="replica-settings-section-icon"/>)}</div>
              {category==='models'&&<Tabs value={view.currentSection.value} onValueChange={value=>void view.scrollTo(value,false).then(()=>document.querySelector('.replica-management-main')?.scrollTo({top:0,behavior:'instant'}))}><TabsList variant="line"><TabsTrigger value="服务地址">模型服务 <Badge variant="secondary">{view.endpoints.value.length}</Badge></TabsTrigger><TabsTrigger value="模型用途">用途分配 <Badge variant="secondary">{view.bindings.value.filter(binding=>!binding.role.startsWith('agent:')).length}</Badge></TabsTrigger></TabsList></Tabs>}
              </div>
              {view.catalogLoading.value&&<p role="status" data-feedback-kind="progress">正在读取正式配置…</p>}
              {!view.catalogReady.value&&view.error.value&&<InlineFeedback role="alert">{view.error.value}<ActionButton type="button" variant="outline" onClick={view.retryCatalog}>重新读取配置</ActionButton></InlineFeedback>}
              <Surface
                as="section"
                hidden={!show('服务地址')||!view.catalogReady.value} data-section="服务地址"
                id={view.sectionId("服务地址")}
                className="settings-group"
              >
                <div className="replica-settings-section-head">
                  <div><h2>模型服务</h2><p>兼容 OpenAI 协议的服务地址与模型目录</p></div>
                  <ActionButton variant="outline"
                    type="button"

                    onClick={view.addEndpoint}
                  >
                    {"添加服务"}
                  </ActionButton>
                </div>
                {!view.endpoints.value.length&&<p className="muted">尚未配置模型服务。添加连接并保存后，可以读取真实模型目录。</p>}
                <Fragment>
                  {listValues(view.endpoints.value).map((h, b) => (
                    <div key={h.id} className="management-block">
                      <div className="replica-service-header">
                        <span className="replica-service-icon"><Server size={19}/></span>
                        <div><b>{displayText(h.name)}</b><small>{displayText(h.status)}</small></div>
                        {!h._source&&<ActionButton variant="destructive"
                          type="button"

                          onClick={() => view.removeEndpoint(b)}
                        >
                          {"移除"}
                        </ActionButton>}
                      </div>
                      <div className="form-grid">
                        <label>
                          {"服务名称"}
                          <FieldInput
                            readOnly={!!h._source}
                            required=""
                            maxLength="128"
                            value={h.name}
                            onChange={(event) => {
                              ((k) => (h.name = k))(event.target.value);
                            }}
                          />
                        </label>
                        <label>
                          {"接口协议"}
                          <ChoiceSelect
                            required=""
                            value={h.protocol}
                            onChange={(event) => {
                              ((k) => (h.protocol = k))(event.target.value);
                            }}
                          >
                            <option value="openai-compatible">
                              {"OpenAI 兼容接口"}
                            </option>
                            {h.protocol &&
                            h.protocol !== "openai-compatible" ? (
                              <option value={h.protocol}>{h.protocol}</option>
                            ) : null}
                          </ChoiceSelect>
                        </label>
                        <label>
                          {"服务地址"}
                          <FieldInput
                            required=""
                            type="url"
                            value={h.url}
                            onChange={(event) => {
                              ((k) => (h.url = k))(event.target.value);
                            }}
                          />
                        </label>
                        <label>
                          {"访问密钥"}
                          <FieldInput
                            type="password"
                            placeholder={
                              h._source?.secret_configured
                                ? "留空保留已保存的密钥"
                                : "可选：填写访问密钥"
                            }
                            autoComplete="new-password"
                            value={h.secret}
                            onChange={(event) => {
                              ((k) => (h.secret = k))(event.target.value);
                            }}
                          />
                        </label>
                        {h._source?.secret_configured ? (
                          <Field orientation="horizontal" className="full-field min-h-11 rounded-lg border border-border p-3"><FieldInput id={'clear-secret-'+h.id} type="checkbox" modelValue={h.clearSecret} onModelChange={value=>h.clearSecret=value}/><FieldLabel htmlFor={'clear-secret-'+h.id}>清除已存密钥</FieldLabel></Field>
                        ) : null}
                      </div>
                      <view.ModelDirectory models={h.models} status={h.modelsStatus} error={h.modelsError} readAt={h.modelsReadAt} onRetry={()=>view.test(h)}/>


                    </div>
                  ))}
                </Fragment>
                <p className="muted">
                  {
                    "服务地址修改并保存后，其用途绑定需要重新确认，已保存绑定与草稿分别显示。"
                  }
                </p>
              </Surface>
              <Surface
                as="section"
                hidden={!show("模型用途")||!view.catalogReady.value} data-section="模型用途"
                id={view.sectionId("模型用途")}
                className="settings-group"
              >
                <h2>{"为每种能力选择模型"}</h2>
                <p className="muted" id="model-role-timeout-hint">
                  {
                    "多个用途可以共用同一个服务，也可以分别配置。未使用的用途可以留空；请求超时留空时使用 120 秒。私聊、主动私聊和动态仍受回复设置中的总时限约束。"
                  }
                </p>
                <Disclosure className="management-block" title="按 Agent 或任务变体覆盖">
                  <p className="muted">未设置覆盖时使用展示的继承来源。保存后只影响新任务，视觉输入继续使用独立配置。</p>
                  <label>选择任务
                    <Select value={view.selectedAgentRoute.value || 'unselected'} onValueChange={value=>view.selectedAgentRoute.value=value==='unselected'?'':value}>
                      <SelectTrigger aria-label="选择 Agent 模型任务" className="w-full min-w-0"><SelectValue placeholder="请选择任务"/></SelectTrigger>
                      <SelectContent position="popper" align="start" className="max-w-[calc(100vw-2rem)]">
                        <SelectItem value="unselected">请选择任务</SelectItem>
                        {modelRouteGroups(view.agentModelRoutes.value).map(group=><SelectGroup key={group.label}>
                          <SelectLabel>{group.label}</SelectLabel>
                          {group.options.map(option=><SelectItem key={option.key} value={option.key}>{option.label}</SelectItem>)}
                        </SelectGroup>)}
                      </SelectContent>
                    </Select>
                  </label>
                  {view.agentModelRoutes.value.some(route=>!route.writable)&&<Disclosure title="视觉任务的现有绑定">{view.agentModelRoutes.value.filter(route=>!route.writable).map(route=><p className="muted" key={route.agent_id}>{modelRouteLabel(route)} · 按阶段或视觉配置确定 · {route.state==='unavailable'?'当前不可用':'能力待验证'}</p>)}</Disclosure>}
                </Disclosure>
                <Fragment>
                  {listValues(view.bindings.value).filter(h=>!h.role.startsWith('agent:')||h.role===view.selectedAgentRoute.value).sort((a,b)=>Number(b.role.startsWith('agent:'))-Number(a.role.startsWith('agent:'))).map((h) => {
                    var b;
                    const required =
                      !!h._source ||
                      !!h.endpoint ||
                      !!h.model ||
                      (h.timeout != null && h.timeout !== "") ||
                      (h.temperature != null && h.temperature !== "") ||
                      (h.topP != null && h.topP !== "");
                    return (
                      <section key={h.role} className="management-block replica-model-role" aria-label={h.name}>
                        <div className="replica-role-heading"><h3>{h.name}</h3><p>{({generic_llm:'规划日程、理解描述、整理记忆',conversation_reply:'组织语言，回应每次对话',vision_understanding:'理解聊天中分享的照片',embedding:'检索已经保存的记忆'})[h.role]||'为这项任务选择已连接的模型服务。'}</p>{h._route&&<p data-testid="agent-model-source">生效来源：{({variant:'任务变体覆盖',agent:'Agent 覆盖',default:'默认用途',legacy:'旧用途绑定',unconfigured:'尚未配置'})[h._route.source]||'来源待确认'}；模型：{h._route.model_id||'未配置'}；{h._route.state==='unavailable'?'当前绑定不可用':h._route.state==='configured'?'能力兼容性尚未验证':'等待配置'}。</p>}{h._source&&h.role.startsWith('agent:')&&<ActionButton type="button" variant="outline" disabled={view.saving.value} onClick={()=>h.clearOverride=!h.clearOverride}>{h.clearOverride?'撤销恢复继承':'恢复继承'}</ActionButton>}{h.clearOverride&&<p role="status">已加入恢复继承的草稿，保存后生效。</p>}</div>
                        <div className="form-grid">
                          <label>
                            {"模型服务"}
                            <ChoiceSelect
                              required={required}
                              disabled={!!h.clearOverride}
                              aria-label="模型服务"
                              value={h.endpoint}
                              onChange={(event) => {
                                ((value) => (h.endpoint = value))(
                                  event.target.value,
                                );
                              }}
                            >
                              <option value="">{"尚未配置"}</option>
                              {view.endpoints.value.map(
                                (endpoint, rowIndex) => (
                                  <option
                                    value={endpoint.id}
                                    key={
                                      typeof endpoint === "object"
                                        ? (endpoint?.id ?? rowIndex)
                                        : endpoint
                                    }
                                  >
                                    {endpoint.name}
                                  </option>
                                ),
                              )}
                            </ChoiceSelect>
                          </label>
                          <label>
                            {"模型标识"}
                            <Input aria-label="模型标识" required={required} disabled={!!h.clearOverride} list={'role-models-'+h.role} value={h.model} placeholder="填写模型标识" onChange={event=>h.model=event.target.value}/>
                            <datalist id={'role-models-'+h.role}>{(view.endpoints.value.find(endpoint=>endpoint.id===h.endpoint)?.models||[]).map(model=>typeof model==='string'?model:model.id||model.name).filter(Boolean).map(model=><option key={model} value={model}/>)}</datalist>
                          </label>
                          <label>
                            {"请求超时（秒，可选）"}
                            <Input
                              type="number"
                              disabled={!!h.clearOverride}
                              min="1"
                              placeholder="默认 120 秒"
                              aria-label="请求超时（秒，可选）"
                              aria-describedby="model-role-timeout-hint"
                              value={h.timeout}
                              onChange={(event) => {
                                ((value) => (h.timeout = value))(
                                  numberInput(event.target.value),
                                );
                              }}
                            />
                          </label>
                          {h.role !== 'embedding' && <label>温度（可选）<Input type="number" min="0" max="2" step="any" disabled={!!h.clearOverride} aria-label="温度（可选）" value={h.temperature ?? ''} placeholder="沿用任务默认" onChange={event=>h.temperature=numberInput(event.target.value)}/></label>}
                          {h.role !== 'embedding' && <label>Top P（可选）<Input type="number" min="0" max="1" step="any" disabled={!!h.clearOverride} aria-label="Top P（可选）" value={h.topP ?? ''} placeholder="沿用任务默认" onChange={event=>h.topP=numberInput(event.target.value)}/></label>}
                        </div>
                      </section>
                    );
                  })}
                </Fragment>
                <Disclosure
                  className="management-block"
                  title={
                    <>
                      {"共用通用模型的场景 · " +
                        displayText(readValue(view.sharedScenarios).length) +
                        " 项"}
                    </>
                  }
                >
                  <Fragment>
                    {listValues(readValue(view.sharedScenarios)).map(
                      (h, rowIndex) => (
                        <div
                          className="shared-scenario border-b border-border pb-3"
                          key={typeof h === "object" ? (h?.id ?? rowIndex) : h}
                        >
                          <span>{displayText(h)}</span>

                        </div>
                      ),
                    )}
                  </Fragment>
                </Disclosure>
              </Surface>
              {[...view.settings.value].sort((a,b)=>category==='runtime'?(['通知','作息与后台节奏','资源与预算'].indexOf(a.title)+1||9)-(['通知','作息与后台节奏','资源与预算'].indexOf(b.title)+1||9):0).map(group=><SettingsSections key={group.title} group={group} view={view} visible={show(group.title)}/>)}
              <Surface
                as="section"
                hidden={!show("安静时段")||!view.catalogReady.value} data-section="安静时段"
                id={view.sectionId("安静时段")}
                className="settings-group replica-quiet-card"
              >
                <h2>{"安静时段"}</h2>
                <div className="form-grid">
                  <Field orientation="horizontal" className="replica-quiet-toggle"><FieldLabel htmlFor="quiet-enabled">启用安静时段</FieldLabel><Switch id="quiet-enabled" checked={view.quiet.value.enabled} onCheckedChange={value=>view.quiet.value.enabled=value}/></Field>
                  <label>
                    {"适用角色"}
                    <ChoiceSelect
                      value={view.quiet.value.character}
                      onChange={(event) => {
                        ((h) => (view.quiet.value.character = h))(
                          event.target.value,
                        );
                      }}
                    >
                      <option>{"全部角色"}</option>
                      <Fragment>
                        {listValues(readValue(view.characters)).map(
                          (h, rowIndex) => (
                            <option
                              key={
                                typeof h === "object" ? (h?.id ?? rowIndex) : h
                              }
                            >
                              {displayText(h.name)}
                            </option>
                          ),
                        )}
                      </Fragment>
                    </ChoiceSelect>
                  </label>
                </div>
                {!view.quiet.value.enabled&&<p className="text-sm text-muted-foreground">开启后可设置每天暂停主动联系的时段。现有时间仍保留。</p>}
                <fieldset disabled={!view.quiet.value.enabled}>
                  <div className="form-grid">
                    <label>
                      {"开始时间"}
                      <FieldInput
                        type="time"
                        value={view.quiet.value.start}
                        onChange={(event) => {
                          ((h) => (view.quiet.value.start = h))(
                            event.target.value,
                          );
                        }}
                      />
                    </label>
                    <label>
                      {"结束时间"}
                      <FieldInput
                        type="time"
                        value={view.quiet.value.end}
                        onChange={(event) => {
                          ((h) => (view.quiet.value.end = h))(
                            event.target.value,
                          );
                        }}
                      />
                    </label>
                    <label>
                      {"行为"}
                      <ChoiceSelect
                        value={view.quiet.value.behavior}
                        onChange={(event) => {
                          ((h) => (view.quiet.value.behavior = h))(
                            event.target.value,
                          );
                        }}
                      >
                        {[
                          <option key={0}>{"暂停主动联系"}</option>,
                          <option
                            disabled={true}
                            title="\u5F53\u524D\u7B56\u7565\u53EA\u6682\u505C\u65B0\u4E3B\u52A8\u884C\u52A8\uFF1B\u5DF2\u53D7\u7406\u4EFB\u52A1\u4ECD\u6309\u539F\u671F\u9650\u5904\u7406"
                            key={1}
                          >
                            {"延后至安静时段结束"}
                          </option>,
                        ]}
                      </ChoiceSelect>
                    </label>
                  </div>
                  <p>{"生效日期（Asia/Shanghai，支持跨夜）"}</p>
                  <ToggleGroup type="multiple" variant="outline" disabled={!view.quiet.value.enabled} aria-label="安静时段生效星期" value={view.quiet.value.days} onValueChange={values=>view.quiet.value.days=values} className="max-w-full flex-wrap justify-start">{readValue(Yi).map(day=><ToggleGroupItem key={day} value={day}>{day}</ToggleGroupItem>)}</ToggleGroup>
                </fieldset>
              </Surface>
              {view.testResult.value ? (
                <OperationStatus kind={view.testKind.value} key={0} className="muted">
                  {displayText(view.testResult.value)}
                </OperationStatus>
              ) : null}
              {view.testRaw.value&&<JsonDetails value={view.testRaw.value} title="测试回执与实际状态"/>}
              {view.catalogReady.value&&<div className="settings-save-bar" data-testid="config-save-bar">
                {view.hasRecovery.value ? <Feedback kind={view.recoveringConfiguration.value?'progress':view.configurationStorageProblem.value?'error':view.recovery.value?.status===409?'warning':[400,403,422].includes(view.recovery.value?.status)?'error':'action-required'} title={view.recoveringConfiguration.value?'正在核对设置':view.configurationStorageProblem.value?'本机提交记录暂时无法读取':view.needsBaselineReview.value&&!view.recovery.value?'请核对最新配置':view.recovery.value?.status===409?'设置版本已变化':[400,403,422].includes(view.recovery.value?.status)?'设置未能保存':'原设置提交结果待核对'} details={(view.configurationStorageProblem.value||view.recovery.value?.failure)&&<pre>{JSON.stringify(view.configurationStorageProblem.value||view.recovery.value.failure,null,2)}</pre>}>
                  <p>{view.error.value||view.saveStatus.value||'原设置提交仍需核对，当前输入会保留。'}</p><view.RecoveryActions/>
                </Feedback> : view.error.value ? (
                  <InlineFeedback key={0} role="alert" >
                    {displayText(view.error.value)}
                  </InlineFeedback>
                ) : (
                  <p className="text-sm" role="status" aria-live="polite" data-feedback-kind={view.saving.value?'progress':view.saveKind.value} data-testid="config-save-status">{view.saveStatus.value||(view.dirty.value?'有未保存修改':view.settings.value.some(group=>Object.values(group.valueSources||{}).includes('runtime_default'))?'已读取当前配置 · 含系统默认，尚未保存':'当前配置已读取')}{view.saveStatus.value&&view.dirty.value&&!view.saving.value&&' · 有未保存修改'}</p>
                )}
                <ActionButton variant="outline" className="settings-discard" type="button" disabled={!view.dirty.value||!view.canDiscard.value} onClick={view.discard}>放弃修改</ActionButton>
                <ActionButton variant="default"

                  disabled={!view.dirty.value||view.saving.value||view.recoveringConfiguration.value||!view.catalogReady.value||view.hasRecovery.value}
                  className="settings-save"
                  data-testid="config-save"
                >
                  {displayText(view.saving.value ? "保存中…" : "保存设置")}
                </ActionButton>
              </div>}
            </form>
          </div>
        ) : (
          <DiagnosticsWorkspace view={view}>
            <section className="replica-model-calls-surface">
              <header className="replica-model-calls-heading"><h2>{view.kind.value==='媒体调用'?'最近的媒体调用':'最近的模型调用'}</h2><p>逐次查看请求、返回与错误。</p></header>
              {view.diagnosticPages.value[view.kind.value].ready&&<DiagnosticRecords view={{...view,selectRecord:(record,trigger)=>{recordTrigger.current=trigger;view.selectRecord(record);}}}/>}
              <CursorControls label="调用" append={{hasContent:view.rows.value.length>0,loading:view.diagnosticPages.value[view.kind.value].loading,hasMore:!!view.diagnosticPages.value[view.kind.value].cursor,loadMore:view.loadMoreDiagnostics,error:view.diagnosticPages.value[view.kind.value].error,failure:view.diagnosticPages.value[view.kind.value].failure,retry:view.loadMoreDiagnostics}}/>
            </section>
            <Collapsible className="replica-model-calls-secondary">
              <CollapsibleTrigger asChild><ActionButton type="button" variant="ghost">筛选与更多运行记录</ActionButton></CollapsibleTrigger>
              <CollapsibleContent className="replica-model-calls-options">
                <form className="grid gap-2" onSubmit={event=>{event.preventDefault();void view.searchHistory();}}><FieldLabel htmlFor="diagnostic-history-search">查找历史调用</FieldLabel><div className="flex gap-2"><Input id="diagnostic-history-search" data-testid="diag-history-search" value={view.historySearch.value} onChange={event=>view.historySearch.value=event.target.value} placeholder="用途、模型或关联 ID"/><Button type="submit">查找历史</Button>{view.diagnosticQuery.value&&<Button type="button" variant="outline" onClick={()=>{view.historySearch.value='';void view.searchHistory();}}>清除历史查询</Button>}</div><small className="muted">查询全部保留记录；下方筛选只作用于已读取结果。{view.diagnosticQuery.value&&' 当前查询：'+view.diagnosticQuery.value}</small></form>
                <label className="replica-diagnostic-kind-filter">调用类型<ChoiceSelect aria-label="调用类型" value={view.kind.value} onChange={event=>view.selectKind(event.target.value)}>{['模型调用','媒体调用'].map(kind=><option key={kind} value={kind}>{kind}</option>)}</ChoiceSelect></label>
                <view.FilterChips search={view.search.value} onUpdateSearch={h=>view.search.value=h} filters={view.filters.value} onUpdateFilters={h=>view.filters.value=h} groups={view.filterGroups.value} testId="diag-filters" placeholder="搜索调用 / 提示词 / 工作流 / 角色"/>
                <small className="muted">{view.rows.value.length} 条已读取记录 · 按记录顺序排列</small>
            <div className="diagnostic-navigation grid gap-4">
              <DropdownMenu><DropdownMenuTrigger asChild><ActionButton type="button" variant="ghost" aria-label="更多运行记录"><MoreHorizontal size={16}/>{view.reference.value||'更多运行记录'}</ActionButton></DropdownMenuTrigger><DropdownMenuContent align="start"><DropdownMenuItem onSelect={()=>view.selectReference('')}>模型与媒体调用</DropdownMenuItem>{readValue(view.referenceSections).map(section=><DropdownMenuItem key={section.title} onSelect={()=>view.selectReference(section.title)}>{section.title}</DropdownMenuItem>)}<DropdownMenuSeparator/><DropdownMenuItem onSelect={view.openTaskDiagnostics}>待处理事项与原任务恢复</DropdownMenuItem></DropdownMenuContent></DropdownMenu>

              {view.referenceSection.value ? <Surface as="section" key={view.referenceSection.value.kind+view.referenceSection.value.title} className="grid gap-4"><div className="row"><h3>{view.referenceSection.value.title}</h3><span className="spacer"/><ActionButton variant="outline" type="button" onClick={()=>view.reference.value=''}>收起面板</ActionButton></div>{view.referenceSection.value.loading&&<p role="status">正在读取记录…</p>}{view.referenceSection.value.error&&<InlineFeedback role="alert">{view.referenceSection.value.error}<ActionButton variant="outline" onClick={view.retryReference}>重新读取原记录页</ActionButton></InlineFeedback>}{!view.referenceSection.value.rows.length&&!view.referenceSection.value.loading&&!view.referenceSection.value.error&&<p className="text-sm text-muted-foreground">当前没有对应记录。</p>}{view.referenceSection.value.rows.map(record=><ActionButton key={view.referenceSection.value.kind+':'+record.ref} variant="outline" className="h-auto min-h-14 w-full justify-between whitespace-normal text-left" onClick={()=>view.openReference(record,view.referenceSection.value)}><span className="grid min-w-0 gap-1"><b>{view.referenceTitle(record)}</b><small>{record.occurred_at?view.userInstant(record.occurred_at):'时间未记录'}</small></span><Badge>{view.referenceStatus(record)}</Badge><span>查看详情</span></ActionButton>)}<CursorControls label="诊断记录" append={{loading:view.referenceSection.value.loading,hasMore:!!view.referenceSection.value.nextCursor,loadMore:view.loadMoreReference}}/></Surface>:null}
              {view.referenceRecord.value&&<view.DetailDrawer title={view.referenceRecord.value.section.title+' · 记录详情'} testId="reference-drawer" onClose={()=>view.referenceRecord.value=null}>{view.referenceRecord.value.loading?<p role="status">正在读取详情…</p>:view.referenceRecord.value.error?<InlineFeedback role="alert">{view.referenceRecord.value.error}<ActionButton variant="outline" onClick={()=>view.openReference(view.referenceRecord.value.record,view.referenceRecord.value.section)}>重新读取</ActionButton></InlineFeedback>:<RecordView value={view.referenceRecord.value.detail?.data||view.referenceRecord.value.detail} technical/>}</view.DetailDrawer>}
            </div>
              </CollapsibleContent>
            </Collapsible>
            {view.selected.value ? (
              <WorkspaceDialog
                open
                key={0}
                title={view.selected.value.label}
                className="replica-model-call-dialog"
                description={view.selected.value.kind+" · "+view.recordTime(view.selected.value)}
                headerAction={<div ref={setCallTechnicalTarget}/>}
                footer={<div className="flex w-full min-w-0 items-center justify-end gap-2">{callPeers.length>1&&<nav aria-label="同组调用" className="grid min-w-0 flex-1 grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2"><Button variant="outline" disabled={callIndex===0} onClick={()=>selectPeer(-1)}>上一条调用</Button><span role="status" className="text-xs text-center text-muted-foreground" aria-label={'同组已读取 '+callPeers.length+' 次，当前查看第 '+(callIndex+1)+' 条'}>{callIndex+1} / {callPeers.length}</span><Button variant="outline" disabled={callIndex===callPeers.length-1} onClick={()=>selectPeer(1)}>下一条调用</Button></nav>}<Button className={callPeers.length>1?'':'flex-1 sm:flex-none'} variant="outline" type="button" onClick={()=>view.selectedId.value=null}>关闭</Button></div>}
                testId="diagnostic-drawer"
                returnFocus={()=>recordTrigger.current}
                onOpenChange={open=>{if(!open)view.selectedId.value=null;}}
              >

                <ModelCallDetails key={view.selected.value.kind+':'+view.selected.value.id} technicalTarget={callTechnicalTarget} record={view.selected.value} onRecover={view.selected.value.status==='error'?()=>view.retry(view.selected.value):undefined} onOpenMedia={view.openMediaAsset}/>
              </WorkspaceDialog>
            ) : null}

          </DiagnosticsWorkspace>
        )}
      </TabsContent>
        </div>
      </div>
    </Tabs>
  );
}
