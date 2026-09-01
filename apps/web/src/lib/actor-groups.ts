export type ActorGroupSnapshot = {
  id: string;
  name: string;
  actor_ids: string[];
  owner_actor_id?: string;
  created_at?: string;
};

type ActorGroupPayload = Partial<ActorGroupSnapshot> & { members?: unknown };

export function normalizeActorGroup(value: ActorGroupPayload): ActorGroupSnapshot | null {
  if (!value || typeof value.id !== "string" || typeof value.name !== "string") return null;
  const actorIds = Array.isArray(value.actor_ids)
    ? value.actor_ids
    : Array.isArray(value.members)
      ? value.members
      : [];
  return {
    id: value.id,
    name: value.name,
    actor_ids: actorIds.filter((actorId): actorId is string => typeof actorId === "string"),
    ...(typeof value.owner_actor_id === "string" ? { owner_actor_id: value.owner_actor_id } : {}),
    ...(typeof value.created_at === "string" ? { created_at: value.created_at } : {}),
  };
}

export function normalizeActorGroups(values: unknown): ActorGroupSnapshot[] {
  if (!Array.isArray(values)) return [];
  return values.map((value) => normalizeActorGroup(value as ActorGroupPayload)).filter((value): value is ActorGroupSnapshot => Boolean(value));
}
