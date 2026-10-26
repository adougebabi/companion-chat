import { useController } from "@/react-app/page";
import { selector } from "@/state/view-model";
import { FieldInput, Disclosure, Badge } from "@/react-app/controls";
import { displayText, listValues, Fragment } from "@/react-app/render-helpers";
function createModelDirectoryController(__props, context, lifecycle) {
  const props = __props,
    query = lifecycle.cell("");
  const models = selector(() =>
      props.models
        .map((m) => (typeof m === "string" ? m : m.id || m.name))
        .filter(Boolean),
    ),
    filtered = selector(() =>
      models.value.filter((m) =>
        m.toLowerCase().includes(query.value.trim().toLowerCase()),
      ),
    );
  return (_ctx, _cache) => {
    return (
      <Disclosure
        className="approved-model-directory"
        title={<>{"可用模型 · " + displayText(models.value.length) + " 项"}</>}
      >
        <FieldInput
          type="search"
          aria-label="搜索可用模型"
          placeholder="搜索模型 ID"
          value={query.value}
          onChange={(event) => {
            (($event) => (query.value = $event))(event.target.value);
          }}
        />
        <div className="chip-options scroll-lock-list" data-scroll-lock="">
          <Fragment>
            {listValues(filtered.value).map((model) => {
              return (
                <Badge key={model} className="pill" title={model}>
                  {displayText(model)}
                </Badge>
              );
            })}
          </Fragment>
          {!models.value.length ? (
            <small key={0} className="muted">
              {"尚未读取模型目录，可在用途配置中填写模型 ID。"}
            </small>
          ) : !filtered.value.length ? (
            <small key={1} className="muted">
              {"没有匹配的模型。"}
            </small>
          ) : null}
        </div>
      </Disclosure>
    );
  };
}
export default function ModelDirectory(input = {}) {
  return useController(createModelDirectoryController, {
    models: (() => [])(),
    ...input,
  });
}
