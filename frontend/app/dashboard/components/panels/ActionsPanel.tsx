'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getDashboardStats, DashboardStats } from '@/lib/api';

interface ActionsPanelProps {
  onClose?: () => void;
}

export function ActionsPanel({ onClose }: ActionsPanelProps) {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [isProcessing, setIsProcessing] = useState<string | null>(null);

  useEffect(() => {
    async function fetchStats() {
      try {
        const data = await getDashboardStats();
        setStats(data);
      } catch (err) {
        console.error(err);
      }
    }
    fetchStats();
  }, []);

  const handleChaseAll = async () => {
    setIsProcessing('chase-all');
    // Navigate to overdue services to chase
    window.location.href = '/dashboard/services?status=overdue';
  };

  const handleBulkReminders = async () => {
    setIsProcessing('bulk-reminders');
    // Navigate to email compose with bulk option
    window.location.href = '/dashboard/email?action=bulk-reminder';
  };

  const overdueCount = stats?.services_overdue || 0;

  return (
    <div className="h-full flex flex-col bg-white dark:bg-slate-800 rounded-lg">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>⚡</span> ACTIONS
        </h2>
        {onClose && (
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
            ✕
          </button>
        )}
      </div>

      {/* Content - 4 Action Buttons */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {/* 📧 Chase All Overdue */}
        <button
          onClick={handleChaseAll}
          disabled={isProcessing === 'chase-all'}
          className="w-full p-4 rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-slate-700 hover:bg-gray-50 dark:hover:bg-slate-600 transition-all text-left"
        >
          <div className="flex items-center gap-3">
            <span className="text-xl">📧</span>
            <span className="font-medium text-gray-900 dark:text-white">
              Chase All Overdue ({overdueCount})
            </span>
          </div>
        </button>

        {/* 🔍 Show Troublemakers */}
        <Link
          href="/dashboard/clients?filter=at_risk"
          className="block w-full p-4 rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-slate-700 hover:bg-gray-50 dark:hover:bg-slate-600 transition-all"
        >
          <div className="flex items-center gap-3">
            <span className="text-xl">🔍</span>
            <span className="font-medium text-gray-900 dark:text-white">
              Show Troublemakers
            </span>
          </div>
        </Link>

        {/* 📨 Send Bulk Reminders */}
        <button
          onClick={handleBulkReminders}
          disabled={isProcessing === 'bulk-reminders'}
          className="w-full p-4 rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-slate-700 hover:bg-gray-50 dark:hover:bg-slate-600 transition-all text-left"
        >
          <div className="flex items-center gap-3">
            <span className="text-xl">📨</span>
            <span className="font-medium text-gray-900 dark:text-white">
              Send Bulk Reminders
            </span>
          </div>
        </button>

        {/* ✏️ Draft Reminder Email */}
        <Link
          href="/dashboard/email/templates"
          className="block w-full p-4 rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-slate-700 hover:bg-gray-50 dark:hover:bg-slate-600 transition-all"
        >
          <div className="flex items-center gap-3">
            <span className="text-xl">✏️</span>
            <span className="font-medium text-gray-900 dark:text-white">
              Draft Reminder Email
            </span>
          </div>
        </Link>
      </div>
    </div>
  );
}

export default ActionsPanel;
