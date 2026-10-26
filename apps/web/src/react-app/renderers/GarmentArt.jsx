import { Fragment } from "@/react-app/render-helpers";
export function GarmentArtView({ view: __view }) {
  return (
    <svg
      viewBox="0 0 100 100"
      width="84"
      height="84"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {__view.props.type === "鞋子" ? (
        <Fragment key={0}>
          <path
            d="M18 55l15-25 16 11 8 17 25 8v13H16V62z"
            fill="currentColor"
            opacity=".3"
          />
          <path d="M18 55l15-25 16 11 8 17 25 8v13H16V62zM17 72h65M39 47l12-4M43 54l12-4" />
        </Fragment>
      ) : __view.props.type === "下衣" ? (
        <Fragment key={1}>
          <path
            d="M28 15h44l7 70H55L50 47 45 85H21z"
            fill="currentColor"
            opacity=".25"
          />
          <path d="M28 15h44l7 70H55L50 47 45 85H21zM28 25h44M50 25v22" />
        </Fragment>
      ) : __view.props.type === "配饰" ? (
        <Fragment key={2}>
          <circle cx="50" cy="50" r="21" />
          <path d="M39 30V10h22v20M39 70v20h22V70M50 36v16l10 7" />
        </Fragment>
      ) : __view.props.type === "袜子" ? (
        <Fragment key={3}>
          <path
            d="M37 14h28v45l-22 25c-13 9-31-4-20-17l14-13z"
            fill="currentColor"
            opacity=".25"
          />
          <path d="M37 14h28v45l-22 25c-13 9-31-4-20-17l14-13zM37 25h28M26 64l22 15" />
        </Fragment>
      ) : __view.props.type === "内裤" ? (
        <Fragment key={4}>
          <path d="M18 28h64l-15 40H33z" fill="currentColor" opacity=".25" />
          <path d="M18 28h64l-15 40H33zM20 37h60M22 42l22 26M78 42L56 68" />
        </Fragment>
      ) : (
        <Fragment key={5}>
          <path
            d="M34 18l-21 13 10 24 12-6v35h30V49l12 6 10-24-21-13c-4 13-28 13-32 0z"
            fill="currentColor"
            opacity=".25"
          />
          <path d="M34 18l-21 13 10 24 12-6v35h30V49l12 6 10-24-21-13c-4 13-28 13-32 0z" />
          {__view.props.type === "外套" ? (
            <path key={0} d="M50 28v56M34 18l8 26 8-9 8 9 8-26" />
          ) : null}
        </Fragment>
      )}
    </svg>
  );
}
function renderGarmentArtBody(__view) {
  return () => <GarmentArtView view={__view} />;
}
export const renderGarmentArt = renderGarmentArtBody;
