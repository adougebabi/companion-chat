// Decorative category sketches copied from the supplied Demo; never a real item photograph.
// Explicit color words select a Demo illustration tone, not a persisted item
// color or a claim about the photograph. Unspecified colors keep the neutral tone.
function illustrationTone(type:string,name:string) {
  if (/奶油色|\bcream\b/i.test(name)) return '#c6bea1';
  if (/米白|奶白|\bivory\b|\boff-white\b/i.test(name))
    return type==='lower_body'?'#d6d1b9':type==='socks'?'#e5e3cf':'#e6e2d1';
  if (/棕色|深棕|浅棕|\bbrown\b/i.test(name)) return '#876954';
  if (/苔绿|\bmoss green\b/i.test(name)) return '#7d8e74';
  if (/深灰|\bcharcoal\b|\bdark gr[ae]y\b/i.test(name)) return '#747d74';
  return '#aeb59c';
}
export default function WorkspaceGarmentArt({
  type,
  color,
  className = "",
  name = "",
}: {
  type: string;
  color?: string;
  className?: string;
  name?: string;
}) {
  const line = "#66715e", tone=color||illustrationTone(type,name);
  return (
    <svg
      className={"r-garment-svg " + className}
      viewBox="0 0 200 180"
      fill="none"
      role="img"
      aria-label={name ? name + "（类别示意，非实物图片）" : "服装类别示意图"}
    >
      <ellipse cx="100" cy="158" rx="55" ry="6" fill="#dce1d4" opacity=".6" />
      {["outer", "upper_body", "underwear"].includes(type) ? (
        <>
          <path
            d="m73 27-29 13-22 40 24 13 13-23-3 78h88l-3-78 13 23 24-13-22-40-29-13c-10 12-43 12-54 0Z"
            fill={tone}
            stroke={line}
            strokeWidth="1.2"
          />
          <path
            d="M73 27c3 17 14 26 27 26s24-9 27-26"
            stroke={line}
            strokeWidth="1.1"
          />
          {type === "outer" ? (
            <>
              <path
                d="m73 28 27 25-12 16 12 11v68M127 28l-27 25 12 16-12 11"
                stroke={line}
                strokeWidth="1.1"
              />
              <path
                d="M67 106h17v20H67Zm49 0h17v20h-17Z"
                stroke={line}
                strokeWidth="1.1"
              />
              {[87, 106, 125].map((y) => (
                <circle key={y} cx="104" cy={y} r="1.8" fill={line} />
              ))}
            </>
          ) : (
            <path
              d="M57 137h86M43 80l7 3m100 0 7-3"
              stroke={line}
              opacity=".5"
            />
          )}
          <path d="m61 54-4 68m82-68 4 68" stroke={line} opacity=".15" />
        </>
      ) : type === "lower_body" && /裙/.test(name) ? (
        <>
          <path
            d="M77 27h46l30 120c-28 11-78 11-106 0Z"
            fill={tone}
            stroke={line}
            strokeWidth="1.2"
          />
          <path
            d="M76 37h49M87 44l-9 99m22-99v103m13-103 10 99"
            stroke={line}
            strokeWidth="1"
            opacity=".45"
          />
          <path
            d="M50 139c28 10 73 10 100 0"
            stroke="#fff"
            strokeWidth="1.5"
            opacity=".55"
          />
        </>
      ) : type === "lower_body" ? (
        <>
          <path
            d="M69 28h62l12 122h-33L99 78l-10 72H55Z"
            fill={tone}
            stroke={line}
            strokeWidth="1.2"
          />
          <path
            d="M68 37h64M99 38v40m-26-34-6 14m59-14 6 14"
            stroke={line}
            strokeWidth="1"
          />
          <path d="m78 68-8 72m50-72 8 72" stroke="#fff" opacity=".25" />
        </>
      ) : type === "shoes" ? (
        <>
          <path
            d="M32 106c10 0 17-10 28-27l24 6 9 24c17 5 29 10 28 21H28c-7-8-4-20 4-24Z"
            fill={tone}
            stroke={line}
            strokeWidth="1.2"
          />
          <path
            d="M92 81c10 0 17-10 28-27l24 6 9 24c17 5 29 10 28 21H88c-7-8-4-20 4-24Z"
            fill={tone}
            stroke={line}
            strokeWidth="1.2"
          />
          <path
            d="M28 123h91M88 98h91M47 103l36 2M107 78l36 2"
            stroke="#eadbc0"
            strokeWidth="3"
          />
          <path d="M31 131h89M91 106h89" stroke={line} strokeWidth="3" />
        </>
      ) : type === "socks" ? (
        <>
          <path
            d="M64 34h29v76c-4 25-49 37-56 19-5-12 8-20 27-23Z"
            fill={tone}
            stroke={line}
          />
          <path
            d="M111 34h29v76c-4 25-49 37-56 19-5-12 8-20 27-23Z"
            fill={tone}
            stroke={line}
          />
          <path
            d="M64 44h29m18 0h29M66 49h25m22 0h25"
            stroke={line}
            opacity=".5"
          />
        </>
      ) : type === "panties" ? (
        <>
          <path d="M48 58h104l-24 64H72Z" fill={tone} stroke={line} />
          <path
            d="M49 67h102M54 76c21 9 29 25 31 45m61-45c-21 9-29 25-31 45"
            stroke={line}
            opacity=".6"
          />
        </>
      ) : (
        <>
          <path
            d="M65 25h48l22 118-31 7-18-91-14 78-28-6Z"
            fill={tone}
            stroke={line}
            strokeWidth="1.2"
          />
          <path
            d="m66 36 42 1m-4 107 30-6M45 124l28 6"
            stroke="#e7e0cd"
            strokeWidth="3"
          />
          <path
            d="m53 133-2 10m10-8-1 11m51 3 2 10m9-12 2 10m9-12 2 9"
            stroke={line}
            strokeWidth="1.2"
          />
        </>
      )}
    </svg>
  );
}
