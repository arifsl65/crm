/**
 * Core API utilities - foundational functions used by all other API modules.
 * Includes authentication helpers, token management, and base fetch wrapper.
 */

export const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

// Token refresh state to prevent multiple simultaneous refresh attempts
let isRefreshing = false;
let refreshPromise: Promise<boolean> | null = null;

/**
 * Attempt to refresh the access token using the httpOnly refresh_token cookie.
 * The backend sets new httpOnly cookies on success - no localStorage needed.
 */
export async function tryRefreshToken(): Promise<boolean> {
  // Fix #7: Removed localStorage access_token check
  // Authentication is now handled solely via httpOnly cookies
  // Skip refresh if no prior session exists (incognito/fresh browser)
  // We'll let the refresh attempt fail naturally if no cookies exist
  
  // If already refreshing, wait for the existing refresh to complete
  if (isRefreshing && refreshPromise) {
    return refreshPromise;
  }

  isRefreshing = true;
  refreshPromise = (async () => {
    try {
      const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include', // Send httpOnly cookies
      });

      if (!res.ok) {
        return false;
      }

      // Backend sets new httpOnly cookies automatically via Set-Cookie header
      // No need to store tokens client-side
      return true;
    } catch {
      return false;
    } finally {
      isRefreshing = false;
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

// Custom event for auth expiry - components can listen and handle navigation
// Fix #36: Use custom event instead of window.location.href for SPA-friendly redirect
export const AUTH_EXPIRED_EVENT = 'auth-expired';

/**
 * Clear auth state and trigger redirect to login.
 * Note: httpOnly cookies are cleared by the backend on logout via Set-Cookie.
 * We clear client-side user data here.
 */
export function clearAuthAndRedirect(): void {
  if (typeof window !== 'undefined') {
    // Fix #7: Removed access_token removal from localStorage (no longer stored there)
    // Clear user data from localStorage
    localStorage.removeItem('user');

    // Dispatch custom event for React components to handle
    window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
  }
}

/**
 * Helper to make authenticated requests with automatic token refresh.
 * Uses httpOnly cookies for authentication (credentials: 'include').
 * No localStorage token handling needed - cookies are sent automatically.
 */
export async function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const makeRequest = async () => {
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    };

    return fetch(`${API_URL}${url}`, {
      ...options,
      headers,
      credentials: 'include', // Send httpOnly cookies automatically
    });
  };

  let res = await makeRequest();

  // If 401 Unauthorized, try to refresh the token
  if (res.status === 401) {
    const refreshed = await tryRefreshToken();

    if (refreshed) {
      // Retry the original request with the new token
      res = await makeRequest();
    } else {
      // Refresh failed, redirect to login
      clearAuthAndRedirect();
    }
  }

  return res;
}
