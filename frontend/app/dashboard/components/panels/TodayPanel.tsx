'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getDashboardStats, getServices, DashboardStats, Service } from '@/lib/api';

interface TodayPanelProps {
  onClose?: () => void;
}

interface UrgentItem {
  id: string;
  type: 'deadline' | 'document' | 'chase';
  title: string;
  subtitle: string;
  urgency: 'overdue' | 'today' | 'soon';
  actionLabel: string;
  actionHref?: string;
}

export function TodayPanel({ onClose }: TodayPanelProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [urgentItems, setUrgentItems] = useState<UrgentItem[]>([]);

  useEffect(() => {
    async function fetchData() {
      try {
        setLoading(true);
        const [dashboardData, servicesData] = await Promise.all([
          getDashboardStats(),
          getServices({ limit: 50 }),
        ]);

        setStats(dashboardData);

        // Build urgent items from services
        const services = servicesData.services || [];
        const now = new Date();
        const items: UrgentItem[] = [];

        services.forEach((service: Service) => {
          if (!service.deadline || service.status === 'completed' || service.status === 'cancelled') return;

          const deadline = new Date(service.deadline);
          const daysUntil = Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));

          if (daysUntil <= 3) {
            items.push({
              id: service.id,
              type: 'deadline',
              title: service.client_name || 'Unknown Client',
              subtitle: service.name,
              urgency: daysUntil < 0 ? 'overdue' : daysUntil === 0 ? 'today' : 'soon',
              actionLabel: 'View',
              actionHref: `/dashboard/services/${service.id}`,
            });
          }
        });

        // Sort by urgency
        items.sort((a, b) => {
          const urgencyOrder = { overdue: 0, today: 1, soon: 2 };
          return urgencyOrder[a.urgency] - urgencyOrder[b.urgency];
        });

        setUrgentItems(items.slice(0, 10));
        setError(null);
      } catch (err) {
        setError('Failed to load today\'s data');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchData();
  }, []);

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>☀️</span> Today
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

  const greeting = getGreeting();

  return (
    <div className="h-full flex flex-col bg-white dark:bg-slate-800 rounded-lg">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>☀️</span> Today
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

        {/* Greeting */}
        <div className="text-center py-2">
          <p className="text-sm text-gray-600 dark:text-gray-400">{greeting}</p>
          <p className="text-xs text-gray-500 dark:text-gray-500 mt-1">
            {new Date().toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' })}
          </p>
        </div>

        {/* Quick Stats */}
        <div className="grid grid-cols-3 gap-3">
          <QuickStat
            value={stats?.services_overdue || 0}
            label="Overdue"
            color="red"
          />
          <QuickStat
            value={stats?.services_due_soon || 0}
            label="Due Soon"
            color="yellow"
          />
          <QuickStat
            value={stats?.documents_pending || 0}
            label="Pending Docs"
            color="blue"
          />
        </div>

        {/* Urgent Items */}
        {urgentItems.length > 0 ? (
          <div>
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
              Needs Attention
            </h3>
            <div className="space-y-2">
              {urgentItems.map((item) => (
                <UrgentItemRow key={item.id} item={item} />
              ))}
            </div>
          </div>
        ) : (
          <div className="text-center py-8">
            <span className="text-4xl">✨</span>
            <p className="mt-2 text-gray-600 dark:text-gray-400">All caught up!</p>
            <p className="text-sm text-gray-500 dark:text-gray-500">No urgent items today</p>
          </div>
        )}

        {/* Recent Activity Preview */}
        {stats?.recent_activity && stats.recent_activity.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
              Recent Activity
            </h3>
            <div className="space-y-2">
              {stats.recent_activity.slice(0, 3).map((activity) => (
                <div
                  key={activity.id}
                  className="flex items-start gap-2 p-2 rounded-lg bg-gray-50 dark:bg-slate-700"
                >
                  <span className="text-sm">📌</span>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-gray-900 dark:text-white truncate">
                      {activity.description}
                    </p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {formatTimeAgo(activity.created_at)}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Footer - Quick Actions */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700 space-y-2">
        <Link
          href="/dashboard/documents/upload"
          className="w-full flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700"
        >
          📤 Upload Document
        </Link>
        <div className="grid grid-cols-2 gap-2">
          <Link
            href="/dashboard/clients/new"
            className="flex items-center justify-center gap-1 px-3 py-2 text-xs font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
          >
            👤 Add Client
          </Link>
          <Link
            href="/dashboard/services/new"
            className="flex items-center justify-center gap-1 px-3 py-2 text-xs font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
          >
            📋 Add Service
          </Link>
        </div>
      </div>
    </div>
  );
}

function QuickStat({
  value,
  label,
  color,
}: {
  value: number;
  label: string;
  color: 'red' | 'yellow' | 'blue';
}) {
  const colorClasses = {
    red: 'text-red-600 dark:text-red-400',
    yellow: 'text-yellow-600 dark:text-yellow-400',
    blue: 'text-blue-600 dark:text-blue-400',
  };

  return (
    <div className="text-center p-3 bg-gray-50 dark:bg-slate-700 rounded-lg">
      <p className={`text-xl font-bold ${colorClasses[color]}`}>{value}</p>
      <p className="text-xs text-gray-500 dark:text-gray-400">{label}</p>
    </div>
  );
}

function UrgentItemRow({ item }: { item: UrgentItem }) {
  const getBgColor = () => {
    switch (item.urgency) {
      case 'overdue':
        return 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800';
      case 'today':
        return 'bg-orange-50 dark:bg-orange-900/20 border-orange-200 dark:border-orange-800';
      default:
        return 'bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-800';
    }
  };

  const getIcon = () => {
    switch (item.urgency) {
      case 'overdue':
        return '🔴';
      case 'today':
        return '🟠';
      default:
        return '🟡';
    }
  };

  const getUrgencyLabel = () => {
    switch (item.urgency) {
      case 'overdue':
        return 'Overdue';
      case 'today':
        return 'Today';
      default:
        return 'Soon';
    }
  };

  return (
    <div className={`flex items-center justify-between p-3 rounded-lg border ${getBgColor()}`}>
      <div className="flex items-center gap-3 min-w-0 flex-1">
        <span>{getIcon()}</span>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
            {item.title}
          </p>
          <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
            {item.subtitle}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-2 ml-2">
        <span className={`text-xs font-medium ${
          item.urgency === 'overdue' ? 'text-red-600 dark:text-red-400' :
          item.urgency === 'today' ? 'text-orange-600 dark:text-orange-400' :
          'text-yellow-600 dark:text-yellow-400'
        }`}>
          {getUrgencyLabel()}
        </span>
        {item.actionHref && (
          <Link
            href={item.actionHref}
            className="px-2 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded"
          >
            {item.actionLabel}
          </Link>
        )}
      </div>
    </div>
  );
}

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 18) return 'Good afternoon';
  return 'Good evening';
}

function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays === 1) return 'Yesterday';
  return `${diffDays}d ago`;
}

export default TodayPanel;
