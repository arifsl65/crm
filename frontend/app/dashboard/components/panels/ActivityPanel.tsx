'use client';

import { useEffect, useState } from 'react';
import { getDashboardStats, DashboardStats } from '@/lib/api';

interface ActivityItem {
  id: string;
  description: string;
  user_name?: string;
  created_at: string;
  type: 'document' | 'client' | 'service' | 'email' | 'other';
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

        // Transform recent_activity to our format
        const items: ActivityItem[] = (data.recent_activity || []).map((activity) => ({
          id: activity.id,
          description: activity.description,
          user_name: activity.user_name,
          created_at: activity.created_at,
          type: getActivityType(activity.description),
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

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>📜</span> Activity
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
          <span>📜</span> Activity Log
        </h2>
        {onClose && (
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
            ✕
          </button>
        )}
      </div>

      {/* Filter */}
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center gap-2">
        <label className="text-sm text-gray-600 dark:text-gray-400">Filter by:</label>
        <select
          value={filterStaff}
          onChange={(e) => setFilterStaff(e.target.value)}
          className="text-sm px-2 py-1 border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
        >
          <option value="all">All Staff</option>
          {staffNames.map((name) => (
            <option key={name} value={name}>{name}</option>
          ))}
        </select>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {error && (
          <div className="p-3 bg-red-50 dark:bg-red-900/30 rounded-lg text-red-700 dark:text-red-300 text-sm">
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
            <h3 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase mb-3">
              {date}
            </h3>
            <div className="space-y-3">
              {items.map((activity) => (
                <ActivityRow key={activity.id} activity={activity} />
              ))}
            </div>
          </div>
        ))}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700 flex gap-2">
        <button className="flex-1 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
          Export
        </button>
        <button className="flex-1 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
          View Full Log
        </button>
      </div>
    </div>
  );
}

function ActivityRow({ activity }: { activity: ActivityItem }) {
  const getIcon = () => {
    switch (activity.type) {
      case 'document': return '📄';
      case 'client': return '👤';
      case 'service': return '📋';
      case 'email': return '📧';
      default: return '📌';
    }
  };

  const getIconBg = () => {
    switch (activity.type) {
      case 'document': return 'bg-blue-100 dark:bg-blue-900/30';
      case 'client': return 'bg-green-100 dark:bg-green-900/30';
      case 'service': return 'bg-purple-100 dark:bg-purple-900/30';
      case 'email': return 'bg-orange-100 dark:bg-orange-900/30';
      default: return 'bg-gray-100 dark:bg-gray-700';
    }
  };

  const formatTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
  };

  return (
    <div className="flex items-start gap-3">
      <div className={`w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 ${getIconBg()}`}>
        <span className="text-sm">{getIcon()}</span>
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm text-gray-900 dark:text-white">
          {activity.description}
        </p>
        <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
          {activity.user_name && <span className="font-medium">{activity.user_name}</span>}
          {activity.user_name && ' · '}
          {formatTime(activity.created_at)}
        </p>
      </div>
    </div>
  );
}

function getActivityType(description: string): ActivityItem['type'] {
  const lower = description.toLowerCase();
  if (lower.includes('document') || lower.includes('upload') || lower.includes('approved') || lower.includes('rejected')) {
    return 'document';
  }
  if (lower.includes('client') || lower.includes('added') || lower.includes('created')) {
    return 'client';
  }
  if (lower.includes('service') || lower.includes('completed') || lower.includes('status')) {
    return 'service';
  }
  if (lower.includes('email') || lower.includes('sent') || lower.includes('chase')) {
    return 'email';
  }
  return 'other';
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
