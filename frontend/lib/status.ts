/**
 * Status badge utilities for consistent styling across the application.
 * Consolidates duplicated status color logic from various components.
 */

// Badge color class constants
export const BADGE_COLORS = {
  success: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  warning: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  error: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  purple: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  neutral: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
} as const;

export type BadgeColor = keyof typeof BADGE_COLORS;

// Status to color mappings by entity type
const STATUS_COLORS: Record<string, BadgeColor> = {
  // Success states
  active: 'success',
  approved: 'success',
  completed: 'success',

  // Warning states
  inactive: 'warning',
  pending_review: 'warning',
  in_progress: 'warning',
  review: 'warning',
  uploaded: 'warning',

  // Error states
  archived: 'error',
  rejected: 'error',
  cancelled: 'error',

  // Info states
  requested: 'info',
  not_started: 'info',

  // Special states
  waiting: 'purple',
};

/**
 * Get the badge CSS class for a given status string.
 * Works for client, service, and document statuses.
 */
export function getStatusBadgeClass(status: string): string {
  const color = STATUS_COLORS[status] || 'neutral';
  return BADGE_COLORS[color];
}

/**
 * Format a status string for display (replaces underscores with spaces).
 */
export function formatStatus(status: string): string {
  return status.replace(/_/g, ' ');
}

/**
 * Get the semantic color key for a status (useful for other UI elements).
 */
export function getStatusColor(status: string): BadgeColor {
  return STATUS_COLORS[status] || 'neutral';
}
