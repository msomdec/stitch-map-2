import { get, post, put, del } from './client';
import type { Pattern, PatternImage } from '../types';

export interface PatternCreateRequest {
  name: string;
  description: string;
  patternType: string;
  hookSize: string;
  yarnWeight: string;
  difficulty: string;
  instructionGroups: {
    label: string;
    repeatCount: number;
    expectedCount: number | null;
    notes: string;
    stitchEntries: {
      stitchId: number;
      count: number;
      intoStitch: string;
      repeatCount: number;
    }[];
  }[];
}

export const patternsApi = {
  list: () => get<{ patterns: Pattern[] }>('/patterns'),
  get: (id: number) =>
    get<{
      pattern: Pattern;
      stitchCount: number;
      patternText: string;
      groupImages: Record<string, PatternImage[]>;
    }>(`/patterns/${id}`),
  create: (data: PatternCreateRequest) => post<{ pattern: Pattern }>('/patterns', data),
  update: (id: number, data: PatternCreateRequest) =>
    put<{ pattern: Pattern }>(`/patterns/${id}`, data),
  delete: (id: number) => del<void>(`/patterns/${id}`),
  duplicate: (id: number) => post<{ pattern: Pattern }>(`/patterns/${id}/duplicate`),
  preview: (data: PatternCreateRequest) =>
    post<{ text: string; stitchCount: number }>('/patterns/preview', data),
};
