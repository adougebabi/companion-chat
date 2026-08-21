import {encodePath, getActivityPage, requestJson} from './client';
import type {Activity, ActivityPage} from '../types';

export interface ActivityPageOptions {
  personaId?: string | null;
  cursor?: string | null;
  limit?: number;
  visibility?: 'visible' | 'hidden';
  signal?: AbortSignal;
}

export function listActivityPage(options: ActivityPageOptions = {}): Promise<ActivityPage> {
  return getActivityPage({...options, limit: options.limit ?? 20});
}

export async function markActivitiesRead(signal?: AbortSignal): Promise<void> {
  await requestJson('/api/companion/activities/read', {method: 'POST', signal});
}

export async function setActivityLike(activityId: string, liked: boolean, signal?: AbortSignal): Promise<Activity | null> {
  const payload = await requestJson(`/api/companion/activities/${encodePath(activityId)}/like`, {
    method: 'PUT',
    signal,
    body: JSON.stringify({liked})
  });
  return payload && typeof payload === 'object' ? payload as Activity : null;
}

export async function setActivityHidden(activityId: string, hidden: boolean, signal?: AbortSignal): Promise<Activity | null> {
  const payload = await requestJson(`/api/companion/activities/${encodePath(activityId)}/hide`, {
    method: 'PUT',
    signal,
    body: JSON.stringify({hidden})
  });
  return payload && typeof payload === 'object' ? payload as Activity : null;
}

export async function commentActivity(activityId: string, content: string, signal?: AbortSignal): Promise<unknown> {
  return requestJson(`/api/companion/activities/${encodePath(activityId)}/comments`, {
    method: 'POST',
    signal,
    body: JSON.stringify({content})
  });
}

