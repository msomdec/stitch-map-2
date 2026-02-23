import { create } from 'zustand';
import type { Stitch } from '../types';
import { stitchesApi, type StitchCreateRequest } from '../api/stitches';
import { ApiClientError } from '../api/client';

interface StitchState {
  predefined: Stitch[];
  custom: Stitch[];
  allStitches: Stitch[];
  loading: boolean;
  error: string | null;

  fetchStitches: (params?: { category?: string; search?: string }) => Promise<void>;
  fetchAllStitches: () => Promise<void>;
  createCustom: (data: StitchCreateRequest) => Promise<void>;
  updateCustom: (id: number, data: StitchCreateRequest) => Promise<void>;
  deleteCustom: (id: number) => Promise<void>;
  clearError: () => void;
}

export const useStitchStore = create<StitchState>((set) => ({
  predefined: [],
  custom: [],
  allStitches: [],
  loading: false,
  error: null,

  fetchStitches: async (params) => {
    set({ loading: true, error: null });
    try {
      const res = await stitchesApi.list(params);
      set({ predefined: res.predefined, custom: res.custom, loading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to load stitches';
      set({ error: message, loading: false });
    }
  },

  fetchAllStitches: async () => {
    set({ loading: true, error: null });
    try {
      const res = await stitchesApi.listAll();
      set({ allStitches: res.stitches, loading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to load stitches';
      set({ error: message, loading: false });
    }
  },

  createCustom: async (data) => {
    set({ loading: true, error: null });
    try {
      await stitchesApi.create(data);
      set({ loading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to create stitch';
      set({ error: message, loading: false });
      throw err;
    }
  },

  updateCustom: async (id, data) => {
    set({ loading: true, error: null });
    try {
      await stitchesApi.update(id, data);
      set({ loading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to update stitch';
      set({ error: message, loading: false });
      throw err;
    }
  },

  deleteCustom: async (id) => {
    set({ error: null });
    try {
      await stitchesApi.delete(id);
      set((state) => ({ custom: state.custom.filter((s) => s.id !== id) }));
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Failed to delete stitch';
      set({ error: message });
      throw err;
    }
  },

  clearError: () => set({ error: null }),
}));
