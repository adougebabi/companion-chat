import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { avatarInitial, avatarSource } from "@/lib/avatar-presentation.mjs";
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
  const url = avatarSource(src);
  return (
    <Avatar
      className={className ? `avatar ${className}` : "avatar"}
      style={{ background: color }}
    >
      {url && <AvatarImage src={url} alt={`${name || "角色"}的头像`} />}
      <AvatarFallback style={{ background: color }}>
        {avatarInitial(name, initial)}
      </AvatarFallback>
    </Avatar>
  );
}
