// Decorative scene from the supplied Demo; no runtime facts are inferred.
import {Coffee,Sprout} from 'lucide-react';
import {cn} from '@/lib/utils';
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
          <Coffee size={28} strokeWidth={1.25} />
        </span>
        <span className="companion-plant">
          <Sprout size={67} strokeWidth={1} />
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
