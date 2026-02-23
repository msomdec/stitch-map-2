import { get, post, put, del } from './client';
import type { Stitch } from '../types';

export interface StitchCreateRequest {
  abbreviation: string;
  name: string;
  description: string;
  category: string;
}

export const stitchesApi = {
  list: (params?: { category?: string; search?: string }) => {
    const searchParams = new URLSearchParams();
    if (params?.category) searchParams.set('category', params.category);
    if (params?.search) searchParams.set('search', params.search);
    const query = searchParams.toString();
    return get<{ predefined: Stitch[]; custom: Stitch[] }>(
      `/stitches${query ? '?' + query : ''}`,
    );
  },
  listAll: () => get<{ stitches: Stitch[] }>('/stitches/all'),
  create: (data: StitchCreateRequest) => post<{ stitch: Stitch }>('/stitches', data),
  update: (id: number, data: StitchCreateRequest) =>
    put<{ stitch: Stitch }>(`/stitches/${id}`, data),
  delete: (id: number) => del<void>(`/stitches/${id}`),
};
