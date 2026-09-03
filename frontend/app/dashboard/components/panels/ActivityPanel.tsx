'use client';

import { useEffect, useState } from 'react';
import { getDashboardStats, DashboardStats } from '@/lib/api';

interface ActivityItem {
  id: string;
  description: string;
  user_name?: string;
  created_at: string;
}

interface ActivityPanelProps {
  onClose?: () => void;
}

export function ActivityPanel({ onClose }: ActivityPanelProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activities, setActivities] = useState<ActivityItem[]>([]);
  const [filterStaff, setFilterStaff] = useState<string>('all');

  useEffect(() => {
    async function fetchActivity() {
      try {
        setLoading(true);
        const data: DashboardStats = await getDashboardStats();

        const items: ActivityItem[] = (data.recent_activity || []).map((activity) => ({
          id: activity.id,
          description: activity.description,
          user_name: activity.user_name,
          created_at: activity.created_at,
        }));

        setActivities(items);
        setError(null);
      } catch (err) {
        setError('Failed to load activity');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchActivity();
  }, []);

  // Get unique staff names for filter
  const staffNames = Array.from(new Set(activities.map(a => a.user_name).filter(Boolean)));

  // Filter activities
  const filteredActivities = filterStaff === 'all'
    ? activities
    : activities.filter(a => a.user_name === filterStaff);

  // Group activities by date
  const groupedActivities = groupByDate(filteredActivities);

  const handleExport = () => {
    // Export activity log as CSV
    const csv = filteredActivities.map(a => {
      const date = new Date(a.created_at);
      return `${date.toLocaleDateString()},${date.toLocaleTimeString()},${a.user_name || 'System'},${a.description}`;
    }).join('\n');

    const blob = new Blob([`Date,Time,User,Action\n${csv}`], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'activity-log.csv';
    link.click();
  };

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>📜</span> ACTIVITY
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
          <span>📜</span> ACTIVITY
        </h2>
        {onClose && (
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
            ✕
          </button>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {error && (
          <div className="p-3 bg-red-50 dark:bg-red-900/30 rounded-lg text-red-700 dark:text-red-300 text-sm mb-4">
            {error}
          </div>
        )}

        {filteredActivities.length === 0 && !error && (
          <div className="text-center py-8">
            <span className="text-4xl">📭</span>
            <p className="mt-2 text-gray-600 dark:text-gray-400">No recent activity</p>
          </div>
        )}

        {Object.entries(groupedActivities).map(([date, items]) => (
          <div key={date} className="mb-6">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
              {date}
            </h3>
            <div className="space-y-2">
              {items.map((activity) => (
                <div key={activity.id} className="flex items-start gap-3 text-sm">
                  <span className="text-gray-500 dark:text-gray-400 font-mono w-12 flex-shrink-0">
                    {formatTime(activity.created_at)}
                  </span>
                  <span className="text-gray-700 dark:text-gray-300">
                    {activity.user_name && (
                      <span className="font-medium">{activity.user_name} </span>
                    )}
                    {activity.description}
                  </span>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700 flex items-center gap-3">
        <select
          value={filterStaff}
          onChange={(e) => setFilterStaff(e.target.value)}
          className="flex-1 text-sm px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
        >
          <option value="all">Filter by Staff</option>
          {staffNames.map((name) => (
            <option key={name} value={name}>{name}</option>
          ))}
        </select>
        <button
          onClick={handleExport}
          className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
        >
          Export
        </button>
      </div>
    </div>
  );
}

function formatTime(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
}

function groupByDate(activities: ActivityItem[]): Record<string, ActivityItem[]> {
  const groups: Record<string, ActivityItem[]> = {};
  const now = new Date();
  const today = now.toDateString();
  const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000).toDateString();

  activities.forEach((activity) => {
    const date = new Date(activity.created_at);
    const dateString = date.toDateString();

    let label: string;
    if (dateString === today) {
      label = 'Today';
    } else if (dateString === yesterday) {
      label = 'Yesterday';
    } else {
      label = date.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'short' });
    }

    if (!groups[label]) {
      groups[label] = [];
    }
    groups[label].push(activity);
  });

  return groups;
}

export default ActivityPanel;
