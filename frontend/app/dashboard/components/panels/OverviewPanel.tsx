'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getDashboardStats, DashboardStats } from '@/lib/api';

interface OverviewPanelProps {
  onClose?: () => void;
}

export function OverviewPanel({ onClose }: OverviewPanelProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [stats, setStats] = useState<DashboardStats | null>(null);

  useEffect(() => {
    async function fetchStats() {
      try {
        setLoading(true);
        const data = await getDashboardStats();
        setStats(data);
        setError(null);
      } catch (err) {
        setError('Failed to load overview');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchStats();
  }, []);

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>📊</span> Overview
          </h2>
          {onClose && (
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
              ✕
            </button>
          )}
        </div>
        <div className="flex-1 flex items-center justify-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col bg-white dark:bg-slate-800 rounded-lg">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>📊</span> Overview
        </h2>
        {onClose && (
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
            ✕
          </button>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 space-y-6">
        {error && (
          <div className="p-3 bg-red-50 dark:bg-red-900/30 rounded-lg text-red-700 dark:text-red-300 text-sm">
            {error}
          </div>
        )}

        {/* Main Stats Grid */}
        <div className="grid grid-cols-2 gap-4">
          <StatCard
            icon="👥"
            label="Total Clients"
            value={stats?.total_clients || 0}
            subtitle={`${stats?.active_clients || 0} active`}
            href="/dashboard/clients"
            color="blue"
          />
          <StatCard
            icon="📋"
            label="In Progress"
            value={stats?.services_in_progress || 0}
            subtitle={`${stats?.services_completed || 0} completed`}
            href="/dashboard/services"
            color="green"
          />
          <StatCard
            icon="📄"
            label="Pending Documents"
            value={stats?.documents_pending || 0}
            subtitle="awaiting review"
            href="/dashboard/documents?status=pending"
            color="orange"
          />
          <StatCard
            icon="⏰"
            label="Due Soon"
            value={stats?.services_due_soon || 0}
            subtitle="deadlines"
            href="/dashboard/services?due=week"
            color="purple"
          />
        </div>

        {/* Overdue Section */}
        {(stats?.services_overdue || 0) > 0 && (
          <div className="bg-red-50 dark:bg-red-900/20 rounded-lg p-4 border border-red-200 dark:border-red-800">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <span className="text-2xl">🔴</span>
                <div>
                  <p className="font-semibold text-red-900 dark:text-red-100">
                    {stats?.services_overdue} Overdue
                  </p>
                  <p className="text-sm text-red-700 dark:text-red-300">
                    Services past their deadline
                  </p>
                </div>
              </div>
              <Link
                href="/dashboard/services?status=overdue"
                className="px-3 py-1.5 text-sm font-medium text-white bg-red-600 hover:bg-red-700 rounded-lg"
              >
                View All
              </Link>
            </div>
          </div>
        )}

        {/* Services Summary */}
        <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
            📋 Services Overview
          </h3>
          <div className="grid grid-cols-3 gap-3">
            <div className="text-center p-3 bg-white dark:bg-slate-800 rounded-lg">
              <p className="text-2xl font-bold text-blue-600">
                {stats?.total_services || 0}
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400">Total</p>
            </div>
            <div className="text-center p-3 bg-white dark:bg-slate-800 rounded-lg">
              <p className="text-2xl font-bold text-green-600">
                {stats?.services_in_progress || 0}
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400">In Progress</p>
            </div>
            <div className="text-center p-3 bg-white dark:bg-slate-800 rounded-lg">
              <p className="text-2xl font-bold text-purple-600">
                {stats?.services_completed || 0}
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400">Completed</p>
            </div>
          </div>
        </div>

        {/* Document Stats */}
        <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
            📄 Documents
          </h3>
          <div className="space-y-2">
            <ProgressBar
              label="Approved"
              value={stats?.documents_approved || 0}
              total={stats?.total_documents || 1}
              color="green"
            />
            <ProgressBar
              label="Pending"
              value={stats?.documents_pending || 0}
              total={stats?.total_documents || 1}
              color="yellow"
            />
            <ProgressBar
              label="Requested"
              value={stats?.documents_requested || 0}
              total={stats?.total_documents || 1}
              color="red"
            />
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700">
        <Link
          href="/dashboard/reports"
          className="w-full flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
        >
          View Full Reports →
        </Link>
      </div>
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
  subtitle,
  href,
  color,
}: {
  icon: string;
  label: string;
  value: number;
  subtitle: string;
  href: string;
  color: 'blue' | 'green' | 'orange' | 'purple';
}) {
  const colorClasses = {
    blue: 'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800',
    green: 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800',
    orange: 'bg-orange-50 dark:bg-orange-900/20 border-orange-200 dark:border-orange-800',
    purple: 'bg-purple-50 dark:bg-purple-900/20 border-purple-200 dark:border-purple-800',
  };

  const valueColors = {
    blue: 'text-blue-600 dark:text-blue-400',
    green: 'text-green-600 dark:text-green-400',
    orange: 'text-orange-600 dark:text-orange-400',
    purple: 'text-purple-600 dark:text-purple-400',
  };

  return (
    <Link
      href={href}
      className={`block p-4 rounded-lg border ${colorClasses[color]} hover:shadow-md transition-shadow`}
    >
      <div className="flex items-center gap-2 mb-2">
        <span className="text-xl">{icon}</span>
        <span className="text-xs font-medium text-gray-600 dark:text-gray-400">{label}</span>
      </div>
      <p className={`text-2xl font-bold ${valueColors[color]}`}>{value}</p>
      <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{subtitle}</p>
    </Link>
  );
}

function ProgressBar({
  label,
  value,
  total,
  color,
}: {
  label: string;
  value: number;
  total: number;
  color: 'green' | 'yellow' | 'red';
}) {
  const percentage = total > 0 ? Math.round((value / total) * 100) : 0;

  const barColors = {
    green: 'bg-green-500',
    yellow: 'bg-yellow-500',
    red: 'bg-red-500',
  };

  return (
    <div className="flex items-center gap-3">
      <span className="text-xs text-gray-600 dark:text-gray-400 w-16">{label}</span>
      <div className="flex-1 h-2 bg-gray-200 dark:bg-slate-600 rounded-full overflow-hidden">
        <div
          className={`h-full ${barColors[color]} rounded-full transition-all`}
          style={{ width: `${percentage}%` }}
        />
      </div>
      <span className="text-xs font-medium text-gray-700 dark:text-gray-300 w-8 text-right">
        {value}
      </span>
    </div>
  );
}

export default OverviewPanel;
