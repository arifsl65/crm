'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getServices, Service } from '@/lib/api';

interface HMRCStats {
  totalFilings: number;
  pendingFilings: number;
  overdueFilings: number;
  completedThisYear: number;
  vatReturns: { total: number; pending: number; overdue: number };
  ct600: { total: number; pending: number; overdue: number };
  selfAssessment: { total: number; pending: number; overdue: number };
  upcomingDeadlines: DeadlineItem[];
}

interface DeadlineItem {
  id: string;
  clientName: string;
  type: 'VAT' | 'CT600' | 'SA';
  deadline: string;
  daysUntil: number;
  status: string;
}

export function HMRCOverviewPanel() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [stats, setStats] = useState<HMRCStats | null>(null);

  useEffect(() => {
    async function fetchHMRCData() {
      try {
        setLoading(true);
        const data = await getServices({ limit: 200 });
        const services = data.services || [];

        // Filter for HMRC-relevant services
        const hmrcServices = services.filter((s: Service) => {
          const name = s.name.toLowerCase();
          return name.includes('vat') || name.includes('ct600') ||
                 name.includes('self assessment') || name.includes('tax return') ||
                 name.includes('corporation tax');
        });

        const now = new Date();
        const startOfYear = new Date(now.getFullYear(), 0, 1);

        // Categorize services
        const categorize = (filter: (s: Service) => boolean) => {
          const filtered = hmrcServices.filter(filter);
          return {
            total: filtered.length,
            pending: filtered.filter(s => s.status === 'in_progress' || s.status === 'not_started').length,
            overdue: filtered.filter(s => {
              if (!s.deadline || s.status === 'completed') return false;
              return new Date(s.deadline) < now;
            }).length,
          };
        };

        const vatStats = categorize(s => s.name.toLowerCase().includes('vat'));
        const ct600Stats = categorize(s =>
          s.name.toLowerCase().includes('ct600') || s.name.toLowerCase().includes('corporation tax')
        );
        const saStats = categorize(s =>
          s.name.toLowerCase().includes('self assessment') ||
          (s.name.toLowerCase().includes('tax return') && !s.name.toLowerCase().includes('corporation'))
        );

        // Get upcoming deadlines (next 30 days)
        const upcomingDeadlines: DeadlineItem[] = hmrcServices
          .filter(s => {
            if (!s.deadline || s.status === 'completed' || s.status === 'cancelled') return false;
            const deadline = new Date(s.deadline);
            const daysUntil = Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
            return daysUntil >= -30 && daysUntil <= 30;
          })
          .map(s => {
            const deadline = new Date(s.deadline!);
            const daysUntil = Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
            const name = s.name.toLowerCase();
            let type: 'VAT' | 'CT600' | 'SA' = 'VAT';
            if (name.includes('ct600') || name.includes('corporation tax')) type = 'CT600';
            else if (name.includes('self assessment') || (name.includes('tax return') && !name.includes('corporation'))) type = 'SA';

            return {
              id: s.id,
              clientName: s.client_name || 'Unknown Client',
              type,
              deadline: s.deadline!,
              daysUntil,
              status: s.status,
            };
          })
          .sort((a, b) => a.daysUntil - b.daysUntil)
          .slice(0, 10);

        const completedThisYear = hmrcServices.filter(s => {
          if (s.status !== 'completed') return false;
          const completedAt = s.updated_at ? new Date(s.updated_at) : null;
          return completedAt && completedAt >= startOfYear;
        }).length;

        setStats({
          totalFilings: hmrcServices.length,
          pendingFilings: vatStats.pending + ct600Stats.pending + saStats.pending,
          overdueFilings: vatStats.overdue + ct600Stats.overdue + saStats.overdue,
          completedThisYear,
          vatReturns: vatStats,
          ct600: ct600Stats,
          selfAssessment: saStats,
          upcomingDeadlines,
        });
        setError(null);
      } catch (err) {
        setError('Failed to load HMRC data');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchHMRCData();
  }, []);

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-center">
          <span className="text-4xl">⚠️</span>
          <p className="mt-2 text-gray-600 dark:text-gray-400">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto p-6 space-y-6">
      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <SummaryCard
          icon="📊"
          label="Total Filings"
          value={stats?.totalFilings || 0}
          color="blue"
        />
        <SummaryCard
          icon="⏳"
          label="Pending"
          value={stats?.pendingFilings || 0}
          color="yellow"
        />
        <SummaryCard
          icon="🔴"
          label="Overdue"
          value={stats?.overdueFilings || 0}
          color="red"
        />
        <SummaryCard
          icon="✅"
          label="Completed (YTD)"
          value={stats?.completedThisYear || 0}
          color="green"
        />
      </div>

      {/* Category Breakdown */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <CategoryCard
          title="VAT Returns"
          icon="📊"
          stats={stats?.vatReturns || { total: 0, pending: 0, overdue: 0 }}
          href="#vat"
        />
        <CategoryCard
          title="CT600"
          icon="🏢"
          stats={stats?.ct600 || { total: 0, pending: 0, overdue: 0 }}
          href="#ct600"
        />
        <CategoryCard
          title="Self Assessment"
          icon="👤"
          stats={stats?.selfAssessment || { total: 0, pending: 0, overdue: 0 }}
          href="#sa"
        />
      </div>

      {/* Upcoming Deadlines */}
      <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-gray-700">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700">
          <h3 className="font-semibold text-gray-900 dark:text-white">Upcoming Deadlines</h3>
        </div>
        <div className="divide-y divide-gray-200 dark:divide-gray-700">
          {stats?.upcomingDeadlines.length === 0 ? (
            <div className="p-8 text-center">
              <span className="text-4xl">✨</span>
              <p className="mt-2 text-gray-600 dark:text-gray-400">No upcoming HMRC deadlines</p>
            </div>
          ) : (
            stats?.upcomingDeadlines.map((item) => (
              <DeadlineRow key={item.id} item={item} />
            ))
          )}
        </div>
      </div>

      {/* Quick Actions */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <QuickActionButton icon="📊" label="File VAT Return" href="/dashboard/hmrc?tab=vat" />
        <QuickActionButton icon="🏢" label="File CT600" href="/dashboard/hmrc?tab=ct600" />
        <QuickActionButton icon="👤" label="File SA Return" href="/dashboard/hmrc?tab=sa" />
        <QuickActionButton icon="📅" label="View Calendar" href="/dashboard/hmrc?tab=deadlines" />
      </div>
    </div>
  );
}

function SummaryCard({
  icon,
  label,
  value,
  color,
}: {
  icon: string;
  label: string;
  value: number;
  color: 'blue' | 'yellow' | 'red' | 'green';
}) {
  const colorClasses = {
    blue: 'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800 text-blue-600 dark:text-blue-400',
    yellow: 'bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-800 text-yellow-600 dark:text-yellow-400',
    red: 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800 text-red-600 dark:text-red-400',
    green: 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800 text-green-600 dark:text-green-400',
  };

  return (
    <div className={`p-4 rounded-lg border ${colorClasses[color]}`}>
      <div className="flex items-center gap-2 mb-2">
        <span className="text-xl">{icon}</span>
        <span className="text-sm font-medium text-gray-600 dark:text-gray-400">{label}</span>
      </div>
      <p className="text-2xl font-bold">{value}</p>
    </div>
  );
}

function CategoryCard({
  title,
  icon,
  stats,
  href,
}: {
  title: string;
  icon: string;
  stats: { total: number; pending: number; overdue: number };
  href: string;
}) {
  return (
    <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>{icon}</span> {title}
        </h3>
        <Link href={href} className="text-sm text-blue-600 dark:text-blue-400 hover:underline">
          View All
        </Link>
      </div>
      <div className="grid grid-cols-3 gap-2 text-center">
        <div>
          <p className="text-xl font-bold text-gray-900 dark:text-white">{stats.total}</p>
          <p className="text-xs text-gray-500 dark:text-gray-400">Total</p>
        </div>
        <div>
          <p className="text-xl font-bold text-yellow-600 dark:text-yellow-400">{stats.pending}</p>
          <p className="text-xs text-gray-500 dark:text-gray-400">Pending</p>
        </div>
        <div>
          <p className="text-xl font-bold text-red-600 dark:text-red-400">{stats.overdue}</p>
          <p className="text-xs text-gray-500 dark:text-gray-400">Overdue</p>
        </div>
      </div>
    </div>
  );
}

function DeadlineRow({ item }: { item: DeadlineItem }) {
  const getUrgencyColor = () => {
    if (item.daysUntil < 0) return 'bg-red-50 dark:bg-red-900/20';
    if (item.daysUntil <= 7) return 'bg-orange-50 dark:bg-orange-900/20';
    return 'bg-gray-50 dark:bg-slate-700';
  };

  const getTypeColor = () => {
    switch (item.type) {
      case 'VAT': return 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300';
      case 'CT600': return 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300';
      case 'SA': return 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300';
    }
  };

  const formatDeadline = () => {
    if (item.daysUntil < 0) return `${Math.abs(item.daysUntil)} days overdue`;
    if (item.daysUntil === 0) return 'Due today';
    if (item.daysUntil === 1) return 'Due tomorrow';
    return `${item.daysUntil} days left`;
  };

  return (
    <div className={`flex items-center justify-between p-4 ${getUrgencyColor()}`}>
      <div className="flex items-center gap-3">
        <span className={`px-2 py-1 text-xs font-medium rounded ${getTypeColor()}`}>
          {item.type}
        </span>
        <div>
          <p className="text-sm font-medium text-gray-900 dark:text-white">{item.clientName}</p>
          <p className="text-xs text-gray-500 dark:text-gray-400">
            {new Date(item.deadline).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <span className={`text-sm font-medium ${
          item.daysUntil < 0 ? 'text-red-600 dark:text-red-400' :
          item.daysUntil <= 7 ? 'text-orange-600 dark:text-orange-400' :
          'text-gray-600 dark:text-gray-400'
        }`}>
          {formatDeadline()}
        </span>
        <Link
          href={`/dashboard/services/${item.id}`}
          className="px-3 py-1 text-sm font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded"
        >
          View
        </Link>
      </div>
    </div>
  );
}

function QuickActionButton({ icon, label, href }: { icon: string; label: string; href: string }) {
  return (
    <Link
      href={href}
      className="flex items-center gap-2 p-3 bg-white dark:bg-slate-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-700"
    >
      <span className="text-xl">{icon}</span>
      <span className="text-sm font-medium text-gray-700 dark:text-gray-300">{label}</span>
    </Link>
  );
}

export default HMRCOverviewPanel;
