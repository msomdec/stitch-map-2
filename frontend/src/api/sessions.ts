import { get, post, del } from './client';
import type { DashboardData, SessionViewData, WorkSession } from '../types';

export const sessionsApi = {
  dashboard: () => get<DashboardData>('/dashboard'),
  loadMoreCompleted: (offset: number) =>
    get<{
      sessions: WorkSession[];
      patternNames: Record<string, string>;
      totalCompleted: number;
    }>(`/dashboard/completed?offset=${offset}`),
  start: (patternId: number) =>
    post<{ session: WorkSession }>(`/patterns/${patternId}/sessions`),
  get: (id: number) => get<SessionViewData>(`/sessions/${id}`),
  next: (id: number) => post<SessionViewData>(`/sessions/${id}/next`),
  prev: (id: number) => post<SessionViewData>(`/sessions/${id}/prev`),
  pause: (id: number) => post<SessionViewData>(`/sessions/${id}/pause`),
  resume: (id: number) => post<SessionViewData>(`/sessions/${id}/resume`),
  abandon: (id: number) => del<void>(`/sessions/${id}`),
};
