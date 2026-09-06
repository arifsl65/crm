/**
 * Search API - Global search across entities.
 */

import { authFetch } from './core';
import type { GlobalSearchResults } from './types';

export async function globalSearch(query: string, params?: {
  limit?: number;
  types?: ('clients' | 'documents' | 'services' | 'emails')[];
}): Promise<GlobalSearchResults> {
  const searchParams = new URLSearchParams();
  searchParams.set('q', query);
  if (params?.limit) searchParams.set('limit', String(params.limit));
  if (params?.types) searchParams.set('types', params.types.join(','));

  const res = await authFetch(`/api/v1/search?${searchParams}`);
  if (!res.ok) throw new Error('Failed to perform search');
  return res.json();
}
