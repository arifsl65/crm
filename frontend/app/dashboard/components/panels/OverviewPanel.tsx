'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getDashboardStats, getServices, getUsers, DashboardStats, Service } from '@/lib/api';

interface OverviewPanelProps {
  onClose?: () => void;
}

interface PriorityTask {
  id: string;
  title: string;
  status: string;
}

export function OverviewPanel({ onClose }: OverviewPanelProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [staffCount, setStaffCount] = useState(0);
  const [priorityTasks, setPriorityTasks] = useState<PriorityTask[]>([]);

  useEffect(() => {
    async function fetchData() {
      try {
        setLoading(true);
        const [dashboardData, servicesData, usersData] = await Promise.all([
          getDashboardStats(),
          getServices({ limit: 50 }),
          getUsers().catch(() => ({ users: [] })),
        ]);

        setStats(dashboardData);
        setStaffCount(usersData.users?.length || 0);

        // Build priority tasks from services
        const services = servicesData.services || [];
        const now = new Date();
        const tasks: PriorityTask[] = [];

        services.forEach((service: Service) => {
          if (!service.deadline || service.status === 'completed' || service.status === 'cancelled') return;

          const deadline = new Date(service.deadline);
          const daysUntil = Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));

          if (daysUntil <= 7) {
            let status = '';
            if (daysUntil < 0) {
              status = `${Math.abs(daysUntil)} days overdue`;
            } else if (daysUntil === 0) {
              status = 'due today';
            } else if (daysUntil === 1) {
              status = 'due tomorrow';
            } else {
              status = `due in ${daysUntil} days`;
            }

            tasks.push({
              id: service.id,
              title: `${service.client_name} ${service.name}`,
              status,
            });
          }
        });

        // Sort by urgency (overdue first, then by days)
        tasks.sort((a, b) => {
          const aOverdue = a.status.includes('overdue');
          const bOverdue = b.status.includes('overdue');
          if (aOverdue && !bOverdue) return -1;
          if (!aOverdue && bOverdue) return 1;
          return 0;
        });

        setPriorityTasks(tasks.slice(0, 3));
        setError(null);
      } catch (err) {
        setError('Failed to load overview');
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
            <span>📊</span> OVERVIEW
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
          <span>📊</span> OVERVIEW
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

        {/* Stats Cards - 3 cards: Clients, Staff, Pending */}
        <div className="grid grid-cols-3 gap-3">
          <div className="text-center p-4 bg-gray-50 dark:bg-slate-700 rounded-lg">
            <p className="text-3xl font-bold text-blue-600 dark:text-blue-400">
              {stats?.total_clients || 0}
            </p>
            <p className="text-sm text-gray-600 dark:text-gray-400">Clients</p>
          </div>
          <div className="text-center p-4 bg-gray-50 dark:bg-slate-700 rounded-lg">
            <p className="text-3xl font-bold text-green-600 dark:text-green-400">
              {staffCount}
            </p>
            <p className="text-sm text-gray-600 dark:text-gray-400">Staff</p>
          </div>
          <div className="text-center p-4 bg-gray-50 dark:bg-slate-700 rounded-lg">
            <p className="text-3xl font-bold text-orange-600 dark:text-orange-400">
              {stats?.documents_pending || 0}
            </p>
            <p className="text-sm text-gray-600 dark:text-gray-400">Pending</p>
          </div>
        </div>

        {/* 📈 This Month Section */}
        <div>
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
            📈 This Month
          </h3>
          <ul className="space-y-2">
            <li className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <span className="text-gray-400">•</span>
              {stats?.active_clients || 0} active clients
            </li>
            <li className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <span className="text-gray-400">•</span>
              {stats?.documents_approved || 0} docs processed
            </li>
            <li className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <span className="text-gray-400">•</span>
              {calculateOnTimeRate(stats)}% on-time rate
            </li>
          </ul>
        </div>

        {/* 🔝 Priority Tasks Section */}
        <div>
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3 flex items-center gap-2">
            🔝 Priority Tasks
          </h3>
          {priorityTasks.length > 0 ? (
            <ol className="space-y-2">
              {priorityTasks.map((task, index) => (
                <li key={task.id} className="flex items-start gap-2 text-sm text-gray-700 dark:text-gray-300">
                  <span className="font-medium text-gray-500">{index + 1}.</span>
                  <Link
                    href={`/dashboard/services/${task.id}`}
                    className="hover:text-blue-600 dark:hover:text-blue-400"
                  >
                    {task.title} ({task.status})
                  </Link>
                </li>
              ))}
            </ol>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400 italic">No priority tasks this week</p>
          )}
        </div>
      </div>
    </div>
  );
}

function calculateOnTimeRate(stats: DashboardStats | null): number {
  if (!stats) return 0;
  const total = (stats.services_completed || 0) + (stats.services_overdue || 0);
  if (total === 0) return 100;
  return Math.round(((stats.services_completed || 0) / total) * 100);
}

export default OverviewPanel;
