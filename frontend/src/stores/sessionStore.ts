import { create } from 'zustand';
import type { DashboardData, SessionViewData, WorkSession } from '../types';
import { sessionsApi } from '../api/sessions';
import { ApiClientError } from '../api/client';

interface SessionState {
  // Dashboard state
  dashboard: DashboardData | null;
  dashboardLoading: boolean;

  // Current session view state
  sessionView: SessionViewData | null;
  sessionLoading: boolean;

  error: string | null;

  fetchDashboard: () => Promise<void>;
  loadMoreCompleted: (offset: number) => Promise<void>;
  startSession: (patternId: number) => Promise<WorkSession>;
  fetchSession: (id: number) => Promise<void>;
  nextStitch: (id: number) => Promise<void>;
  prevStitch: (id: number) => Promise<void>;
  pauseSession: (id: number) => Promise<void>;
  resumeSession: (id: number) => Promise<void>;
  abandonSession: (id: number) => Promise<void>;
  clearError: () => void;
  clearSession: () => void;
}

export const useSessionStore = create<SessionState>((set) => ({
  dashboard: null,
  dashboardLoading: false,
  sessionView: null,
  sessionLoading: false,
  error: null,

  fetchDashboard: async () => {
    set({ dashboardLoading: true, error: null });
    try {
      const data = await sessionsApi.dashboard();
      set({ dashboard: data, dashboardLoading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to load dashboard';
      set({ error: message, dashboardLoading: false });
    }
  },

  loadMoreCompleted: async (offset) => {
    try {
      const data = await sessionsApi.loadMoreCompleted(offset);
      set((state) => {
        if (!state.dashboard) return state;
        return {
          dashboard: {
            ...state.dashboard,
            completedSessions: [...state.dashboard.completedSessions, ...data.sessions],
            patternNames: { ...state.dashboard.patternNames, ...data.patternNames },
            totalCompleted: data.totalCompleted,
          },
        };
      });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to load more sessions';
      set({ error: message });
    }
  },

  startSession: async (patternId) => {
    set({ error: null });
    try {
      const res = await sessionsApi.start(patternId);
      return res.session;
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to start session';
      set({ error: message });
      throw err;
    }
  },

  fetchSession: async (id) => {
    set({ sessionLoading: true, error: null });
    try {
      const data = await sessionsApi.get(id);
      set({ sessionView: data, sessionLoading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to load session';
      set({ error: message, sessionLoading: false });
    }
  },

  nextStitch: async (id) => {
    try {
      const data = await sessionsApi.next(id);
      set({ sessionView: data });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to advance';
      set({ error: message });
    }
  },

  prevStitch: async (id) => {
    try {
      const data = await sessionsApi.prev(id);
      set({ sessionView: data });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to retreat';
      set({ error: message });
    }
  },

  pauseSession: async (id) => {
    try {
      const data = await sessionsApi.pause(id);
      set({ sessionView: data });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to pause';
      set({ error: message });
    }
  },

  resumeSession: async (id) => {
    try {
      const data = await sessionsApi.resume(id);
      set({ sessionView: data });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to resume';
      set({ error: message });
    }
  },

  abandonSession: async (id) => {
    set({ error: null });
    try {
      await sessionsApi.abandon(id);
      set({ sessionView: null });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to abandon';
      set({ error: message });
      throw err;
    }
  },

  clearError: () => set({ error: null }),
  clearSession: () => set({ sessionView: null }),
}));
