const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  tenant_id: string;
  two_factor_enabled?: boolean;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  expires_at: string;
  user: User;
}

export interface LoginCredentials {
  email: string;
  password: string;
  tenant_domain?: string; // Required when email exists in multiple tenants
}

export interface RegisterData {
  email: string;
  password: string;
  name: string;
  tenant_id: string;
}

export interface InviteAcceptData {
  token: string;
  password: string;
  name?: string; // Optional: update name if provided
}

export async function login(credentials: LoginCredentials): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(credentials),
  });

  if (!res.ok) {
    const error = await res.json();
    // Throw stringified error so we can parse the full structure
    throw new Error(JSON.stringify(error));
  }

  return res.json();
}

export async function register(data: RegisterData): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });

  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Registration failed');
  }

  return res.json();
}

/**
 * Accept an invitation and set password.
 * This is the proper way to register new users in a multi-tenant system.
 * Users receive an invite email with a token, then use this endpoint to activate.
 */
export async function acceptInvite(data: InviteAcceptData): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/invite-accept`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });

  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to accept invitation');
  }

  return res.json();
}

export async function refreshToken(refresh_token: string): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token }),
  });

  if (!res.ok) {
    throw new Error('Token refresh failed');
  }

  return res.json();
}

export function saveAuth(auth: AuthResponse): void {
  if (typeof window !== 'undefined') {
    localStorage.setItem('access_token', auth.access_token);
    localStorage.setItem('refresh_token', auth.refresh_token);
    // Only save user if it exists to prevent storing "undefined" string
    if (auth.user) {
      localStorage.setItem('user', JSON.stringify(auth.user));
    }
  }
}

export function getAccessToken(): string | null {
  if (typeof window !== 'undefined') {
    return localStorage.getItem('access_token');
  }
  return null;
}

export function getUser(): User | null {
  if (typeof window !== 'undefined') {
    const user = localStorage.getItem('user');
    if (!user || user === 'undefined') {
      return null;
    }
    try {
      return JSON.parse(user);
    } catch {
      // Invalid JSON in localStorage, clear it
      localStorage.removeItem('user');
      return null;
    }
  }
  return null;
}

export function clearAuth(): void {
  if (typeof window !== 'undefined') {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    localStorage.removeItem('user');
  }
}

export function isAuthenticated(): boolean {
  return !!getAccessToken();
}

/**
 * Validates the current access token with the server.
 * Fix #30: Prevents stale/expired tokens from passing as authenticated.
 * Returns the user if valid, null if invalid (triggers refresh or logout).
 */
export async function validateToken(): Promise<User | null> {
  const token = getAccessToken();
  if (!token) {
    return null;
  }

  try {
    const res = await fetch(`${API_URL}/api/v1/auth/me`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
    });

    if (!res.ok) {
      return null;
    }

    const data = await res.json();
    // Backend returns user data at root level, not wrapped in { user: ... }
    if (data && data.id && data.email) {
      return data as User;
    }
    return null;
  } catch {
    return null;
  }
}

/**
 * Detects the current tenant domain from the browser hostname.
 * This helps pre-fill tenant_domain on multi-tenant deployments.
 *
 * Examples:
 * - acme.crm.example.com → acme.crm.example.com
 * - crm.example.com → null (main app, no tenant subdomain)
 * - localhost:3000 → null (development)
 */
export function detectTenantDomain(): string | null {
  if (typeof window === 'undefined') {
    return null;
  }

  const hostname = window.location.hostname;

  // Skip localhost and IP addresses
  if (hostname === 'localhost' || /^[\d.]+$/.test(hostname)) {
    return null;
  }

  // Return the full hostname as the tenant domain
  // The backend will match against tenants.domain or tenants.custom_domain
  return hostname;
}

/**
 * Login error structure returned by the API
 */
export interface LoginError {
  error: string;
  message: string;
}

/**
 * Checks if the error indicates tenant selection is required
 */
export function isTenantRequiredError(error: LoginError): boolean {
  return error.error === 'tenant_required';
}
