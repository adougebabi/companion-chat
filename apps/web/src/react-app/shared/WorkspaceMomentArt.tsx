// Decorative scene from the supplied Demo; no runtime facts are inferred.
import {cn} from '@/lib/utils';
// Freeze these decorative paths from the supplied Demo so package icon updates
// do not alter the scene artwork. Ordinary control icons still use lucide-react.
function SceneCup(){return <svg aria-hidden="true" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round"><path d="M10 2v2"/><path d="M14 2v2"/><path d="M16 8a1 1 0 0 1 1 1v8a4 4 0 0 1-4 4H7a4 4 0 0 1-4-4V9a1 1 0 0 1 1-1h14a4 4 0 1 1 0 8h-1"/><path d="M6 2v2"/></svg>;}
function ScenePlant(){return <svg aria-hidden="true" width="67" height="67" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1" strokeLinecap="round" strokeLinejoin="round"><path d="M7 20h10"/><path d="M10 20c5.5-2.5.8-6.4 3-10"/><path d="M9.5 9.4c1.1.8 1.8 2.2 2.3 3.7-2 .4-3.5.4-4.8-.3-1.2-.6-2.3-1.9-3-4.2 2.8-.5 4.4 0 5.5.8z"/><path d="M14.1 6a7 7 0 0 0-1.1 4c1.9-.1 3.3-.6 4.3-1.4 1-1 1.6-2.3 1.7-4.6-2.7.1-4 1-4.9 2z"/></svg>;}
export default function WorkspaceMomentArt({
  kind = "books",
  compact = false,
}: {
  kind?: "books" | "river" | "garden";
  compact?: boolean;
}) {
  return (
    <div
      className={cn(
        "companion-scene",
        `companion-scene-${kind}`,
        compact && "companion-scene-compact",
      )}
      aria-label={
        kind === "books"
          ? "书店窗边的示意插画"
          : kind === "river"
            ? "河岸风景示意插画"
            : "窗边绿植示意插画"
      }
    >
      <div className="companion-scene-window">
        <i />
        <i />
      </div>
      <span className="companion-scene-sun" />
      <div className="companion-scene-table">
        <span className="companion-book one" />
        <span className="companion-book two" />
        <span className="companion-book three" />
        <span className="companion-cup">
          <SceneCup />
        </span>
        <span className="companion-plant">
          <ScenePlant />
        </span>
      </div>
      <div className="companion-scene-caption">
        <span>A LITTLE MOMENT</span>
        <strong>
          {kind === "books"
            ? "把时间，留给喜欢的事。"
            : kind === "river"
              ? "走慢一点，也没有关系。"
              : "等一阵风，和新的故事。"}
        </strong>
      </div>
    </div>
  );
}
