import { create } from 'zustand';
import type { User } from '../types';
import { authApi, type LoginRequest, type RegisterRequest } from '../api/auth';
import { ApiClientError } from '../api/client';

interface AuthState {
  user: User | null;
  loading: boolean;
  error: string | null;
  initialized: boolean;

  login: (data: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => Promise<void>;
  fetchMe: () => Promise<void>;
  clearError: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: false,
  error: null,
  initialized: false,

  login: async (data) => {
    set({ loading: true, error: null });
    try {
      const res = await authApi.login(data);
      set({ user: res.user, loading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Login failed';
      set({ error: message, loading: false });
      throw err;
    }
  },

  register: async (data) => {
    set({ loading: true, error: null });
    try {
      await authApi.register(data);
      set({ loading: false });
    } catch (err) {
      const message = err instanceof ApiClientError ? err.message : 'Registration failed';
      set({ error: message, loading: false });
      throw err;
    }
  },

  logout: async () => {
    try {
      await authApi.logout();
    } finally {
      set({ user: null });
    }
  },

  fetchMe: async () => {
    try {
      const res = await authApi.me();
      set({ user: res.user, initialized: true });
    } catch {
      set({ user: null, initialized: true });
    }
  },

  clearError: () => set({ error: null }),
}));
