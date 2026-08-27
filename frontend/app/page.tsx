'use client';

import { useState, useEffect } from 'react';

interface HealthStatus {
  status: string;
  db?: string;
  redis?: string;
}

export default function Home() {
  const [theme, setTheme] = useState<'light' | 'dark'>('light');
  const [backendHealth, setBackendHealth] = useState<HealthStatus | null>(null);
  const [aiHealth, setAiHealth] = useState<HealthStatus | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Check system theme preference
    if (typeof window !== 'undefined') {
      const isDark = document.documentElement.classList.contains('dark');
      setTheme(isDark ? 'dark' : 'light');
    }

    // Check backend health
    const checkHealth = async () => {
      try {
        const [backendRes, aiRes] = await Promise.allSettled([
          fetch(`${process.env.NEXT_PUBLIC_API_URL}/ready`),
          fetch(`${process.env.NEXT_PUBLIC_AI_API_URL}/health`),
        ]);

        if (backendRes.status === 'fulfilled' && backendRes.value.ok) {
          setBackendHealth(await backendRes.value.json());
        } else {
          setBackendHealth({ status: 'error' });
        }

        if (aiRes.status === 'fulfilled' && aiRes.value.ok) {
          setAiHealth(await aiRes.value.json());
        } else {
          setAiHealth({ status: 'error' });
        }
      } catch {
        setBackendHealth({ status: 'error' });
        setAiHealth({ status: 'error' });
      } finally {
        setLoading(false);
      }
    };

    checkHealth();
  }, []);

  const toggleTheme = () => {
    const newTheme = theme === 'light' ? 'dark' : 'light';
    setTheme(newTheme);
    document.documentElement.classList.toggle('dark');
  };

  return (
    <main className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 dark:from-slate-900 dark:to-slate-800">
      {/* Navigation */}
      <nav className="border-b border-slate-200 bg-white/80 backdrop-blur-sm dark:border-slate-700 dark:bg-slate-900/80">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
          <div className="flex items-center gap-2">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-600 text-white">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="h-6 w-6"
              >
                <path d="M4.5 3.75a3 3 0 00-3 3v.75h21v-.75a3 3 0 00-3-3h-15z" />
                <path
                  fillRule="evenodd"
                  d="M1.5 9.75v7.5a3 3 0 003 3h15a3 3 0 003-3v-7.5h-21zm4.5 3a.75.75 0 01.75-.75h7.5a.75.75 0 010 1.5h-7.5a.75.75 0 01-.75-.75z"
                  clipRule="evenodd"
                />
              </svg>
            </div>
            <span className="text-xl font-bold text-slate-900 dark:text-white">
              Accountant CRM
            </span>
          </div>

          <button
            onClick={toggleTheme}
            className="rounded-lg p-2 text-slate-500 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800"
            aria-label="Toggle theme"
          >
            {theme === 'light' ? (
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="h-5 w-5"
              >
                <path
                  fillRule="evenodd"
                  d="M9.528 1.718a.75.75 0 01.162.819A8.97 8.97 0 009 6a9 9 0 009 9 8.97 8.97 0 003.463-.69.75.75 0 01.981.98 10.503 10.503 0 01-9.694 6.46c-5.799 0-10.5-4.701-10.5-10.5 0-4.368 2.667-8.112 6.46-9.694a.75.75 0 01.818.162z"
                  clipRule="evenodd"
                />
              </svg>
            ) : (
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="h-5 w-5"
              >
                <path d="M12 2.25a.75.75 0 01.75.75v2.25a.75.75 0 01-1.5 0V3a.75.75 0 01.75-.75zM7.5 12a4.5 4.5 0 119 0 4.5 4.5 0 01-9 0zM18.894 6.166a.75.75 0 00-1.06-1.06l-1.591 1.59a.75.75 0 101.06 1.061l1.591-1.59zM21.75 12a.75.75 0 01-.75.75h-2.25a.75.75 0 010-1.5H21a.75.75 0 01.75.75zM17.834 18.894a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 10-1.061 1.06l1.59 1.591zM12 18a.75.75 0 01.75.75V21a.75.75 0 01-1.5 0v-2.25A.75.75 0 0112 18zM7.758 17.303a.75.75 0 00-1.061-1.06l-1.591 1.59a.75.75 0 001.06 1.061l1.591-1.59zM6 12a.75.75 0 01-.75.75H3a.75.75 0 010-1.5h2.25A.75.75 0 016 12zM6.697 7.757a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 00-1.061 1.06l1.59 1.591z" />
              </svg>
            )}
          </button>
        </div>
      </nav>

      {/* Hero Section */}
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
        <div className="text-center">
          <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-5xl md:text-6xl">
            <span className="block">AI-Powered</span>
            <span className="block text-primary-600 dark:text-primary-400">
              Accounting Platform
            </span>
          </h1>
          <p className="mx-auto mt-6 max-w-2xl text-lg text-slate-600 dark:text-slate-300">
            Streamline your accounting workflows with intelligent document
            processing, automated data extraction, and smart insights.
          </p>
          <div className="mt-10 flex justify-center gap-4">
            <button className="btn-primary">
              Get Started
            </button>
            <button className="btn-secondary">
              Learn More
            </button>
          </div>
        </div>
      </div>

      {/* Status Cards */}
      <div className="mx-auto max-w-7xl px-4 pb-16 sm:px-6 lg:px-8">
        <h2 className="mb-6 text-center text-xl font-semibold text-slate-900 dark:text-white">
          System Status
        </h2>
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {/* Backend Status */}
          <div className="card">
            <div className="flex items-center gap-3">
              <div
                className={`h-3 w-3 rounded-full ${
                  loading
                    ? 'animate-pulse bg-yellow-500'
                    : backendHealth?.db === 'ok'
                    ? 'bg-green-500'
                    : 'bg-red-500'
                }`}
              />
              <h3 className="font-medium text-slate-900 dark:text-white">
                Go Backend
              </h3>
            </div>
            <div className="mt-4 space-y-2 text-sm text-slate-600 dark:text-slate-400">
              {loading ? (
                <p>Checking connection...</p>
              ) : backendHealth ? (
                <>
                  <p>
                    Database:{' '}
                    <span
                      className={
                        backendHealth.db === 'ok'
                          ? 'text-green-600 dark:text-green-400'
                          : 'text-red-600 dark:text-red-400'
                      }
                    >
                      {backendHealth.db || 'unknown'}
                    </span>
                  </p>
                  <p>
                    Redis:{' '}
                    <span
                      className={
                        backendHealth.redis === 'ok'
                          ? 'text-green-600 dark:text-green-400'
                          : 'text-red-600 dark:text-red-400'
                      }
                    >
                      {backendHealth.redis || 'unknown'}
                    </span>
                  </p>
                </>
              ) : (
                <p className="text-red-600 dark:text-red-400">
                  Unable to connect
                </p>
              )}
            </div>
          </div>

          {/* AI Service Status */}
          <div className="card">
            <div className="flex items-center gap-3">
              <div
                className={`h-3 w-3 rounded-full ${
                  loading
                    ? 'animate-pulse bg-yellow-500'
                    : aiHealth?.status === 'ok'
                    ? 'bg-green-500'
                    : 'bg-red-500'
                }`}
              />
              <h3 className="font-medium text-slate-900 dark:text-white">
                AI Service
              </h3>
            </div>
            <div className="mt-4 space-y-2 text-sm text-slate-600 dark:text-slate-400">
              {loading ? (
                <p>Checking connection...</p>
              ) : aiHealth ? (
                <p>
                  Status:{' '}
                  <span
                    className={
                      aiHealth.status === 'ok'
                        ? 'text-green-600 dark:text-green-400'
                        : 'text-red-600 dark:text-red-400'
                    }
                  >
                    {aiHealth.status}
                  </span>
                </p>
              ) : (
                <p className="text-red-600 dark:text-red-400">
                  Unable to connect
                </p>
              )}
            </div>
          </div>

          {/* Features */}
          <div className="card">
            <h3 className="font-medium text-slate-900 dark:text-white">
              Features
            </h3>
            <ul className="mt-4 space-y-2 text-sm text-slate-600 dark:text-slate-400">
              <li className="flex items-center gap-2">
                <svg
                  className="h-4 w-4 text-primary-600"
                  fill="currentColor"
                  viewBox="0 0 20 20"
                >
                  <path
                    fillRule="evenodd"
                    d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                    clipRule="evenodd"
                  />
                </svg>
                Multi-tenant Architecture
              </li>
              <li className="flex items-center gap-2">
                <svg
                  className="h-4 w-4 text-primary-600"
                  fill="currentColor"
                  viewBox="0 0 20 20"
                >
                  <path
                    fillRule="evenodd"
                    d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                    clipRule="evenodd"
                  />
                </svg>
                AI Document Processing
              </li>
              <li className="flex items-center gap-2">
                <svg
                  className="h-4 w-4 text-primary-600"
                  fill="currentColor"
                  viewBox="0 0 20 20"
                >
                  <path
                    fillRule="evenodd"
                    d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                    clipRule="evenodd"
                  />
                </svg>
                Dark/Light Theme
              </li>
            </ul>
          </div>
        </div>
      </div>

      {/* Footer */}
      <footer className="border-t border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
        <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
          <p className="text-center text-sm text-slate-500 dark:text-slate-400">
            &copy; {new Date().getFullYear()} Accountant CRM. All rights
            reserved.
          </p>
        </div>
      </footer>
    </main>
  );
}
