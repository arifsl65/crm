'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/components/auth-guard';
import {
  Notification,
  getNotifications,
  markNotificationRead,
  markAllNotificationsRead,
  dismissNotification,
  dismissAllNotifications,
} from '@/lib/api';
import { useToast } from '@/components';

const notificationTypeIcons: Record<string, string> = {
  document: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z',
  deadline: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z',
  email: 'M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z',
  system: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z',
  reminder: 'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9',
};

const notificationTypeColors: Record<string, string> = {
  document: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400',
  deadline: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400',
  email: 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400',
  system: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400',
  reminder: 'bg-yellow-100 text-yellow-600 dark:bg-yellow-900/30 dark:text-yellow-400',
};

function formatTimeAgo(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (seconds < 60) return 'Just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`;
  return date.toLocaleDateString();
}

export default function NotificationsPage() {
  const { user } = useAuth();
  const router = useRouter();
  const toast = useToast();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [unreadCount, setUnreadCount] = useState(0);
  const [filter, setFilter] = useState<'all' | 'unread'>('all');
  const [typeFilter, setTypeFilter] = useState<string>('');

  const fetchNotifications = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getNotifications({
        unread: filter === 'unread',
        type: typeFilter || undefined,
        limit: 100,
      });
      setNotifications(data.notifications || []);
      setUnreadCount(data.unread_count || 0);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load notifications');
    } finally {
      setLoading(false);
    }
  }, [filter, typeFilter]);

  useEffect(() => {
    fetchNotifications();
  }, [fetchNotifications]);

  const handleMarkRead = async (id: string) => {
    try {
      await markNotificationRead(id);
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, is_read: true } : n))
      );
      setUnreadCount((prev) => Math.max(0, prev - 1));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to mark as read');
    }
  };

  const handleMarkAllRead = async () => {
    try {
      const result = await markAllNotificationsRead();
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
      setUnreadCount(0);
      toast.success(`Marked ${result.count} notifications as read`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to mark all as read');
    }
  };

  const handleDismiss = async (id: string) => {
    try {
      await dismissNotification(id);
      setNotifications((prev) => prev.filter((n) => n.id !== id));
      toast.success('Notification dismissed');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to dismiss notification');
    }
  };

  const handleDismissAll = async () => {
    if (!confirm('Are you sure you want to dismiss all notifications?')) return;
    try {
      const result = await dismissAllNotifications();
      setNotifications([]);
      setUnreadCount(0);
      toast.success(`Dismissed ${result.count} notifications`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to dismiss all notifications');
    }
  };

  const handleNotificationClick = async (notification: Notification) => {
    if (!notification.is_read) {
      await handleMarkRead(notification.id);
    }
    if (notification.link) {
      router.push(notification.link);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <header className="bg-white dark:bg-slate-800 shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
          <div className="flex items-center space-x-4">
            <Link
              href="/dashboard"
              className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
              </svg>
            </Link>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              Notifications
              {unreadCount > 0 && (
                <span className="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300">
                  {unreadCount} unread
                </span>
              )}
            </h1>
          </div>
          <div className="flex items-center space-x-2">
            {unreadCount > 0 && (
              <button
                onClick={handleMarkAllRead}
                className="inline-flex items-center px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-md transition-colors"
              >
                <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                Mark all read
              </button>
            )}
            {notifications.length > 0 && (
              <button
                onClick={handleDismissAll}
                className="inline-flex items-center px-3 py-2 text-sm font-medium text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-md transition-colors"
              >
                <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
                Clear all
              </button>
            )}
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Filters */}
        <div className="mb-6 flex flex-col sm:flex-row gap-4">
          <div className="flex rounded-lg overflow-hidden border border-gray-300 dark:border-gray-600">
            <button
              onClick={() => setFilter('all')}
              className={`px-4 py-2 text-sm font-medium ${
                filter === 'all'
                  ? 'bg-blue-600 text-white'
                  : 'bg-white dark:bg-slate-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-slate-600'
              }`}
            >
              All
            </button>
            <button
              onClick={() => setFilter('unread')}
              className={`px-4 py-2 text-sm font-medium ${
                filter === 'unread'
                  ? 'bg-blue-600 text-white'
                  : 'bg-white dark:bg-slate-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-slate-600'
              }`}
            >
              Unread
            </button>
          </div>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
          >
            <option value="">All types</option>
            <option value="document">Documents</option>
            <option value="deadline">Deadlines</option>
            <option value="email">Emails</option>
            <option value="system">System</option>
            <option value="reminder">Reminders</option>
          </select>
        </div>

        {/* Error */}
        {error && (
          <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
            <p className="text-red-700 dark:text-red-300">{error}</p>
          </div>
        )}

        {/* Notifications List */}
        <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
          {loading ? (
            <div className="p-8 flex justify-center">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
          ) : notifications.length === 0 ? (
            <div className="p-12 text-center">
              <svg
                className="mx-auto h-12 w-12 text-gray-400"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
                />
              </svg>
              <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                No notifications
              </h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {filter === 'unread' ? "You're all caught up!" : "You don't have any notifications yet."}
              </p>
            </div>
          ) : (
            <ul className="divide-y divide-gray-200 dark:divide-gray-700">
              {notifications.map((notification) => (
                <li
                  key={notification.id}
                  className={`hover:bg-gray-50 dark:hover:bg-slate-700/50 transition-colors ${
                    !notification.is_read ? 'bg-blue-50/50 dark:bg-blue-900/10' : ''
                  }`}
                >
                  <div className="px-4 py-4 sm:px-6 flex items-start space-x-4">
                    {/* Icon */}
                    <div
                      className={`flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${
                        notificationTypeColors[notification.type] || notificationTypeColors.system
                      }`}
                    >
                      <svg
                        className="w-5 h-5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d={notificationTypeIcons[notification.type] || notificationTypeIcons.system}
                        />
                      </svg>
                    </div>

                    {/* Content */}
                    <div
                      className="flex-1 min-w-0 cursor-pointer"
                      onClick={() => handleNotificationClick(notification)}
                    >
                      <div className="flex items-center justify-between">
                        <p
                          className={`text-sm font-medium ${
                            notification.is_read
                              ? 'text-gray-900 dark:text-white'
                              : 'text-blue-900 dark:text-blue-100'
                          }`}
                        >
                          {notification.title}
                        </p>
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          {formatTimeAgo(notification.created_at)}
                        </span>
                      </div>
                      <p className="mt-1 text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
                        {notification.message}
                      </p>
                      {!notification.is_read && (
                        <span className="inline-flex items-center mt-2 px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300">
                          New
                        </span>
                      )}
                    </div>

                    {/* Actions */}
                    <div className="flex-shrink-0 flex items-center space-x-2">
                      {!notification.is_read && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            handleMarkRead(notification.id);
                          }}
                          className="p-1 text-gray-400 hover:text-blue-600 dark:hover:text-blue-400"
                          title="Mark as read"
                        >
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                          </svg>
                        </button>
                      )}
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDismiss(notification.id);
                        }}
                        className="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400"
                        title="Dismiss"
                      >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </main>
    </div>
  );
}
