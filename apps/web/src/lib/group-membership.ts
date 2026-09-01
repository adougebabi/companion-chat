import type { ActorGroupSnapshot } from "./actor-groups";

export type { ActorGroupSnapshot } from "./actor-groups";

export function planDefaultGroupMembership(groups: ActorGroupSnapshot[], actorIds: string[]) {
  const defaultGroup = groups.find((group) => group.name === "默认") ?? null;
  const assignedActorIds = new Set(groups.flatMap((group) => group.actor_ids));
  return {
    defaultGroup,
    ungroupedActorIds: actorIds.filter((actorId) => !assignedActorIds.has(actorId)),
  };
}
