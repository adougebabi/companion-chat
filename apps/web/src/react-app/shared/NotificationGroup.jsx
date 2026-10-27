
import { readValue } from "@/state/view-model";
import { ActionButton, Surface, Disclosure } from "@/react-app/controls";
import { displayText, listValues, Fragment } from "@/react-app/render-helpers";
import CharacterAvatar from "@/react-app/shared/CharacterAvatar";
import { userInstant } from "@/v3/presentation-time.mjs";
export default function NotificationGroup(input = {}) {
  const __props = {
    ...input,
  };
  return (
    <Surface as="section" className="notification-group">
      <Disclosure
        title={
          <>
            <CharacterAvatar
              name={__props.group.name}
              initial={__props.group.initial}
              color={__props.group.color}
            />
            <b>{displayText(__props.group.name)}</b>
            <span className="spacer" />
            <span>{displayText(__props.group.notices.length) + " 条通知 · "+__props.group.notices.filter(notice=>!notice.read_at).length+" 条未读"}</span>
          </>
        }
      >
        <p className="muted">
          {displayText(
            __props.group.notices[0]?.preview || __props.group.notices[0]?.body,
          )}
        </p>
        <Fragment>
          {listValues(__props.group.notices).map((notice) => {
            return (
              <ActionButton variant="ghost"
                key={notice.id}
                className="notification-item h-auto min-h-14 whitespace-normal text-left"
                onClick={($event) => __props.onOpen?.(notice.id)}
              >
                <b>{displayText(notice.title || "新通知")}</b>
                <p>{displayText(notice.preview || notice.body)}</p>
                <small className="muted">
                  {displayText(
                    readValue(userInstant)(
                      notice.source_occurred_at ||
                        notice.occurred_at ||
                        notice.created_at,
                    ),
                  )}
                </small>
              </ActionButton>
            );
          })}
        </Fragment>
      </Disclosure>
    </Surface>
  );
}
