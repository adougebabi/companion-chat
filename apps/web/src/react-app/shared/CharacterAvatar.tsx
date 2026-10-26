import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
export default function CharacterAvatar({
  name,
  initial,
  color,
  src,
  className,
}: {
  name?: string;
  initial?: string;
  color?: string;
  src?: string | { url?: string };
  className?: string;
}) {
  const url = typeof src === "string" ? src : src?.url;
  return (
    <Avatar
      className={className ? `avatar ${className}` : "avatar"}
      style={{ background: color }}
    >
      {url && <AvatarImage src={url} alt={`${name || "角色"}的头像`} />}
      <AvatarFallback style={{ background: color }}>
        {initial || [...(name || "光")].at(-1)}
      </AvatarFallback>
    </Avatar>
  );
}
