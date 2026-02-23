import { get, post } from './client';
import type { User } from '../types';

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  displayName: string;
  password: string;
  confirmPassword: string;
}

export const authApi = {
  login: (data: LoginRequest) => post<{ user: User }>('/auth/login', data),
  register: (data: RegisterRequest) => post<{ user: User }>('/auth/register', data),
  logout: () => post<void>('/auth/logout'),
  me: () => get<{ user: User }>('/auth/me'),
};
