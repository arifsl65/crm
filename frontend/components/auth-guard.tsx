'use client';

import { useEffect, useState } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { isAuthenticated, getUser, clearAuth, refreshToken, getAccessToken, saveAuth } from '@/lib/auth';

interface AuthGuardProps {
  children: React.ReactNode;
}

// Public routes that don't require authentication
const publicRoutes = ['/login', '/register', '/forgot-password', '/reset-password'];

export function AuthGuard({ children }: AuthGuardProps) {
  const router = useRouter();
  const pathname = usePathname();
  const [isLoading, setIsLoading] = useState(true);
  const [isAuthed, setIsAuthed] = useState(false);

  useEffect(() => {
    const checkAuth = async () => {
      const isPublicRoute = publicRoutes.some(route => pathname.startsWith(route));

      if (isPublicRoute) {
        // Redirect authenticated users away from public routes
        if (isAuthenticated()) {
          router.replace('/dashboard');
          return;
        }
        setIsLoading(false);
        setIsAuthed(false);
        return;
      }

      // Protected route - check authentication
      if (!isAuthenticated()) {
        clearAuth();
        router.replace('/login');
        return;
      }

      // Try to validate token by checking if user exists
      const user = getUser();
      if (!user) {
        clearAuth();
        router.replace('/login');
        return;
      }

      setIsAuthed(true);
      setIsLoading(false);
    };

    checkAuth();
  }, [pathname, router]);

  // Show loading state while checking auth
  if (isLoading && !publicRoutes.some(route => pathname.startsWith(route))) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-slate-900">
        <div className="flex flex-col items-center space-y-4">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
          <p className="text-gray-600 dark:text-gray-400">Loading...</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}

// Hook to get current user with type safety
export function useAuth() {
  const [user, setUser] = useState(getUser());
  const router = useRouter();

  const logout = async () => {
    try {
      const token = getAccessToken();
      if (token) {
        // Call logout endpoint to invalidate token on server
        const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
        await fetch(`${API_URL}/api/v1/auth/logout`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        });
      }
    } catch (error) {
      console.error('Logout error:', error);
    } finally {
      clearAuth();
      setUser(null);
      router.replace('/login');
    }
  };

  const refreshSession = async () => {
    try {
      const storedRefreshToken = typeof window !== 'undefined'
        ? localStorage.getItem('refresh_token')
        : null;

      if (!storedRefreshToken) {
        throw new Error('No refresh token');
      }

      const auth = await refreshToken(storedRefreshToken);
      saveAuth(auth);
      setUser(auth.user);
      return true;
    } catch (error) {
      console.error('Refresh error:', error);
      clearAuth();
      setUser(null);
      router.replace('/login');
      return false;
    }
  };

  return { user, logout, refreshSession, isAuthenticated: !!user };
}
