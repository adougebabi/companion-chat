import { useId } from "react";
// Supplied Demo decoration only; never an avatar, candidate or generated result.
export default function WorkspaceCharacterSketch({
  className,
}: {
  className?: string;
}) {
  const id = useId().replace(/:/g, "");
  return (
    <svg
      className={className}
      viewBox="0 0 400 480"
      fill="none"
      role="img"
      aria-label="初始化装饰插画：形象示意，非角色实际生成结果"
      preserveAspectRatio="xMidYMid slice"
    >
      <defs>
        <linearGradient
          id={`${id}-paper`}
          x1="0"
          y1="0"
          x2="400"
          y2="480"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#E8E9D9" />
          <stop offset="1" stopColor="#D3D9C7" />
        </linearGradient>
        <pattern
          id={`${id}-grain`}
          width="12"
          height="12"
          patternUnits="userSpaceOnUse"
        >
          <circle cx="2" cy="3" r=".55" fill="#59715B" opacity=".09" />
          <circle cx="9" cy="9" r=".45" fill="#FFFDF0" opacity=".45" />
        </pattern>
      </defs>
      <path fill={`url(#${id}-paper)`} d="M0 0h400v480H0z" />
      <circle cx="283" cy="108" r="79" fill="#EFEDD9" />
      <path d="M32 47h161v230H32Z" fill="#ACB69C" />
      <path d="M39 54h147v213H39Z" fill="#D6D9C2" />
      <g fill="#F2ECDD">
        <path d="M49 69h12v58H49ZM65 79h17v48H65ZM87 65h13v62H87ZM121 78h14v49h-14ZM154 67h20v60h-20ZM48 150h19v55H48ZM75 146h12v59H75ZM111 153h17v52h-17ZM144 143h15v62h-15Z" />
      </g>
      <g fill="#7E927B">
        <path d="M102 72h15v55h-15ZM138 84h13v43h-13ZM90 140h17v65H90ZM130 157h10v48h-10ZM163 153h12v52h-12Z" />
      </g>
      <g stroke="#9AAB91" strokeWidth="2">
        <path d="M52 81h6m11 7h9m11-13h8m28 13h7m25-10h13M51 161h12m15-3h6m31 8h11m21-12h8" />
      </g>
      <path
        d="M29 127h164v8H29ZM29 205h164v8H29ZM29 267h164v10H29Z"
        fill="#9CAC8E"
      />
      <path d="M305 225h58v120h-58Z" fill="#BBC6AD" />
      <path d="M313 232h42v106h-42Z" fill="#E2E5D1" />
      <path d="M334 232v106m-21-53h42" stroke="#B4C0A5" strokeWidth="3" />
      <path
        d="M37 426c-3-60 18-106 66-120l57-17h77l61 17c44 13 65 63 61 120v54H37Z"
        fill="#5C745B"
      />
      <path d="m160 289 39 27 38-27 25 191H131Z" fill="#F5F0DF" />
      <path
        d="m147 294-34 10 20 74 33-26-10 128H85l17-148ZM247 294l33 10-18 74-32-26 11 128h73l-21-148Z"
        fill="#73866A"
      />
      <path d="M174 246h50v48c-3 13-12 23-25 23s-23-10-25-23Z" fill="#D7B59A" />
      <path
        d="M175 257c14 13 31 17 49 6v14c-17 12-38 9-49-1Z"
        fill="#C39B7F"
        opacity=".5"
      />
      <path
        d="M130 190c-15-56-5-101 35-118 35-15 82-7 104 24 24 35 18 72 12 105l-2 56c-21 13-42 15-62 12l-61 1-29-19Z"
        fill="#38473A"
      />
      <path
        d="M146 160c0-36 23-65 54-65s55 29 55 65v45c0 37-23 66-55 66s-54-29-54-66Z"
        fill="#E0BDA1"
      />
      <path
        d="M146 187c-14-11-21 0-14 18 3 8 9 11 17 10ZM254 187c14-11 21 0 14 18-3 8-9 11-17 10Z"
        fill="#D8AE91"
      />
      <path
        d="M138 184c-12-37-7-68 15-90 15-16 39-23 61-18 29 6 49 23 56 52 4 18 0 37-10 57l-8-37c-27-1-53-11-72-26-7 28-22 44-42 48Z"
        fill="#38473A"
      />
      <path
        d="M179 120c-7 22-19 34-34 43M193 99c20 28 40 41 63 43"
        stroke="#4A5947"
        strokeWidth="3"
        strokeLinecap="round"
      />
      <path
        d="M157 182c7-4 14-4 20-1m47 0c7-3 14-3 20 1"
        stroke="#5B5445"
        strokeWidth="2.5"
        strokeLinecap="round"
      />
      <path
        d="M160 194c4-3 9-3 13 0m54 0c4-3 9-3 13 0"
        stroke="#424B3E"
        strokeWidth="2"
        strokeLinecap="round"
      />
      <circle cx="167" cy="195" r="2" fill="#374339" />
      <circle cx="233" cy="195" r="2" fill="#374339" />
      <path
        d="m201 195-3 20 6 2"
        stroke="#BD9278"
        strokeWidth="1.8"
        strokeLinecap="round"
      />
      <path
        d="M189 234c7 4 15 4 23-1"
        stroke="#AD7868"
        strokeWidth="2"
        strokeLinecap="round"
      />
      <path
        d="M185 231h27"
        stroke="#D9A28B"
        strokeWidth="1.3"
        strokeLinecap="round"
      />
      <circle cx="143" cy="216" r="3.6" fill="#DCC98F" />
      <circle cx="257" cy="216" r="3.6" fill="#DCC98F" />
      <path d="m111 359 35 70 90-44-28-59Z" fill="#B7B096" />
      <path d="m145 429 89-45 8 12-91 45Z" fill="#EEE7D2" />
      <path d="m111 359 94-47 32 71-91 46Z" fill="#A3AB89" />
      <path d="m122 363 81-40 2 5-81 40Z" fill="#CFD0AF" />
      <path
        d="m160 363 27-13m-21 25 27-14m-19 24 27-13"
        stroke="#E5E3C8"
        strokeWidth="2"
        strokeLinecap="round"
      />
      <path
        d="M103 393c8 5 17 9 27 13l16 9c7 7 6 17-3 20-13 3-39-6-51-15Z"
        fill="#D7B398"
      />
      <path
        d="M271 380c-11 1-21 5-29 12l-17 11c-6 5-3 13 6 13 15-1 37-8 48-16Z"
        fill="#D7B398"
      />
      <path
        d="m63 351 40 43-12 30-39-28M313 349l-44 31 9 26 52-30"
        fill="#5C745B"
      />
      <path
        d="m70 361 29 35m207-37-33 24"
        stroke="#8A9C7F"
        strokeWidth="1.7"
        strokeLinecap="round"
      />
      <path d="M349 430c-16-19-18-41-6-64 16 13 19 32 6 64Z" fill="#93AA87" />
      <path d="M349 445c-29-2-48-17-49-39 25-1 43 14 49 39Z" fill="#A4B797" />
      <path d="M348 410v70" stroke="#6F8B64" strokeWidth="2" />
      <path fill={`url(#${id}-grain)`} d="M0 0h400v480H0z" />
    </svg>
  );
}
