// Decorative artwork from the user-supplied 2026-10-27 workspace Demo.
// This is not a character portrait, generated asset or runtime world projection.
import { useId } from "react";

type ArtProps = { className?: string };

export function WorldArt({
  kind,
  className,
}: ArtProps & { kind: "city" | "forest" | "space" }) {
  const id = useId().replace(/:/g, "");
  if (kind === "forest") {
    return (
      <svg
        className={className}
        viewBox="0 0 400 240"
        fill="none"
        role="img"
        aria-label="自然森林：远山、树木与湖泊"
        preserveAspectRatio="xMidYMid slice"
      >
        <defs>
          <linearGradient
            id={`${id}-sky`}
            x1="200"
            y1="0"
            x2="200"
            y2="240"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#E4ECE7" />
            <stop offset="1" stopColor="#D2E0D7" />
          </linearGradient>
        </defs>
        <path fill={`url(#${id}-sky)`} d="M0 0h400v240H0z" />
        <circle cx="291" cy="60" r="28" fill="#F4F2DF" />
        <path
          d="M0 129 70 59l54 61 62-86 95 104 51-56 68 54v104H0Z"
          fill="#B9CFC3"
        />
        <path
          d="m70 59-17 38 19-10 19 11Zm116-25-23 47 24-13 21 10Z"
          fill="#D6E1D6"
        />
        <path
          d="M0 163c54-51 99-21 137-48 37-26 66-16 111 11 57 33 105-24 152 13v101H0Z"
          fill="#90B3A2"
        />
        <path
          d="M0 190c70-29 119-21 168-16 49 6 77-15 126-7 42 7 77 1 106-8v81H0Z"
          fill="#C0D3C6"
        />
        <path
          d="M171 166c22 8 14 15-9 24-36 14-17 24 50 27 24 1 46 8 57 23H89c-24-20 6-30 41-43 28-10 37-18 22-25Z"
          fill="#E9EEDC"
        />
        <path
          d="M0 206c47-15 83-8 109 3-19 9-30 19-20 31H0ZM400 199c-65-14-114 6-150 24l19 17h131Z"
          fill="#7DA18D"
        />
        <g fill="#507D67">
          <path d="m41 91-24 66h15l-21 29h60l-22-29h15Z" />
          <path d="m84 123-17 44h10l-16 22h46l-16-22h10Z" />
          <path d="m341 93-25 66h16l-23 30h64l-23-30h15Z" />
          <path d="m305 131-17 42h11l-15 21h44l-15-21h10Z" />
        </g>
        <g stroke="#39634F" strokeWidth="2" strokeLinecap="round">
          <path d="M41 134v66M341 137v67M84 157v43M305 162v40" />
        </g>
        <g fill="#335D48">
          <path d="m12 165-20 39h13l-13 22h42l-16-22h12Z" />
          <path d="m380 160-25 51h16l-18 29h54l-19-29h17Z" />
        </g>
        <g
          stroke="#F3F4E7"
          strokeWidth="1.3"
          strokeLinecap="round"
          opacity=".7"
        >
          <path d="M155 192h23M139 201h17M199 220h24M84 104h29M236 93h38M259 100h23" />
        </g>
        <g fill="#739583" opacity=".5">
          <circle cx="131" cy="54" r="1.5" />
          <circle cx="243" cy="55" r="1.5" />
          <circle cx="328" cy="37" r="1.5" />
        </g>
        <path
          d="m218 74 4-3 4 3m6-5 3-2 3 2"
          stroke="#6F8D7C"
          strokeLinecap="round"
        />
      </svg>
    );
  }
  if (kind === "space") {
    return (
      <svg
        className={className}
        viewBox="0 0 400 240"
        fill="none"
        role="img"
        aria-label="星际世界：星球、轨道与远方的星光"
        preserveAspectRatio="xMidYMid slice"
      >
        <defs>
          <linearGradient
            id={`${id}-sky`}
            x1="0"
            y1="0"
            x2="400"
            y2="240"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#E4E3EB" />
            <stop offset="1" stopColor="#D3D6E0" />
          </linearGradient>
          <linearGradient
            id={`${id}-planet`}
            x1="181"
            y1="77"
            x2="258"
            y2="166"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#FAF5E9" />
            <stop offset="1" stopColor="#B6B9CC" />
          </linearGradient>
        </defs>
        <path fill={`url(#${id}-sky)`} d="M0 0h400v240H0z" />
        <circle cx="206" cy="121" r="94" stroke="#F5F4F4" strokeWidth="1" />
        <circle
          cx="206"
          cy="121"
          r="116"
          stroke="#F5F4F4"
          strokeWidth="1"
          strokeDasharray="2 7"
        />
        <ellipse
          cx="205"
          cy="123"
          rx="129"
          ry="39"
          transform="rotate(-25 205 123)"
          stroke="#9D9FB5"
          strokeWidth="12"
        />
        <ellipse
          cx="205"
          cy="123"
          rx="129"
          ry="39"
          transform="rotate(-25 205 123)"
          stroke="#CCC8D4"
          strokeWidth="6"
        />
        <circle cx="205" cy="121" r="65" fill={`url(#${id}-planet)`} />
        <path
          d="M155 80c28-3 68 7 107 35M144 102c30 0 71 15 120 44M142 125c37 2 72 22 104 44"
          stroke="#C7C6CD"
          strokeWidth="9"
          opacity=".45"
        />
        <path
          d="M100 192c41 8 123-20 190-65 18-12 32-24 41-35"
          stroke="#9D9FB5"
          strokeWidth="12"
        />
        <path
          d="M100 192c41 8 123-20 190-65 18-12 32-24 41-35"
          stroke="#DDD5DB"
          strokeWidth="6"
        />
        <circle cx="94" cy="52" r="13" fill="#F5F0E3" />
        <circle cx="99" cy="48" r="2" fill="#DBD7CC" />
        <circle cx="88" cy="57" r="3" fill="#E1DDD2" />
        <circle cx="326" cy="180" r="21" fill="#ADB6C4" />
        <path
          d="M311 173c10-8 22-6 33 1M307 184c12-5 25-2 36 4"
          stroke="#C9CED5"
          strokeWidth="3"
        />
        <g fill="#898CA7">
          <circle cx="52" cy="106" r="2" />
          <circle cx="129" cy="213" r="1.5" />
          <circle cx="289" cy="41" r="2" />
          <circle cx="358" cy="85" r="1.5" />
          <circle cx="247" cy="214" r="1.5" />
          <circle cx="38" cy="202" r="1.5" />
        </g>
        <g fill="#F7F6EF">
          <path d="m57 48 2 6 6 2-6 2-2 6-2-6-6-2 6-2ZM342 120l2 7 7 2-7 2-2 7-2-7-7-2 7-2ZM276 193l1.5 4.5 4.5 1.5-4.5 1.5-1.5 4.5-1.5-4.5-4.5-1.5 4.5-1.5Z" />
        </g>
        <path
          d="m58 158 11-8m-7 12 21-16"
          stroke="#FBF8EE"
          strokeWidth="1.5"
          strokeLinecap="round"
        />
        <path d="M20 228c54-31 99-25 149 12H20Z" fill="#B5BDCA" opacity=".5" />
      </svg>
    );
  }
  return (
    <svg
      className={className}
      viewBox="0 0 400 240"
      fill="none"
      role="img"
      aria-label="现代城市：温柔日光下的建筑、桥与河流"
      preserveAspectRatio="xMidYMid slice"
    >
      <defs>
        <linearGradient
          id={`${id}-sky`}
          x1="200"
          y1="0"
          x2="200"
          y2="240"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#EBEEE4" />
          <stop offset="1" stopColor="#E0E5D9" />
        </linearGradient>
      </defs>
      <path fill={`url(#${id}-sky)`} d="M0 0h400v240H0z" />
      <circle cx="288" cy="64" r="30" fill="#EEE1B6" />
      <g stroke="#F9F9F0" strokeWidth="3" strokeLinecap="round">
        <path d="M37 60h43m-24 9h41M207 40h30m-20 9h38M331 100h37" />
      </g>
      <g fill="#CBD4C5">
        <path d="M15 126h36v56H15ZM56 110h28v72H56ZM96 87h36v95H96ZM249 105h38v77h-38ZM289 91h30v91h-30ZM335 121h52v61h-52Z" />
      </g>
      <path d="M84 108h54v83H84Z" fill="#9BAE9A" />
      <path d="M81 105h60v7H81Z" fill="#7D967F" />
      <path d="M146 66h56v125h-56Z" fill="#F8F4E6" />
      <path d="M153 59h42v7h-42Z" fill="#C0C5AD" />
      <path d="M208 94h46v97h-46Z" fill="#AFBEA7" />
      <path d="M219 86h24v8h-24Z" fill="#829980" />
      <path d="M265 131h58v60h-58Z" fill="#E8E7D4" />
      <path d="m260 131 34-22 34 22Z" fill="#80977F" />
      <g fill="#66836E">
        <path d="M157 79h9v13h-9ZM181 79h9v13h-9ZM157 103h9v13h-9ZM181 103h9v13h-9ZM157 127h9v13h-9ZM181 127h9v13h-9ZM97 120h7v10h-7ZM117 120h7v10h-7ZM97 140h7v10h-7ZM117 140h7v10h-7ZM219 108h7v13h-7ZM236 108h7v13h-7ZM219 133h7v13h-7ZM236 133h7v13h-7Z" />
      </g>
      <path
        d="M166 191v-30a9 9 0 0 1 18 0v30ZM284 191v-32a10 10 0 0 1 20 0v32Z"
        fill="#617E68"
      />
      <path d="M278 141h8v10h-8ZM303 141h8v10h-8Z" fill="#A2AF98" />
      <path d="M0 182h400v58H0Z" fill="#BDCEC4" />
      <path d="M0 178h400v10H0Z" fill="#809782" />
      <path
        d="M46 205v-17h309v17h-24c-7-14-25-14-32 0h-26c-7-14-25-14-32 0h-26c-7-14-25-14-32 0h-26c-7-14-25-14-32 0h-26c-7-14-25-14-32 0Z"
        fill="#EDEBD9"
      />
      <path d="M42 185h317v5H42Z" fill="#A6AF98" />
      <path
        d="M49 172v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12m16-12v12M49 172h304"
        stroke="#81977E"
        strokeWidth="1.4"
      />
      <g fill="#567A5F">
        <ellipse cx="39" cy="145" rx="20" ry="28" />
        <ellipse cx="360" cy="149" rx="23" ry="32" />
        <ellipse cx="376" cy="159" rx="14" ry="24" />
      </g>
      <path
        d="M39 148v36m321-36v36m16-27v27"
        stroke="#395D47"
        strokeWidth="2"
      />
      <g stroke="#E6ECE0" strokeWidth="1.5" strokeLinecap="round">
        <path d="M29 218h32m18 12h45m21-15h35m25 15h42m24-12h20m32 12h42" />
      </g>
      <path
        d="m122 54 4-3 4 3m6-6 4-3 4 3"
        stroke="#93A58D"
        strokeLinecap="round"
      />
    </svg>
  );
}

