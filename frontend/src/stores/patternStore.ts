import { create } from 'zustand';
import type { Pattern, PatternImage } from '../types';
import { patternsApi, type PatternCreateRequest } from '../api/patterns';
import { ApiClientError } from '../api/client';

interface PatternState {
  patterns: Pattern[];
  currentPattern: Pattern | null;
  currentPatternText: string;
  currentStitchCount: number;
  currentGroupImages: Record<string, PatternImage[]>;
  loading: boolean;
  error: string | null;

  fetchPatterns: () => Promise<void>;
  fetchPattern: (id: number) => Promise<void>;
  createPattern: (data: PatternCreateRequest) => Promise<Pattern>;
  updatePattern: (id: number, data: PatternCreateRequest) => Promise<Pattern>;
  deletePattern: (id: number) => Promise<void>;
  duplicatePattern: (id: number) => Promise<void>;
  previewPattern: (data: PatternCreateRequest) => Promise<{ text: string; stitchCount: number }>;
  clearError: () => void;
  clearCurrent: () => void;
}

export const usePatternStore = create<PatternState>((set) => ({
  patterns: [],
  currentPattern: null,
  currentPatternText: '',
  currentStitchCount: 0,
  currentGroupImages: {},
  loading: false,
  error: null,

  fetchPatterns: async () => {
    set({ loading: true, error: null });
    try {
      const res = await patternsApi.list();
      set({ patterns: res.patterns, loading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to load patterns';
      set({ error: message, loading: false });
    }
  },

  fetchPattern: async (id) => {
    set({ loading: true, error: null });
    try {
      const res = await patternsApi.get(id);
      set({
        currentPattern: res.pattern,
        currentPatternText: res.patternText,
        currentStitchCount: res.stitchCount,
        currentGroupImages: res.groupImages,
        loading: false,
      });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to load pattern';
      set({ error: message, loading: false });
    }
  },

  createPattern: async (data) => {
    set({ loading: true, error: null });
    try {
      const res = await patternsApi.create(data);
      set({ loading: false });
      return res.pattern;
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to create pattern';
      set({ error: message, loading: false });
      throw err;
    }
  },

  updatePattern: async (id, data) => {
    set({ loading: true, error: null });
    try {
      const res = await patternsApi.update(id, data);
      set({ loading: false });
      return res.pattern;
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to update pattern';
      set({ error: message, loading: false });
      throw err;
    }
  },

  deletePattern: async (id) => {
    set({ error: null });
    try {
      await patternsApi.delete(id);
      set((state) => ({ patterns: state.patterns.filter((p) => p.id !== id) }));
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to delete pattern';
      set({ error: message });
      throw err;
    }
  },

  duplicatePattern: async (id) => {
    set({ error: null });
    try {
      await patternsApi.duplicate(id);
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to duplicate pattern';
      set({ error: message });
      throw err;
    }
  },

  previewPattern: async (data) => {
    try {
      return await patternsApi.preview(data);
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to preview pattern';
      set({ error: message });
      throw err;
    }
  },

  clearError: () => set({ error: null }),
  clearCurrent: () => set({ currentPattern: null, currentPatternText: '', currentStitchCount: 0, currentGroupImages: {} }),
}));
