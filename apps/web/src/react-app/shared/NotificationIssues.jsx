import {ArrowUpRightIcon as ExternalLinkIcon,MoreHorizontalIcon as MoreActionsIcon} from "lucide-react";
import { ActionButton, Surface, Disclosure } from "@/react-app/controls";
import { displayText, listValues, Fragment } from "@/react-app/render-helpers";
export default function NotificationIssues(input = {}) {
  const __props = {
    ...input,
  };
  return (
    <Fragment>
      {listValues(__props.groups).map((group) => {
        return (
          <Surface
            key={group.id}
            as="section"
            className="notification-group"
          >
            <Disclosure
              title={
                <>
                  <b>{displayText(group.label)}</b>
                  <span className="spacer" />
                  <span>{displayText(group.items.length) + " 条记录"}</span>
                </>
              }
            >
              <Fragment>
                {listValues(group.items).map((item) => {
                  return (
                    <ActionButton variant="ghost"
                      key={item.id}
                      className="notification-item h-auto min-h-14 whitespace-normal text-left"
                      data-testid={"notification-diagnostic-" + item.id}
                      onClick={($event) => __props.onOpen?.(item.id)}
                    >
                      <b>{displayText(item.label)}</b>
                      <p>{displayText(item.reason)}</p>
                      <span>{"查看原诊断 "}<ExternalLinkIcon className="inline size-3.5 shrink-0" aria-hidden="true"/></span>
                    </ActionButton>
                  );
                })}
              </Fragment>
            </Disclosure>
          </Surface>
        );
      })}
    </Fragment>
  );
}
