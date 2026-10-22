/** UI projection only. Browser wire DTOs remain owned by the generated client. */
export interface ProfileSnapshot {
  id: string;
  name: string;
  avatar?: string;
  bio: string;
  status: string;
  readOnly: boolean;
  mode: 'real' | 'mock';
  foundationRevision: number | null;
  runtimeRevision: number | null;
  activeProfileId: string | null;
  switchTargetIds?: string[];
  historyHasMore?: boolean;
  profiles: Array<Record<string, unknown> & { id: string }>;
  groups: Array<{ key: string; title: string; value: unknown }>;
  history: Array<Record<string, unknown>>;
  revisions: Array<Record<string, unknown>>;
}
export interface ProfileSwitchCommand {
  targetPersonaId: string;
  expectedRevision: number;
  expectedRuntimeRevision: number;
  reason: string;
  confirm: true;
  idempotencyKey: string;
}
export interface ProfileProvider {
  read(characterId: string): ProfileSnapshot | Promise<ProfileSnapshot>;
  switchPersona?(characterId: string, command: ProfileSwitchCommand, options?: { signal?: AbortSignal }): Promise<unknown>;
  editBio?(characterId: string, bio: string, expectedRevision?: number | null): Promise<void>;
}
