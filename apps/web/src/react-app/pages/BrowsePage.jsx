import InboxPage from "@/react-app/pages/InboxPage";
// App.tsx routes every other workspace page to its own page before falling back
// here, so the only page this renders is 通知与待处理. Anything else renders
// nothing, as the former generic browse view did for an unknown page.
/** @param {{page: string, onGo?: (page: string) => void}} input */
export default function BrowsePage(input) {
  return input.page === "通知与待处理" ? <InboxPage /> : null;
}
