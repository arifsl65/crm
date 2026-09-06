/**
 * Auth API - Authentication, 2FA, password management, sessions.
 */

import { authFetch, API_URL, AUTH_EXPIRED_EVENT } from './core';
import type { AuthResponse, User, Session, TwoFactorSetupResponse } from './types';

// ============================================================================
// Login & Registration
// ============================================================================

/**
 * Login with email and password.
 * Rate limited: 5 attempts per IP+email, 15min lockout on failure.
 */
export async function login(
  email: string,
  password: string,
  tenantDomain?: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ email, password, tenant_domain: tenantDomain }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Login failed');
  }
  return res.json();
}

/**
 * Register a new user account.
 */
export async function register(
  email: string,
  password: string,
  name: string,
  tenantId: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ email, password, name, tenant_id: tenantId }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Registration failed');
  }
  return res.json();
}

/**
 * Logout the user by calling the backend logout endpoint.
 */
export async function logout(): Promise<void> {
  try {
    await authFetch('/api/v1/auth/logout', { method: 'POST' });
  } catch {
    // Continue with client-side cleanup even if backend call fails
  }
  if (typeof window !== 'undefined') {
    sessionStorage.removeItem('user');
    window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
  }
}

// ============================================================================
// Magic Link
// ============================================================================

/**
 * Request a magic link for passwordless login.
 */
export async function sendMagicLink(email: string): Promise<{ message: string }> {
  const res = await fetch(`${API_URL}/api/v1/auth/magic-link`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to send magic link');
  }
  return res.json();
}

/**
 * Verify a magic link token and login.
 */
export async function verifyMagicLink(token: string): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/magic-link?token=${encodeURIComponent(token)}`, {
    method: 'GET',
    credentials: 'include',
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Invalid or expired magic link');
  }
  return res.json();
}

// ============================================================================
// Invite
// ============================================================================

/**
 * Accept a team invitation and set password.
 */
export async function acceptInvite(
  token: string,
  password: string,
  name?: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/invite-accept`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ token, password, name }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Invalid or expired invitation');
  }
  return res.json();
}

// ============================================================================
// Password Reset
// ============================================================================

/**
 * Request a password reset email.
 */
export async function forgotPassword(email: string): Promise<{ message: string }> {
  const res = await fetch(`${API_URL}/api/v1/auth/reset-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to send reset email');
  }
  return res.json();
}

/**
 * Reset password using a reset token.
 */
export async function resetPassword(
  token: string,
  newPassword: string
): Promise<{ message: string }> {
  const res = await fetch(`${API_URL}/api/v1/auth/reset-password/confirm`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token, new_password: newPassword }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to reset password');
  }
  return res.json();
}

/**
 * Change the current user's password.
 */
export async function changePassword(
  currentPassword: string,
  newPassword: string
): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/auth/password', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to change password');
  }
  return res.json();
}

// ============================================================================
// User Profile
// ============================================================================

/**
 * Get current authenticated user's profile.
 */
export async function getMe(): Promise<User> {
  const res = await authFetch('/api/v1/auth/me');
  if (!res.ok) throw new Error('Failed to fetch user profile');
  return res.json();
}

/**
 * Update current user's profile.
 */
export async function updateMe(updates: {
  name?: string;
  phone?: string;
  avatar_url?: string;
  settings?: Record<string, unknown>;
}): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/auth/me', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to update profile');
  }
  return res.json();
}

// ============================================================================
// Sessions
// ============================================================================

/**
 * Get all active sessions for the current user.
 */
export async function getSessions(): Promise<{ sessions: Session[] }> {
  const res = await authFetch('/api/v1/auth/sessions');
  if (!res.ok) throw new Error('Failed to fetch sessions');
  return res.json();
}

// ============================================================================
// Two-Factor Authentication
// ============================================================================

/**
 * Initialize 2FA setup by generating a TOTP secret.
 */
export async function setup2FA(): Promise<TwoFactorSetupResponse> {
  const res = await authFetch('/api/v1/auth/2fa/setup', { method: 'POST' });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to setup 2FA');
  }
  return res.json();
}

/**
 * Verify TOTP code to complete 2FA setup.
 */
export async function verify2FA(code: string): Promise<{ message: string; backup_codes?: string[] }> {
  const res = await authFetch('/api/v1/auth/2fa/verify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Invalid verification code');
  }
  return res.json();
}

/**
 * Disable 2FA for the current user.
 */
export async function disable2FA(password: string): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/auth/2fa', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to disable 2FA');
  }
  return res.json();
}

/**
 * Generate new backup codes for 2FA recovery.
 */
export async function generateBackupCodes(): Promise<{ backup_codes: string[] }> {
  const res = await authFetch('/api/v1/auth/2fa/backup-codes', { method: 'POST' });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to generate backup codes');
  }
  return res.json();
}

/**
 * Login using a backup code instead of TOTP.
 */
export async function verifyBackupCode(
  email: string,
  password: string,
  backupCode: string
): Promise<AuthResponse> {
  const res = await fetch(`${API_URL}/api/v1/auth/2fa/backup-codes/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ email, password, backup_code: backupCode }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Invalid backup code');
  }
  return res.json();
}

// ============================================================================
// Token Management
// ============================================================================

/**
 * Revoke all tokens in a refresh token family.
 */
export async function revokeTokenFamily(familyId: string): Promise<{ message: string }> {
  const res = await authFetch('/api/v1/auth/refresh/revoke-family', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ family_id: familyId }),
  });
  if (!res.ok) {
    const error = await res.json();
    throw new Error(error.message || 'Failed to revoke token family');
  }
  return res.json();
}
