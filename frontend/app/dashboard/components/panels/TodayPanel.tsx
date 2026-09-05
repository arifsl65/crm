'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { getDashboardStats, getServices, DashboardStats, Service } from '@/lib/api';
import { PendingReviewPanel } from '@/components/pending-review-panel';

interface TodayPanelProps {
  onClose?: () => void;
}

interface TaskItem {
  id: string;
  icon: string;
  title: string;
  subtitle: string;
  actionLabel: string;
  actionHref?: string;
  onAction?: () => void;
}

export function TodayPanel({ onClose }: TodayPanelProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [doFirstItems, setDoFirstItems] = useState<TaskItem[]>([]);
  const [laterTodayItems, setLaterTodayItems] = useState<TaskItem[]>([]);
  const [aiSuggestion, setAiSuggestion] = useState<string | null>(null);
  const [showReviewPanel, setShowReviewPanel] = useState(false);
  const [pendingDocsCount, setPendingDocsCount] = useState(0);

  useEffect(() => {
    async function fetchData() {
      try {
        setLoading(true);
        const [dashboardData, servicesData] = await Promise.all([
          getDashboardStats(),
          getServices({ limit: 50 }),
        ]);

        setStats(dashboardData);

        const services = servicesData.services || [];
        const now = new Date();
        const doFirst: TaskItem[] = [];
        const laterToday: TaskItem[] = [];

        // Build DO FIRST items (overdue, due today, documents needing review)
        services.forEach((service: Service) => {
          if (!service.deadline || service.status === 'completed' || service.status === 'cancelled') return;

          const deadline = new Date(service.deadline);
          const daysUntil = Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));

          if (daysUntil < 0) {
            // Overdue - DO FIRST
            doFirst.push({
              id: service.id,
              icon: '⏰',
              title: `${service.name} overdue`,
              subtitle: `${service.client_name} · ${Math.abs(daysUntil)} days overdue`,
              actionLabel: 'Chase →',
              actionHref: `/dashboard/services/${service.id}`,
            });
          } else if (daysUntil === 0) {
            // Due today - DO FIRST
            doFirst.push({
              id: service.id,
              icon: '⏰',
              title: `${service.name} due today`,
              subtitle: service.client_name || 'Unknown Client',
              actionLabel: 'View →',
              actionHref: `/dashboard/services/${service.id}`,
            });
          } else if (daysUntil <= 3) {
            // Due soon - LATER TODAY
            laterToday.push({
              id: service.id,
              icon: '📋',
              title: service.name,
              subtitle: service.client_name || 'Unknown Client',
              actionLabel: 'View →',
              actionHref: `/dashboard/services/${service.id}`,
            });
          }
        });

        // Add pending documents to DO FIRST
        setPendingDocsCount(dashboardData.documents_pending);
        if (dashboardData.documents_pending > 0) {
          doFirst.unshift({
            id: 'pending-docs',
            icon: '📄',
            title: `Review ${dashboardData.documents_pending} pending document${dashboardData.documents_pending > 1 ? 's' : ''}`,
            subtitle: 'Awaiting your review',
            actionLabel: 'Review →',
            onAction: () => setShowReviewPanel(true),
          });
        }

        setDoFirstItems(doFirst.slice(0, 5));
        setLaterTodayItems(laterToday.slice(0, 5));

        // AI Suggestion based on data
        if (dashboardData.services_overdue > 0) {
          setAiSuggestion(`Chase ${dashboardData.services_overdue} overdue`);
        } else if (dashboardData.documents_pending > 0) {
          setAiSuggestion(`Review ${dashboardData.documents_pending} pending documents`);
        } else {
          setAiSuggestion(null);
        }

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

  const handleAiAction = () => {
    if (stats?.services_overdue && stats.services_overdue > 0) {
      window.location.href = '/dashboard/services?status=overdue';
    } else if (pendingDocsCount > 0) {
      setShowReviewPanel(true);
    }
  };

  // Update pending docs count and UI when documents are reviewed
  const handlePendingCountChange = useCallback((count: number) => {
    setPendingDocsCount(count);
    // Update the doFirst items to reflect new count
    setDoFirstItems(prev => {
      const filtered = prev.filter(item => item.id !== 'pending-docs');
      if (count > 0) {
        return [{
          id: 'pending-docs',
          icon: '📄',
          title: `Review ${count} pending document${count > 1 ? 's' : ''}`,
          subtitle: 'Awaiting your review',
          actionLabel: 'Review →',
          onAction: () => setShowReviewPanel(true),
        }, ...filtered];
      }
      return filtered;
    });
    // Update AI suggestion if needed
    if (count > 0 && (!stats?.services_overdue || stats.services_overdue === 0)) {
      setAiSuggestion(`Review ${count} pending documents`);
    } else if (count === 0 && (!stats?.services_overdue || stats.services_overdue === 0)) {
      setAiSuggestion(null);
    }
  }, [stats?.services_overdue]);

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>📅</span> TODAY
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
          <span>📅</span> TODAY
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

        {/* 🔴 DO FIRST Section */}
        <div>
          <h3 className="text-sm font-semibold text-red-600 dark:text-red-400 mb-3 flex items-center gap-2">
            🔴 DO FIRST ({doFirstItems.length})
          </h3>
          {doFirstItems.length > 0 ? (
            <div className="space-y-2">
              {doFirstItems.map((item) => (
                <div
                  key={item.id}
                  className="flex items-center justify-between p-3 bg-gray-50 dark:bg-slate-700 rounded-lg border border-gray-200 dark:border-gray-600"
                >
                  <div className="flex items-start gap-3 min-w-0 flex-1">
                    <span className="text-lg">{item.icon}</span>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {item.title}
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        {item.subtitle}
                      </p>
                    </div>
                  </div>
                  {item.onAction ? (
                    <button
                      onClick={item.onAction}
                      className="px-3 py-1.5 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded whitespace-nowrap"
                    >
                      {item.actionLabel}
                    </button>
                  ) : item.actionHref && (
                    <Link
                      href={item.actionHref}
                      className="px-3 py-1.5 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded whitespace-nowrap"
                    >
                      {item.actionLabel}
                    </Link>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400 italic">No urgent tasks</p>
          )}
        </div>

        {/* 🟡 LATER TODAY Section */}
        <div>
          <h3 className="text-sm font-semibold text-yellow-600 dark:text-yellow-400 mb-3 flex items-center gap-2">
            🟡 LATER TODAY ({laterTodayItems.length})
          </h3>
          {laterTodayItems.length > 0 ? (
            <ul className="space-y-1.5">
              {laterTodayItems.map((item) => (
                <li key={item.id} className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                  <span className="text-gray-400">•</span>
                  <Link
                    href={item.actionHref || '#'}
                    className="hover:text-blue-600 dark:hover:text-blue-400"
                  >
                    {item.title} - {item.subtitle}
                  </Link>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400 italic">Nothing scheduled for later</p>
          )}
        </div>

        {/* 🟢 AI SUGGESTION Section */}
        <div>
          <h3 className="text-sm font-semibold text-green-600 dark:text-green-400 mb-3 flex items-center gap-2">
            🟢 AI SUGGESTION
          </h3>
          {aiSuggestion ? (
            <div className="flex items-center justify-between p-3 bg-green-50 dark:bg-green-900/20 rounded-lg border border-green-200 dark:border-green-800">
              <p className="text-sm text-gray-700 dark:text-gray-300">
                "{aiSuggestion}"
              </p>
              <button
                onClick={handleAiAction}
                className="px-3 py-1.5 text-xs font-medium text-white bg-green-600 hover:bg-green-700 rounded"
              >
                Do it
              </button>
            </div>
          ) : (
            <div className="p-3 bg-green-50 dark:bg-green-900/20 rounded-lg border border-green-200 dark:border-green-800">
              <p className="text-sm text-gray-600 dark:text-gray-400">
                ✨ All caught up! No suggestions right now.
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Pending Review Panel */}
      <PendingReviewPanel
        isOpen={showReviewPanel}
        onClose={() => setShowReviewPanel(false)}
        onCountChange={handlePendingCountChange}
      />
    </div>
  );
}

export default TodayPanel;
