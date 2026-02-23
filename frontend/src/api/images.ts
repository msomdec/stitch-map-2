import { del, uploadFile } from './client';
import type { PatternImage } from '../types';

export const imagesApi = {
  upload: (patternId: number, groupIndex: number, file: File) =>
    uploadFile<{ image: PatternImage }>(
      `/patterns/${patternId}/groups/${groupIndex}/images`,
      file,
    ),
  delete: (id: number) => del<void>(`/images/${id}`),
  url: (id: number) => `/api/images/${id}`,
};
