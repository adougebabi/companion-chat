import StarRating from '@/react-app/shared/StarRating';
import {DiaryCalendar} from '@/react-app/shared/DateField';
import { readValue } from "@/state/view-model";
import { cn } from "@/lib/utils";
import {
  ActionButton,
  Surface,
  FieldInput,
  Textarea,
  MenuDisclosure,
  DialogFrame,
  Badge,
} from "@/react-app/controls";
import {
  displayText,
  eventModifiers,
  keyModifiers,
  listValues,
  Fragment,
} from "@/react-app/render-helpers";
export function LifePageView({ view: view }) {
  return (
    <Fragment>
      {view.props.page === "此刻" ? (
        <Fragment key={0}>
          <Surface as="div" className="card now-card">
            <span className="eyebrow">
              {displayText(readValue(view.todayLabel))}
            </span>
            <h2>
              {displayText(
                readValue(view.character).activity || "此刻状态尚未更新",
              )}
            </h2>
            <p>
              {displayText(
                view.joined(
                  readValue(view.character).location,
                  readValue(view.character).mood,
                ),
              )}
            </p>
            <p>{displayText(readValue(view.character).world)}</p>
          </Surface>
          <div className="grid now-grid">
            <Surface as="div" className="card">
              <view.CurrentOutfit compact="" />
            </Surface>
            <Surface as="section" className="card now-appointments">
              <div className="row">
                <h2>{"约定"}</h2>
                <span className="spacer" />
                <ActionButton
                  className="text-button"
                  data-testid="now-appointments"
                  onClick={(k) => {
                    ((view.appointmentHighlight.value = !0),
                      view.emit("go", "目标"));
                  }}
                >
                  {"查看全部 ↗"}
                </ActionButton>
              </div>
              <Fragment>
                {listValues(view.life.value.appointments.slice(0, 2)).map(
                  (k) => (
                    <div key={k.date + k.time} className="appointment-row">
                      <time>
                        {displayText(k.date.slice(5).replace("-", "月")) + "日"}
                        <br />
                        {displayText(k.time)}
                      </time>
                      <span>{displayText(k.text)}</span>
                    </div>
                  ),
                )}
              </Fragment>
              {view.life.value.appointments.length ? null : (
                <p key={0} className="muted">
                  {"还没有约定，留一些期待给明天。"}
                </p>
              )}
            </Surface>
          </div>
        </Fragment>
      ) : null}
      {view.props.page === "此刻" || view.props.page === "日程" ? (
        <Fragment key={1}>
          <h2>
            {displayText(
              view.props.page === "此刻" ? "此刻与接下来" : "今天的安排",
            )}
          </h2>
          {view.props.page === "日程" && <ActionButton className="button" onClick={view.openSchedulePlanning}>查看与规划日程</ActionButton>}
          <Surface
            as="section"
            className="card schedule-list"
            data-testid="schedule-list"
          >
            <Fragment>
              {listValues(
                view.props.page === "此刻"
                  ? view.upcoming.value
                  : view.schedule.value,
              ).map((k) => (
                <div
                  key={k.time + k.text}
                  className={cn([
                    "schedule-row",
                    {
                      "is-current": k === view.active.value,
                    },
                  ])}
                  data-testid={"schedule-" + k.time}
                >
                  {view.editing.value ===
                    readValue(view.character).schedule.indexOf(k) &&
                  view.props.page === "日程" ? (
                    <form
                      key={0}
                      className="schedule-editor"
                      onSubmit={eventModifiers(view.saveSchedule, ["prevent"])}
                    >
                      <FieldInput
                        type="time"
                        aria-label="时间"
                        value={view.time.value}
                        onChange={(event) => {
                          ((ve) => (view.time.value = ve))(event.target.value);
                        }}
                      />
                      <FieldInput
                        aria-label="日程内容"
                        value={view.text.value}
                        onChange={(event) => {
                          ((ve) => (view.text.value = ve))(event.target.value);
                        }}
                      />
                      <ActionButton
                        className="button"
                        disabled={
                          readValue(view.scheduleBusy) ||
                          !view.text.value.trim() ||
                          !view.time.value
                        }
                        data-testid="schedule-save"
                      >
                        {displayText(
                          readValue(view.scheduleBusy) ? "处理中…" : "保存",
                        )}
                      </ActionButton>
                      <ActionButton
                        type="button"
                        className="text-button"
                        disabled={readValue(view.scheduleBusy)}
                        onClick={(ve) => (view.editing.value = null)}
                      >
                        {"取消"}
                      </ActionButton>
                    </form>
                  ) : (
                    <Fragment key={1}>
                      <time>{displayText(k.time)}</time>
                      <span className="schedule-dot" />
                      <span className="schedule-text">
                        {displayText(k.text)}
                      </span>
                      {k === view.active.value ? (
                        <Badge
                          key={0}
                          className="pill"
                          data-testid="schedule-current"
                        >
                          {"进行中"}
                        </Badge>
                      ) : null}
                      {view.props.page === "日程" ? (
                        <MenuDisclosure
                          key={1}
                          className="actions-menu"
                          title={<>{"⋯"}</>}
                          aria-label="日程操作"
                        >
                          <div className="scroll-lock-list" data-scroll-lock="">
                            <ActionButton
                              disabled={
                                readValue(view.scheduleBusy) ||
                                k.editable === false
                              }
                              onClick={(ve) => view.editSchedule(k, ve)}
                            >
                              {"编辑"}
                            </ActionButton>
                          </div>
                        </MenuDisclosure>
                      ) : null}
                    </Fragment>
                  )}
                </div>
              ))}
            </Fragment>
            {(view.props.page === "此刻"
              ? view.upcoming.value
              : view.schedule.value
            ).length ? null : (
              <div key={0} className="muted">
                <p>{"暂时没有已确认的日程。"}</p>
                {view.props.page === "日程" ? (
                  <div className="row">
                    <ActionButton
                      className="button"
                      onClick={view.recoverInitialSchedule}
                    >
                      {"恢复初始日程"}
                    </ActionButton>
                  </div>
                ) : null}
              </div>
            )}
          </Surface>
        </Fragment>
      ) : null}
      {view.props.page === "目标" ? (
        <Fragment key={2}>
          <p className="muted">{"想做的事慢慢来，约好的事认真记住。"}</p>
          <div className="small-card-grid" data-testid="goal-grid">
            <Fragment>
              {listValues(readValue(view.character).goals).map((k, ve) => (
                <Surface as="article" key={k.id} className="card goal-card">
                  <div className="row">
                    <Badge className="pill">
                      {displayText(k.category || "目标")}
                    </Badge>
                    <span className="spacer" />
                    <small>
                      {displayText(k.done ? 100 : k.progress) + "%"}
                    </small>
                  </div>
                  <label className="goal">
                    <FieldInput
                      type="checkbox"
                      title="请求核对进展，完成状态由实际证据决定"
                      data-testid={"goal-toggle-" + ve}
                      modelValue={k.done}
                      onModelChange={(Ye) => view.toggleGoal(k, Ye)}
                    />
                    <span
                      className={cn({
                        done: k.done,
                      })}
                    >
                      {displayText(k.text)}
                    </span>
                  </label>
                  <div
                    className="progress"
                    role="progressbar"
                    aria-label={k.text}
                    aria-valuenow={k.done ? 100 : k.progress}
                    aria-valuemin={0}
                    aria-valuemax={100}
                  >
                    <i
                      style={{
                        width: (k.done ? 100 : k.progress) + "%",
                      }}
                      data-testid={"goal-progress-" + ve}
                    />
                  </div>
                </Surface>
              ))}
            </Fragment>
          </div>
          <section
            id="appointments"
            className={cn({
              highlight: readValue(view.appointmentHighlight),
            })}
          >
            <h2 className="section-heading">{"约定"}</h2>
            <div className="small-card-grid" data-testid="appointment-grid">
              <Fragment>
                {listValues(
                  [...view.life.value.appointments].sort((k, ve) =>
                    (k.date + k.time).localeCompare(ve.date + ve.time),
                  ),
                ).map((k) => (
                  <Surface
                    as="article"
                    key={k.date + k.time}
                    className="card appointment-card"
                  >
                    <small>
                      {displayText(k.date) + " · " + displayText(k.time)}
                    </small>
                    <h3>{displayText(k.text)}</h3>
                    <span className="muted">
                      {"留一个位置，给共同的期待。"}
                    </span>
                  </Surface>
                ))}
              </Fragment>
            </div>
          </section>
        </Fragment>
      ) : null}
      {view.props.page === "日记" ? (
        <Fragment key={3}>
          <div className="row diary-heading">
            <div>
              <h2>{"写给自己的时光"}</h2>
              <p className="muted">{"有些心事，留在纸页之间。"}</p>
            </div>
            <span className="spacer" />
            <view.DiaryGeneration
              date={view.selectedDate.value}
              onSelect={view.selectDate}
            />
          </div>
          <div className="diary-layout">
            <Surface as="section" className="card calendar">
              <DiaryCalendar value={view.selectedDate.value} max={view.closedDiaryDate.value} dates={view.diaries.value.map(entry=>entry.date)} onChange={view.selectDate}/>
            </Surface>
            <section
              className="diary-book"
              data-testid="diary-book"
              aria-label="日记簿"
            >
              <Fragment>
                {view.entry.value ? (
                  <article
                    key={view.entry.value.date}
                    className="diary-paper"
                    data-testid="diary-entry"
                    data-date={view.entry.value.date}
                  >
                    <small>
                      {displayText(
                        view.joined(
                          view.entry.value.date,
                          view.entry.value.time,
                        ),
                      )}
                    </small>
                    <h2>{displayText(view.entry.value.title)}</h2>
                    <div className="row diary-attributes">
                      {view.entry.value.weather ? (
                        <span>
                          {"天气 · " + displayText(view.entry.value.weather)}
                        </span>
                      ) : null}
                      {view.entry.value.mood ? (
                        <span>
                          {"心情 · " + displayText(view.entry.value.mood)}
                        </span>
                      ) : null}
                    </div>
                    <p className="diary-text">
                      {displayText(view.entry.value.text)}
                    </p>
                    <p className="diary-signature">
                      {"— " + displayText(readValue(view.character).name)}
                    </p>
                  </article>
                ) : (
                  <div key={1} className="diary-paper empty-state">
                    {[<p key={0}>{"这一天还没有日记。"}</p>]}
                  </div>
                )}
              </Fragment>
              <footer className="book-controls">
                <ActionButton
                  className="text-button"
                  disabled={view.diaryIndex.value <= 0}
                  data-testid="diary-prev"
                  onClick={(k) => view.moveDiary(-1)}
                >
                  {"← 上一页"}
                </ActionButton>
                <small aria-live="polite" data-testid="diary-page-indicator">
                  {displayText(
                    view.entry.value ? view.diaryIndex.value + 1 : 0,
                  ) +
                    " / " +
                    displayText(view.diaries.value.length)}
                </small>
                <ActionButton
                  className="text-button"
                  disabled={
                    view.diaryIndex.value < 0 ||
                    view.diaryIndex.value >= view.diaries.value.length - 1
                  }
                  data-testid="diary-next"
                  onClick={(k) => view.moveDiary(1)}
                >
                  {"下一页 →"}
                </ActionButton>
              </footer>
            </section>
          </div>
        </Fragment>
      ) : null}
      {view.props.page === "记忆" ? (
        <Fragment key={4}>
          <p className="muted">{"把相处留下的细节，慢慢收进记忆。"}</p>
          <p className="muted">
            {
              "对话会先在空闲后整理成片段，每日经历在当天结束后形成；此列表展示已正式保存的记忆。向量检索需要配置记忆检索模型。"
            }
            <ActionButton asChild className="text-button">
              <a
                className="text-button"
                href={"#/settings?section=" + encodeURIComponent("模型用途")}
              >
                {"查看模型用途 ↗"}
              </a>
            </ActionButton>
          </p>
          <view.FilterChips
            search={view.search.value}
            onUpdateSearch={(k) => (view.search.value = k)}
            filters={view.filters.value}
            onUpdateFilters={(k) => (view.filters.value = k)}
            groups={view.groups.value}
            testId="memory-filters"
            placeholder="搜索记忆 / 标签"
          />
          <section className="memory-list">
            <Fragment>
              {listValues(view.memories.value).map((k) => (
                <article
                  key={k.id}
                  className={cn([
                    "memory-item",
                    {
                      expanded: view.expandedMemoryId.value === k.id,
                    },
                  ])}
                  data-testid={"memory-row-" + k.id}
                >
                  <ActionButton
                    className="memory-summary"
                    aria-expanded={view.expandedMemoryId.value === k.id}
                    aria-controls={"memory-details-" + k.id}
                    data-testid={"memory-toggle-" + k.id}
                    onClick={(ve) =>
                      (view.expandedMemoryId.value =
                        view.expandedMemoryId.value === k.id ? null : k.id)
                    }
                  >
                    <span className="memory-chevron">
                      {displayText(
                        view.expandedMemoryId.value === k.id ? "−" : "+",
                      )}
                    </span>
                    <span className="memory-content">
                      {displayText(k.text)}
                    </span>
                    <time>{displayText(k.date)}</time>
                    <Badge className="pill">{displayText(k.category)}</Badge>
                  </ActionButton>
                  {view.expandedMemoryId.value === k.id ? (
                    <div
                      key={0}
                      id={"memory-details-" + k.id}
                      className="memory-details"
                    >
                      <dl>
                        <div>
                          <dt>{"来源"}</dt>
                          <dd>
                            {displayText(k.source) +
                              " · " +
                              displayText(k.date)}
                          </dd>
                        </div>
                        <div>
                          <dt>{"重要度"}</dt>
                          <dd><StarRating value={k.importance}/></dd>
                        </div>
                        <div>
                          <dt>{"标签"}</dt>
                          <dd>
                            <Fragment>
                              {listValues(k.tags).map((ve) => (
                                <Badge key={ve} className="pill">
                                  {"# " + displayText(ve)}
                                </Badge>
                              ))}
                            </Fragment>
                            {k.tags.length ? null : (
                              <span key={0} className="muted">
                                {"暂无标签"}
                              </span>
                            )}
                          </dd>
                        </div>
                      </dl>
                      <div className="row">
                        <ActionButton
                          className="text-button"
                          data-testid={"memory-edit-" + k.id}
                          onClick={(ve) => {
                            ((view.draftMemory.value = {
                              ...k,
                              tags: [...k.tags],
                            }),
                              (view.newTag.value = ""));
                          }}
                        >
                          {"编辑"}
                        </ActionButton>
                        <ActionButton
                          className="text-button"
                          data-testid={"memory-forget-" + k.id}
                          onClick={(ve) => (view.forgetMemory.value = k)}
                        >
                          {"遗忘"}
                        </ActionButton>
                      </div>
                    </div>
                  ) : null}
                </article>
              ))}
            </Fragment>
          </section>
          {view.memories.value.length ? null : (
            <Surface as="div" key={0} className="card empty-state">
              <p>{"没有符合条件的记忆。"}</p>
              <ActionButton
                className="button"
                data-testid="memory-empty-clear"
                onClick={view.clearFilters}
              >
                {"清除筛选"}
              </ActionButton>
            </Surface>
          )}
          {view.draftMemory.value ? (
            <view.DetailDrawer
              key={1}
              title="编辑记忆"
              testId="memory-drawer"
              onClose={(k) => (view.draftMemory.value = null)}
            >
              <form
                className="form-grid"
                onSubmit={eventModifiers(view.saveMemory, ["prevent"])}
              >
                <label>
                  {"内容"}
                  <Textarea
                    rows="4"
                    data-testid="memory-edit-content"
                    value={view.draftMemory.value.text}
                    onChange={(event) => {
                      ((k) => (view.draftMemory.value.text = k))(
                        event.target.value,
                      );
                    }}
                  />
                  {view.draftMemory.value.text.trim() ? null : (
                    <small key={0} className="inline-error">
                      {"请填写记忆内容"}
                    </small>
                  )}
                </label>
                <label>
                  {"来源"}
                  <FieldInput
                    readOnly={true}
                    value={view.draftMemory.value.source}
                    onChange={(event) => {
                      ((k) => (view.draftMemory.value.source = k))(
                        event.target.value,
                      );
                    }}
                  />
                </label>
                <label>
                  {"来源日期"}
                  <FieldInput
                    type="date"
                    readOnly={true}
                    value={view.draftMemory.value.date}
                    onChange={(event) => {
                      ((k) => (view.draftMemory.value.date = k))(
                        event.target.value,
                      );
                    }}
                  />
                </label>
                <section className="grid gap-3 rounded-lg border bg-muted/30 p-4"><h3 className="font-medium">记录信息（只读）</h3><p className="text-sm text-muted-foreground">评分、来源、分类和标签由实际记忆记录维护，此处只修正正文。</p><StarRating value={view.draftMemory.value.importance}/><Badge variant="secondary">{view.draftMemory.value.category}</Badge><div className="flex flex-wrap gap-2">{view.draftMemory.value.tags.map(tag=><Badge key={tag} variant="outline">{tag}</Badge>)}</div></section>
                <view.ActionFeedback labels={["记忆"]} />
                <ActionButton
                  className="button primary"
                  disabled={
                    readValue(view.memoryBusy) ||
                    !view.draftMemory.value.text.trim()
                  }
                  data-testid="memory-save"
                >
                  {displayText(readValue(view.memoryBusy) ? "处理中…" : "保存")}
                </ActionButton>
              </form>
            </view.DetailDrawer>
          ) : null}
          {view.forgetMemory.value ? (
            <DialogFrame
              key={2}
              className="modal-backdrop"
              onClose={() => (view.forgetMemory.value = null)}
            >
              <Surface
                as="section"
                className="card confirm-dialog"
                role="alertdialog"
                aria-modal="true"
                aria-label="遗忘这段记忆？"
                data-testid="memory-forget-confirm"
              >
                <h2>{"遗忘这段记忆？"}</h2>
                <p>{displayText(view.forgetMemory.value.text)}</p>
                <div className="row">
                  <ActionButton
                    className="button"
                    onClick={(k) => (view.forgetMemory.value = null)}
                  >
                    {"取消"}
                  </ActionButton>
                  <ActionButton
                    className="button primary"
                    data-testid="memory-confirm-forget"
                    onClick={() => view.forget()}
                    disabled={view.memoryBusy.value}
                  >
                    {"确认遗忘"}
                  </ActionButton>
                </div>
              </Surface>
            </DialogFrame>
          ) : null}
        </Fragment>
      ) : null}
      {view.draftMemory.value ? null : (
        <view.ActionFeedback
          key={view.props.page}
          labels={[view.props.page === "此刻" ? "日程" : view.props.page]}
        />
      )}
    </Fragment>
  );
}
