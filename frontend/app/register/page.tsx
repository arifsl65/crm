'use client';

import Link from 'next/link';

export default function RegisterPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-slate-900 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        <div>
          <h2 className="mt-6 text-center text-3xl font-extrabold text-gray-900 dark:text-white">
            Join Your Organization
          </h2>
        </div>

        <div className="bg-white dark:bg-slate-800 rounded-lg shadow-sm p-6 space-y-4">
          <div className="text-center">
            <svg
              className="mx-auto h-12 w-12 text-blue-500"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          <div className="text-center space-y-2">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white">
              Invitation Required
            </h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Registration is by invitation only. If your organization uses this platform,
              ask your administrator to send you an invitation.
            </p>
          </div>

          <div className="bg-blue-50 dark:bg-blue-900/30 rounded-md p-4">
            <h4 className="text-sm font-medium text-blue-800 dark:text-blue-300 mb-2">
              Have an invitation?
            </h4>
            <p className="text-sm text-blue-700 dark:text-blue-400">
              Check your email for an invitation link from your organization.
              Click the link to set up your account.
            </p>
          </div>

          <div className="border-t border-gray-200 dark:border-gray-700 pt-4">
            <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Need an invitation?
            </h4>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Contact your organization administrator to request access.
            </p>
          </div>
        </div>

        <div className="text-center">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Already have an account?{' '}
            <Link href="/login" className="font-medium text-blue-600 hover:text-blue-500 dark:text-blue-400">
              Sign in
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}
