import RecordView from '@/react-app/shared/RecordView';
import CursorControls from '@/react-app/shared/CursorControls';
import {Field,FieldLabel} from '@/components/ui/field';
import Combobox from '@/react-app/shared/Combobox';
import {InlineFeedback as OperationStatus} from "@/react-app/shared/Presentation";
import {InlineFeedback} from "@/react-app/shared/Presentation";
import ModelTranscript from '@/react-app/shared/ModelTranscript';
import JsonDetails from '@/react-app/shared/JsonDetails';
import {Table,TableBody,TableCell,TableRow} from '@/components/ui/table';
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
  return (
    <Tabs
      value={view.tab.value}
      onValueChange={(value) => (view.tab.value = value)}
    >
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-3"><TabsList className="h-auto group-data-horizontal/tabs:h-auto max-w-full flex-wrap">
        <Fragment>
          {listValues(["配置中心", "运行诊断"]).map((h) => (
            <TabsTrigger
              key={h}
              className={cn({
                active: readValue(view.tab) === h,
              })}
              value={h}
            >
              {displayText(h)}
            </TabsTrigger>
          ))}
        </Fragment>
      </TabsList></div>
      <TabsContent value={view.tab.value} className="min-w-0">
        {readValue(view.tab) === "配置中心" ? (
          <div key={0} className="configuration-layout">
            <nav
              className="configuration-nav hidden min-[801px]:flex"
              aria-label="配置章节"
              data-testid="config-section-nav"
            >
              <Fragment>
                {listValues(view.sections.value).map((h) => (
                  <ActionButton variant={view.currentSection.value===h?"secondary":"ghost"} aria-current={view.currentSection.value===h?"location":undefined} type="button" key={h} className="justify-start text-left" data-testid={"config-anchor-"+h} onClick={()=>view.scrollTo(h)}>{view.sectionLabel(h)}</ActionButton>
                ))}
              </Fragment>
            </nav>
            <div className="configuration-mobile-nav min-[801px]:hidden"><label className="grid gap-2 text-sm">配置章节<ChoiceSelect aria-label="选择配置章节" value={view.currentSection.value} onChange={event=>view.scrollTo(event.target.value)}>{view.sections.value.map(section=><option key={section} value={section}>{view.sectionLabel(section)}</option>)}</ChoiceSelect></label></div>
            <form
              ref={elementRef(view.configForm)}
              className="settings-form management-form"
              onSubmit={eventModifiers(view.save, ["prevent"])}
            >
              <Surface
                as="section"
                id={view.sectionId("服务地址")}
                className="settings-group"
              >
                <div className="row">
                  <h2>{"服务地址"}</h2>
                  <span className="spacer" />
                  <ActionButton variant="ghost"
                    type="button"

                    onClick={view.addEndpoint}
                  >
                    {"添加服务"}
                  </ActionButton>
                </div>
                <Fragment>
                  {listValues(view.endpoints.value).map((h, b) => (
                    <div key={h.id} className="management-block">
                      <div className="row">
                        <b>{displayText(h.name)}</b>
                        <Badge variant="secondary">{displayText(h.status)}</Badge>
                        <span className="spacer" />
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
                      <view.ModelDirectory models={h.models} status={h.modelsStatus} error={h.modelsError} onRetry={()=>view.test(h)}/>
                      <ActionButton variant="outline"
                        type="button"

                        onClick={(k) => view.test(h)}
                      >
                        {"同步模型目录（已保存服务）"}
                      </ActionButton>
                      {h.feedback ? (
                        <OperationStatus variant="default" className="muted" role="status">
                          {h.feedback}
                        </OperationStatus>
                      ) : null}
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
                id={view.sectionId("模型用途")}
                className="settings-group"
              >
                <h2>{"各用途对应的模型配置"}</h2>
                <p className="muted">
                  {
                    "可以先配置思考与通用分析、聊天回复；看图与记忆检索向量可在使用时再配置。应用不设置输入、输出 Token 上限。"
                  }
                </p>
                <Fragment>
                  {listValues(view.bindings.value).map((h) => {
                    var b;
                    const required =
                      !!h._source ||
                      !!h.endpoint ||
                      !!h.model ||
                      (h.timeout != null && h.timeout !== "");
                    return (
                      <Disclosure
                        key={h.name}
                        className="management-block"
                        open={
                          !!h._source ||
                          ["generic_llm", "conversation_reply"].includes(h.role)
                        }
                        title={<>{h.name}</>}
                      >
                        <div className="form-grid">
                          <label>
                            {"使用服务"}
                            <ChoiceSelect
                              required={required}
                              aria-label="使用服务"
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
                            {"模型 ID"}
                            <Combobox label="模型 ID" required={required} value={h.model} allowCustom options={[{value:'',label:'尚未配置'},...(view.endpoints.value.find(endpoint=>endpoint.id===h.endpoint)?.models||[]).map(model=>typeof model==='string'?model:model.id||model.name).filter(Boolean)]} onChange={value=>h.model=value}/>
                          </label>
                          <label>
                            {"超时（秒，可选）"}
                            <FieldInput
                              type="number"
                              min="1"
                              placeholder="默认 120 秒"
                              aria-label="超时（秒，可选）"
                              value={h.timeout}
                              onChange={(event) => {
                                ((value) => (h.timeout = value))(
                                  numberInput(event.target.value),
                                );
                              }}
                            />
                            <small className="muted">
                              {"留空使用 120 秒；未配置用途可以全部留空。"}
                            </small>
                          </label>
                        </div>
                      </Disclosure>
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
              <Surface
                as="section"
                id={view.sectionId("安静时段")}
                className="settings-group"
              >
                <h2>{"安静时段"}</h2>
                <div className="form-grid">
                  <Field orientation="horizontal" className="min-h-11 self-end rounded-lg border border-border p-3"><FieldInput id="quiet-enabled" className="switch" type="checkbox" modelValue={view.quiet.value.enabled} onModelChange={value=>view.quiet.value.enabled=value}/><FieldLabel htmlFor="quiet-enabled">启用安静时段</FieldLabel></Field>
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
              <Fragment>
                {listValues(view.settings.value).map((h) => (
                  <Surface as="section"
                    key={h.title}
                    id={view.sectionId(h.title)}
                    className="settings-group"
                  >
                    <Disclosure title={h.title === "高级" ? "高级 · 辅助决策服务" : view.sectionLabel(h.title)}>
                    {h.title === "图片（ComfyUI）" ? (
                      <p key={2} className="muted">
                        {"图片使用 ComfyUI；需要图片功能时再展开配置。"}
                      </p>
                    ) : null}
                    <div className="form-grid">
                      <Fragment>
                        {listValues(h.fields).map((b) => (
                          <view.SettingsField key={b.label} field={b} />
                        ))}
                      </Fragment>
                    </div>
                    {["图片（ComfyUI）", "高级", "看图理解"].includes(
                      h.title,
                    ) ? (
                      <ActionButton variant="outline" className="mt-4"
                        key={3}
                        type="button"

                        onClick={(b) => view.test(h.title)}
                      >
                        {displayText(
                          h.title === "看图理解"
                            ? "测试看图（使用已保存配置）"
                            : "测试连接",
                        )}
                      </ActionButton>
                    ) : null}
                    </Disclosure>
                  </Surface>
                ))}
              </Fragment>
              {view.testResult.value ? (
                <OperationStatus variant="default" key={0} role="status" className="muted">
                  {displayText(view.testResult.value)}
                </OperationStatus>
              ) : null}
              {view.testRaw.value&&<JsonDetails value={view.testRaw.value} title="测试回执与实际状态"/>}
              <div className="settings-save-bar" data-testid="config-save-bar">
                {view.error.value ? (
                  <InlineFeedback key={0} role="alert" >
                    {displayText(view.error.value)}
                  </InlineFeedback>
                ) : (
                  <p className="text-sm" role="status" aria-live="polite" data-testid="config-save-status">{view.saveStatus.value||(view.dirty.value?'有未保存修改':'设置已同步')}{view.saveStatus.value&&view.dirty.value&&!view.saving.value&&' · 有未保存修改'}</p>
                )}
                <ActionButton variant="outline" className="h-auto min-h-11 whitespace-normal" type="button" disabled={!view.dirty.value||!view.canDiscard.value} onClick={view.discard}>放弃未保存修改</ActionButton>
                <ActionButton variant="default"

                  disabled={view.saving.value}
                  className="h-auto min-h-11 whitespace-normal"
                  data-testid="config-save"
                >
                  {displayText(view.saving.value ? "保存中…" : "保存设置")}
                </ActionButton>
              </div>
            </form>
          </div>
        ) : (
          <Fragment key={1}>
            <p className="muted">
              {"每次回应背后的记录。模型与媒体的运行情况，都在这里。"}
            </p>
            <Tabs value={view.kind.value} onValueChange={view.selectKind}><TabsList>{['模型调用','媒体调用'].map(kind=><TabsTrigger key={kind} value={kind} data-testid={'diag-tab-'+kind}>{kind}</TabsTrigger>)}</TabsList></Tabs>
            <div className="diagnostic-navigation grid gap-4">
              <ActionButton variant="outline" onClick={view.openTaskDiagnostics}>查看待处理事项与原任务恢复</ActionButton>
              <label className="grid max-w-md gap-2 text-sm">查看其它运行记录<ChoiceSelect aria-label="其它运行记录" value={view.reference.value} onChange={event=>view.selectReference(event.target.value)}><option value="">模型与媒体调用</option>{readValue(view.referenceSections).map(section=><option key={section.title} value={section.title}>{section.title}</option>)}</ChoiceSelect></label>

              {view.referenceSection.value ? <Surface as="section" key={view.referenceSection.value.kind+view.referenceSection.value.title} className="grid gap-4"><div className="row"><h3>{view.referenceSection.value.title}</h3><span className="spacer"/><ActionButton variant="outline" type="button" onClick={()=>view.reference.value=''}>收起面板</ActionButton></div>{view.referenceSection.value.loading&&<p role="status">正在读取记录…</p>}{view.referenceSection.value.error&&<InlineFeedback role="alert">{view.referenceSection.value.error}<ActionButton variant="outline" onClick={view.retryReference}>重新读取原记录页</ActionButton></InlineFeedback>}{!view.referenceSection.value.rows.length&&!view.referenceSection.value.loading&&!view.referenceSection.value.error&&<p className="text-sm text-muted-foreground">当前没有对应记录。</p>}{view.referenceSection.value.rows.map(record=><ActionButton key={view.referenceSection.value.kind+':'+record.ref} variant="outline" className="h-auto min-h-14 w-full justify-between whitespace-normal text-left" onClick={()=>view.openReference(record,view.referenceSection.value)}><span className="grid min-w-0 gap-1"><b>{view.referenceTitle(record)}</b><small>{record.occurred_at?view.userInstant(record.occurred_at):'时间未记录'}</small></span><Badge>{view.referenceStatus(record)}</Badge><span>查看详情</span></ActionButton>)}<CursorControls label="诊断记录" append={{loading:view.referenceSection.value.loading,hasMore:!!view.referenceSection.value.nextCursor,loadMore:view.loadMoreReference}}/></Surface>:null}
              {view.referenceRecord.value&&<view.DetailDrawer title={view.referenceRecord.value.section.title+' · 记录详情'} testId="reference-drawer" onClose={()=>view.referenceRecord.value=null}>{view.referenceRecord.value.loading?<p role="status">正在读取详情…</p>:view.referenceRecord.value.error?<InlineFeedback role="alert">{view.referenceRecord.value.error}<ActionButton variant="outline" onClick={()=>view.openReference(view.referenceRecord.value.record,view.referenceRecord.value.section)}>重新读取</ActionButton></InlineFeedback>:<RecordView value={view.referenceRecord.value.detail?.data||view.referenceRecord.value.detail} technical/>}</view.DetailDrawer>}
            </div>
            {!view.reference.value&&<Fragment>
            <view.FilterChips
              search={view.search.value}
              onUpdateSearch={(h) => (view.search.value = h)}
              filters={view.filters.value}
              onUpdateFilters={(h) => (view.filters.value = h)}
              groups={view.filterGroups.value}
              testId="diag-filters"
              placeholder="搜索调用 / 提示词 / 工作流 / 角色"
            />
            <small className="muted">
              {displayText(view.rows.value.length) + " 条已读取记录 · 按记录顺序排列"}
            </small>
            <p className="muted">{view.kind.value === "媒体调用" ? "图片、音频和视频任务的实际生成记录。" : "模型处理请求的状态、用时和结果摘要。"}</p>
            <DiagnosticRecords view={view}/><CursorControls label="调用" append={{loading:view.diagnosticPages.value[view.kind.value].loading,hasMore:!!view.diagnosticPages.value[view.kind.value].cursor,loadMore:view.loadMoreDiagnostics,error:view.diagnosticPages.value[view.kind.value].error,retry:view.loadMoreDiagnostics}}/>
            {view.selected.value ? (
              <view.DetailDrawer
                key={0}
                title={view.selected.value.label + " · 调用详情"}
                testId="diagnostic-drawer"
                onClose={(h) => (view.selectedId.value = null)}
              >
                <div className="detail-meta">
                  <Badge >
                    {displayText(view.selected.value.kind)}
                  </Badge>
                  <p>
                    {displayText(view.selected.value.character) +
                      " · " +
                      displayText(view.recordTime(view.selected.value)) +
                      " · " +
                      displayText(view.selected.value.durationLabel)}
                  </p>
                  <span
                    className={cn(["call-status", view.selected.value.status])}
                  >
                    {displayText(view.statusLabel(view.selected.value.status))}
                  </span>
                </div>
                {view.selected.value.status === "error" ? (
                  <InlineFeedback key={0} >
                    {displayText(view.selected.value.category) +
                      "：" +
                      displayText(view.selected.value.reason) +
                      " "}
                    <ActionButton variant="outline"

                      data-testid="diag-drawer-retry"
                      onClick={(h) => view.retry(view.selected.value)}
                    >
                      {"查看原记录与恢复方式"}
                    </ActionButton>
                  </InlineFeedback>
                ) : null}
                {view.selected.value.kind === "媒体调用" &&
                view.selected.value.media ? (
                  <Fragment key={1}>
                    <dl
                      className="media-call-attributes"
                      data-testid="media-call-details"
                    >
                      <div>
                        <dt>{"角色"}</dt>
                        <dd>{displayText(view.selected.value.character)}</dd>
                      </div>
                      <div>
                        <dt>{"媒体类型"}</dt>
                        <dd>
                          {displayText(view.selected.value.media.type) +
                            " · 图片"}
                        </dd>
                      </div>
                      <div>
                        <dt>{"工作流"}</dt>
                        <dd>
                          {displayText(view.selected.value.media.workflow)}
                        </dd>
                      </div>
                    </dl>
                    <section className="detail-section">
                      <h3>{"提示词 / Prompt"}</h3>
                      <RecordView value={view.selected.value.media.prompt}/>
                    </section>
                    <section className="detail-section">
                      <h3>{"负面提示词 / Negative prompt"}</h3>
                      <RecordView value={view.selected.value.media.negativePrompt}/>
                    </section>
                    <section className="detail-section">
                      <h3>{"生成参数"}</h3>
                      <Table><TableBody>{Object.entries(view.selected.value.media.params||{}).map(([key,value])=><TableRow key={key}><TableCell>{({width:'宽度',height:'高度',steps:'步数',cfg:'CFG',seed:'种子',sampler:'采样器'})[key]||key}</TableCell><TableCell>{value===null||value===undefined?'尚未记录':typeof value==='object'?JSON.stringify(value):String(value)}</TableCell></TableRow>)}</TableBody></Table>
                    </section>
                    <section
                      className="detail-section"
                      data-testid="media-call-output"
                    >
                      <h3>{"输出"}</h3>
                      {view.selected.value.media.assetId?<><p>{view.selected.value.media.output}</p><ActionButton variant="outline" type="button"  onClick={()=>view.openMediaAsset(view.selected.value.media.assetId)}>查看已记录素材</ActionButton></>:<p className="muted">{view.selected.value.status==='pending'?'媒体任务仍在处理。':'尚未关联可读取的媒体素材；调用完成不代表图片已生成。'}</p>}
                    </section>
                  </Fragment>
                ) : (
                  <Fragment key={2}>
                    <p className="muted">
                      {"模型 · " + displayText(view.selected.value.model)}
                      {view.selected.value.tokens ? (
                        <Fragment key={0}>
                          {" · " +
                            displayText(view.selected.value.tokens) +
                            " token"}
                        </Fragment>
                      ) : null}
                    </p>
                    <ModelTranscript key={view.selected.value.id} record={view.selected.value}/>
                  </Fragment>
                )}
                <JsonDetails value={view.selected.value._record} title="技术详情与关联标识"/>
              </view.DetailDrawer>
            ) : null}
            </Fragment>}

          </Fragment>
        )}
      </TabsContent>
    </Tabs>
  );
}
