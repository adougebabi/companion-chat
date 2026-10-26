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
      <TabsList className="max-w-full overflow-x-auto">
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
      </TabsList>
      <TabsContent value={view.tab.value} className="min-w-0">
        {readValue(view.tab) === "配置中心" ? (
          <div key={0} className="configuration-layout">
            <nav
              className="configuration-nav"
              aria-label="配置章节"
              data-testid="config-section-nav"
            >
              <Fragment>
                {listValues(view.sections.value).map((h) => (
                  <ActionButton type="button" key={h} className="text-button justify-start text-left" data-testid={"config-anchor-"+h} onClick={()=>view.scrollTo(h)}>{h}</ActionButton>
                ))}
              </Fragment>
            </nav>
            <form
              ref={elementRef(view.configForm)}
              className="settings-form management-form"
              onSubmit={eventModifiers(view.save, ["prevent"])}
            >
              <Surface
                as="section"
                id={view.sectionId("服务地址")}
                className="card settings-group"
              >
                <div className="row">
                  <h2>{"服务地址"}</h2>
                  <span className="spacer" />
                  <ActionButton
                    type="button"
                    className="text-button"
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
                        <small>{displayText(h.status)}</small>
                        <span className="spacer" />
                        <ActionButton
                          type="button"
                          className="text-button"
                          onClick={() => view.removeEndpoint(b)}
                        >
                          {"移除"}
                        </ActionButton>
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
                          <label className="check-field">
                            {"清除已存密钥"}
                            <FieldInput
                              type="checkbox"
                              modelValue={h.clearSecret}
                              onModelChange={(k) => (h.clearSecret = k)}
                            />
                          </label>
                        ) : null}
                      </div>
                      <view.ModelDirectory models={h.models} />
                      <ActionButton
                        type="button"
                        className="text-button"
                        onClick={(k) => view.test(h)}
                      >
                        {"读取模型目录（已保存连接）"}
                      </ActionButton>
                      {h.feedback ? (
                        <p className="muted" role="status">
                          {h.feedback}
                        </p>
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
                className="card settings-group"
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
                            <FieldInput
                              list={"models-" + h.role}
                              required={required}
                              aria-label="模型 ID"
                              value={h.model}
                              onChange={(event) => {
                                ((value) => (h.model = value))(
                                  event.target.value,
                                );
                              }}
                            />
                            <datalist id={"models-" + h.role}>
                              {(
                                view.endpoints.value.find(
                                  (endpoint) => endpoint.id === h.endpoint,
                                )?.models || []
                              ).map((model, rowIndex) => (
                                <option
                                  value={model}
                                  key={
                                    typeof model === "object"
                                      ? (model?.id ?? rowIndex)
                                      : model
                                  }
                                />
                              ))}
                            </datalist>
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
                          className="shared-scenario"
                          key={typeof h === "object" ? (h?.id ?? rowIndex) : h}
                        >
                          <span>{displayText(h)}</span>
                          <small>{"使用：通用模型"}</small>
                        </div>
                      ),
                    )}
                  </Fragment>
                </Disclosure>
              </Surface>
              <Surface
                as="section"
                id={view.sectionId("安静时段")}
                className="card settings-group"
              >
                <h2>{"安静时段"}</h2>
                <div className="form-grid">
                  <label className="check-field">
                    {"启用安静时段"}
                    <FieldInput
                      className="switch"
                      type="checkbox"
                      modelValue={view.quiet.value.enabled}
                      onModelChange={(h) => (view.quiet.value.enabled = h)}
                    />
                  </label>
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
                  <Disclosure
                    key={h.title}
                    id={view.sectionId(h.title)}
                    className="card settings-group"
                    title={
                      <>
                        {h.title === "高级" ? "高级 · Kev 决策服务" : h.title}
                      </>
                    }
                  >
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
                      <ActionButton
                        key={3}
                        type="button"
                        className="text-button"
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
                ))}
              </Fragment>
              {view.testResult.value ? (
                <p key={0} role="status" className="muted">
                  {displayText(view.testResult.value)}
                </p>
              ) : null}
              <div className="settings-save-bar" data-testid="config-save-bar">
                {view.error.value ? (
                  <p key={0} role="alert" className="inline-error">
                    {displayText(view.error.value)}
                  </p>
                ) : (
                  <p
                    className="muted"
                    role="status"
                    aria-live="polite"
                    data-testid="config-save-status"
                  >
                    {view.saveStatus.value || "让相处的节奏，更合心意。"}
                  </p>
                )}
                <ActionButton
                  className="button primary"
                  disabled={view.saving.value}
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
              {displayText(view.rows.value.length) + " 条记录 · 最近的记录在前"}
            </small>
            <p className="muted">{view.kind.value === "媒体调用" ? "图片、音频和视频任务的实际生成记录。" : "模型处理请求的状态、用时和结果摘要。"}</p>
            <DiagnosticRecords view={view}/>
            {view.selected.value ? (
              <view.DetailDrawer
                key={0}
                title={view.selected.value.label + " · 调用详情"}
                testId="diagnostic-drawer"
                onClose={(h) => (view.selectedId.value = null)}
              >
                <div className="detail-meta">
                  <Badge className="pill">
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
                  <p key={0} className="inline-error">
                    {displayText(view.selected.value.category) +
                      "：" +
                      displayText(view.selected.value.reason) +
                      " "}
                    <ActionButton
                      className="text-button"
                      data-testid="diag-drawer-retry"
                      onClick={(h) => view.retry(view.selected.value)}
                    >
                      {"重试"}
                    </ActionButton>
                  </p>
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
                      <ScrollArea className="h-64"><pre className="whitespace-pre-wrap break-words">{view.selected.value.media.prompt}</pre></ScrollArea>
                    </section>
                    <section className="detail-section">
                      <h3>{"负面提示词 / Negative prompt"}</h3>
                      <ScrollArea className="h-48"><pre className="whitespace-pre-wrap break-words">{view.selected.value.media.negativePrompt}</pre></ScrollArea>
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
                      {view.selected.value.media.assetId?<><p>{view.selected.value.media.output}</p><ActionButton type="button" className="button" onClick={()=>view.openMediaAsset(view.selected.value.media.assetId)}>查看已记录素材</ActionButton></>:<p className="muted">{view.selected.value.status==='pending'?'媒体任务仍在处理。':'尚未关联可读取的媒体素材；调用完成不代表图片已生成。'}</p>}
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
            <div className="secondary-diagnostics">
              <small>{"其它诊断"}</small>
              <ToggleGroup type="single" variant="outline" value={view.reference.value} onValueChange={value=>view.reference.value=value} className="max-w-full flex-wrap justify-start" aria-label="其他诊断">{readValue(view.referenceSections).map(section=><ToggleGroupItem key={section.title} value={section.title}>{section.title}</ToggleGroupItem>)}</ToggleGroup>
              {view.referenceSection.value ? (
                <Surface as="div" key={0} className="card">
                  <div className="row">
                    <b>
                      {displayText(view.referenceSection.value.title) +
                        " · 实际记录"}
                    </b>
                    <span className="spacer" />
                    <ActionButton onClick={(h) => (view.reference.value = "")}>
                      {"关闭"}
                    </ActionButton>
                  </div>
                  <Fragment>
                    {listValues(view.referenceSection.value.rows).map(
                      (h, rowIndex) => (
                        <p
                          key={typeof h === "object" ? (h?.id ?? rowIndex) : h}
                        >
                          {displayText(h)}
                        </p>
                      ),
                    )}
                  </Fragment>
                  {["后台任务", "待处理事项 / 重试耗尽"].includes(
                    view.reference.value,
                  ) ? (
                    <ActionButton
                      key={0}
                      className="text-button"
                      onClick={view.openTaskDiagnostics}
                    >
                      {"查看原任务与恢复"}
                    </ActionButton>
                  ) : null}
                  <p role="status">{displayText(view.testResult.value)}</p>
                </Surface>
              ) : null}
            </div>
          </Fragment>
        )}
      </TabsContent>
    </Tabs>
  );
}
