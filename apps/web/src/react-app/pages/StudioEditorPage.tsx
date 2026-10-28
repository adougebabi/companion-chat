import {
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { useWorkspace } from "../page";
import StudioJobs from "../shared/StudioJobs";
import StudioEditorView from "../views/StudioEditor";
import { createStudioEditor } from "@/state/studio-editor.mjs";
import { studioChanges } from "@/v3/proto/components/studio-comparison.js";
const clone = (value: any) => JSON.parse(JSON.stringify(value));
export default function StudioEditorPage({
  resourceId,
  kind = "event_template",
  jobId = "",
  sandbox = false,
}: {
  resourceId: string;
  kind?: string;
  jobId?: string;
  sandbox?: boolean;
}) {
  const formRef = useRef<HTMLFormElement | null>(null);
  const dialogTriggers = useRef<Record<string, HTMLElement>>({});
  const w = useWorkspace(),
    owner = w.state.session.actorId;
  const controller = useMemo(
    () =>
      createStudioEditor({
        owner,
        resourceId,
        kind,
        jobId,
        allowNpc: true,
        service: w.services.studioClient,
        ownerCurrent: () => w.state.session?.actorId === owner,
        navigate: (path: string) => w.navigatePath(path),
      }),
    [w, owner, resourceId],
  );
  const state: any = useSyncExternalStore(
    controller.subscribe,
    controller.getSnapshot,
    controller.getSnapshot,
  );
  useEffect(() => {
    void controller.start();
    return () => controller.dispose();
  }, [controller]);
  const d = state.draft,
    r = state.resource,
    readOnly = r?.status === "archived",
    preview = state.preview;
  const changes =
    d &&
    studioChanges(
      { name: r?.name || "", body: r?.body || {} },
      { name: d.name, body: d.body },
    );
  const resourceChanged = (value: any) => {
    controller.state.resource = clone(value);
    controller.state.preview = null;
  };
  const jobs = d ? (
    <StudioJobs
      key={r?.id || "new"}
      resource={r}
      readDraft={() => controller.state.draft}
      adopt={controller.adoptDefinition}
      jobId={state.jobId}
      onSelect={(id: string) => (controller.state.jobId = id)}
      initialKind="assistant"
      onResourceChanged={resourceChanged}
    />
  ) : null;

  const [applyOpen, setApplyOpen] = useState(false),
    [historyOpen, setHistoryOpen] = useState(false);
  const changeMode = () => {
    const query = new URLSearchParams({
      kind: d?.kind || kind,
      ...(state.jobId ? { job: state.jobId } : {}),
    });
    if (!sandbox) query.set("res", resourceId);
    w.navigatePath(
      (sandbox
        ? "/studio/edit/" + encodeURIComponent(resourceId)
        : "/studio/preview") +
        "?" +
        query,
    );
  };
  return (
    <StudioEditorView
      model={{
        w,
        formRef,
        dialogTriggers,
        save: () => formRef.current?.requestSubmit(),
        controller,
        state,
        d,
        r,
        readOnly,
        preview,
        changes,
        jobs,
        resourceId,
        kind,
        sandbox,
        resourceChanged,
        applyOpen,
        setApplyOpen: (open: boolean) => {
          if (open) controller.state.lastApplication = null;
          setApplyOpen(open);
        },
        historyOpen,
        setHistoryOpen,
        changeMode,
      }}
    />
  );
}