export function StoryMark({ className }: ArtProps) {
  return (
    <svg
      className={className}
      viewBox="0 0 240 180"
      fill="none"
      role="img"
      aria-label="一颗等待生长的新故事种子"
    >
      <circle cx="120" cy="88" r="57" fill="#EAF0E1" />
      <circle cx="120" cy="88" r="74" stroke="#D5DECC" strokeDasharray="2 6" />
      <ellipse
        cx="120"
        cy="93"
        rx="87"
        ry="27"
        transform="rotate(-28 120 93)"
        stroke="#A8BE9E"
      />
      <path
        d="m120 42 12 32 33 13-33 12-12 34-12-34-33-12 33-13Z"
        fill="#42694F"
      />
      <path d="m120 61 7 20 20 6-20 7-7 20-7-20-19-7 19-6Z" fill="#DCE5C6" />
      <circle cx="191" cy="50" r="4" fill="#9DAE81" />
      <circle cx="52" cy="130" r="3" fill="#BAC8A5" />
      <path
        d="m53 43 2 6 6 2-6 2-2 6-2-6-6-2 6-2Zm130 89 1.5 4.5 4.5 1.5-4.5 1.5-1.5 4.5-1.5-4.5-4.5-1.5 4.5-1.5Z"
        fill="#A6B994"
      />
      <path d="M100 158h40" stroke="#D5DECC" strokeLinecap="round" />
    </svg>
  );
}
