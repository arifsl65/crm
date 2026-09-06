/**
 * API Module - Re-exports from the api/ directory.
 *
 * This file maintains backwards compatibility for existing imports.
 * All functionality is now organized in the api/ subdirectory.
 *
 * Usage remains unchanged:
 *   import { getClients, Client, authFetch } from '@/lib/api';
 */
export * from './api/index';
