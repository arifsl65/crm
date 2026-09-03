'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getDashboardDeadlines, DashboardDeadline } from '@/lib/api';

export function TodayPanel() {
  const [deadlines, setDeadlines] = useState<DashboardDeadline[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    async function fetchDeadlines() {
      try {
        setError(null);
        const data = await getDashboardDeadlines();
        setDeadlines(data.deadlines || []);
      } catch (err) {
        console.error('Failed to load deadlines:', err);
        setError('Failed to load tasks');
      } finally {
        setLoading(false);
      }
    }
    fetchDeadlines();
  }, []);

  // Group deadlines by urgency
  const doFirst = deadlines.filter(d => d.urgency === 'overdue' || d.urgency === 'today');
  const laterToday = deadlines.filter(d => d.urgency === 'urgent' || d.urgency === 'soon');
  const upcoming = deadlines.filter(d => d.urgency === 'upcoming');

  const formatDeadline = (deadline: string, urgency: string) => {
    const date = new Date(deadline);
    const today = new Date();
    const diffDays = Math.ceil((date.getTime() - today.getTime()) / (1000 * 60 * 60 * 24));

    if (urgency === 'overdue') {
      return `${Math.abs(diffDays)} day${Math.abs(diffDays) !== 1 ? 's' : ''} overdue`;
    } else if (urgency === 'today') {
      return 'Due today';
    } else if (diffDays === 1) {
      return 'Due tomorrow';
    } else {
      return `Due in ${diffDays} days`;
    }
  };

  const getUrgencyIcon = (urgency: string) => {
    switch (urgency) {
      case 'overdue':
        return '🔴';
      case 'today':
        return '🟠';
      case 'urgent':
        return '🟡';
      case 'soon':
        return '🟡';
      default:
        return '🟢';
    }
  };

  if (loading) {
    return (
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-sm p-6 mb-8">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>📅</span> Today
          </h3>
        </div>
        <div className="animate-pulse space-y-3">
          <div className="h-16 bg-gray-200 dark:bg-slate-700 rounded"></div>
          <div className="h-16 bg-gray-200 dark:bg-slate-700 rounded"></div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-sm p-6 mb-8">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>📅</span> Today
          </h3>
        </div>
        <p className="text-red-500 text-sm">{error}</p>
      </div>
    );
  }

  const totalTasks = doFirst.length + laterToday.length;

  return (
    <div className="bg-white dark:bg-slate-800 rounded-lg shadow-sm p-6 mb-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>📅</span> Today
          {totalTasks > 0 && (
            <span className="ml-2 px-2 py-0.5 text-xs font-medium rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
              {totalTasks} task{totalTasks !== 1 ? 's' : ''}
            </span>
          )}
        </h3>
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
        >
          {collapsed ? (
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          ) : (
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
            </svg>
          )}
        </button>
      </div>

      {!collapsed && (
        <>
          {/* Empty State */}
          {totalTasks === 0 && upcoming.length === 0 && (
            <div className="text-center py-8">
              <div className="text-4xl mb-3">☀️</div>
              <p className="text-gray-600 dark:text-gray-400 font-medium">All caught up!</p>
              <p className="text-gray-500 dark:text-gray-500 text-sm mt-1">No urgent tasks or deadlines for today.</p>
              <Link
                href="/dashboard/services"
                className="inline-block mt-4 text-sm text-blue-600 dark:text-blue-400 hover:underline"
              >
                View all services →
              </Link>
            </div>
          )}

          {/* Do First Section */}
          {doFirst.length > 0 && (
            <div className="mb-6">
              <h4 className="text-sm font-semibold text-red-600 dark:text-red-400 mb-3 flex items-center gap-2">
                🔴 DO FIRST ({doFirst.length})
              </h4>
              <div className="space-y-2">
                {doFirst.map((task) => (
                  <TaskCard key={task.id} task={task} formatDeadline={formatDeadline} />
                ))}
              </div>
            </div>
          )}

          {/* Later Today Section */}
          {laterToday.length > 0 && (
            <div className="mb-6">
              <h4 className="text-sm font-semibold text-yellow-600 dark:text-yellow-400 mb-3 flex items-center gap-2">
                🟡 LATER TODAY ({laterToday.length})
              </h4>
              <div className="space-y-2">
                {laterToday.map((task) => (
                  <TaskCard key={task.id} task={task} formatDeadline={formatDeadline} compact />
                ))}
              </div>
            </div>
          )}

          {/* Upcoming Section (collapsed by default, show count) */}
          {upcoming.length > 0 && (
            <div className="pt-4 border-t border-gray-200 dark:border-gray-700">
              <Link
                href="/dashboard/services"
                className="text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 flex items-center justify-between"
              >
                <span className="flex items-center gap-2">
                  🟢 {upcoming.length} upcoming task{upcoming.length !== 1 ? 's' : ''} this week
                </span>
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </Link>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function TaskCard({
  task,
  formatDeadline,
  compact = false
}: {
  task: DashboardDeadline;
  formatDeadline: (deadline: string, urgency: string) => string;
  compact?: boolean;
}) {
  const isOverdue = task.urgency === 'overdue';

  return (
    <div className={`
      flex items-center justify-between p-3 rounded-lg border
      ${isOverdue
        ? 'border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-900/20'
        : 'border-gray-200 bg-gray-50 dark:border-gray-700 dark:bg-slate-700/50'
      }
    `}>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm">
            {task.priority === 'high' ? '⚡' : task.priority === 'urgent' ? '🔥' : '📋'}
          </span>
          <p className={`font-medium truncate ${compact ? 'text-sm' : 'text-sm'} text-gray-900 dark:text-white`}>
            {task.name}
          </p>
        </div>
        <div className="flex items-center gap-2 mt-1">
          <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
            {task.client_name}
          </p>
          <span className="text-gray-300 dark:text-gray-600">•</span>
          <p className={`text-xs ${isOverdue ? 'text-red-600 dark:text-red-400 font-medium' : 'text-gray-500 dark:text-gray-400'}`}>
            {formatDeadline(task.deadline, task.urgency)}
          </p>
        </div>
      </div>
      <Link
        href={`/dashboard/services?search=${encodeURIComponent(task.name)}`}
        className={`
          ml-3 px-3 py-1.5 text-xs font-medium rounded-md transition-colors whitespace-nowrap
          ${isOverdue
            ? 'bg-red-600 text-white hover:bg-red-700'
            : 'bg-blue-600 text-white hover:bg-blue-700'
          }
        `}
      >
        View →
      </Link>
    </div>
  );
}

export default TodayPanel;
