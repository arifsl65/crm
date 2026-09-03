'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getServices, Service } from '@/lib/api';

interface DeadlineItem {
  id: string;
  name: string;
  clientName: string;
  deadline: string;
  daysUntil: number;
  status: string;
  type: string;
}

interface DeadlinesPanelProps {
  onClose?: () => void;
}

export function DeadlinesPanel({ onClose }: DeadlinesPanelProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [overdue, setOverdue] = useState<DeadlineItem[]>([]);
  const [thisWeek, setThisWeek] = useState<DeadlineItem[]>([]);
  const [next30Days, setNext30Days] = useState<DeadlineItem[]>([]);

  useEffect(() => {
    async function fetchDeadlines() {
      try {
        setLoading(true);
        const data = await getServices({ limit: 100 });
        const services = data.services || [];

        const now = new Date();
        const weekFromNow = new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000);
        const monthFromNow = new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000);

        const overdueItems: DeadlineItem[] = [];
        const thisWeekItems: DeadlineItem[] = [];
        const next30DaysItems: DeadlineItem[] = [];

        services.forEach((service: Service) => {
          if (!service.deadline || service.status === 'completed' || service.status === 'cancelled') return;

          const deadline = new Date(service.deadline);
          const daysUntil = Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));

          const item: DeadlineItem = {
            id: service.id,
            name: service.name,
            clientName: service.client_name || 'Unknown Client',
            deadline: service.deadline,
            daysUntil,
            status: service.status,
            type: service.name.toLowerCase().includes('vat') ? 'VAT' :
                  service.name.toLowerCase().includes('ct600') ? 'CT600' :
                  service.name.toLowerCase().includes('sa') ? 'SA' : 'Service',
          };

          if (deadline < now) {
            overdueItems.push(item);
          } else if (deadline <= weekFromNow) {
            thisWeekItems.push(item);
          } else if (deadline <= monthFromNow) {
            next30DaysItems.push(item);
          }
        });

        // Sort by deadline
        const sortByDeadline = (a: DeadlineItem, b: DeadlineItem) => a.daysUntil - b.daysUntil;

        setOverdue(overdueItems.sort(sortByDeadline));
        setThisWeek(thisWeekItems.sort(sortByDeadline));
        setNext30Days(next30DaysItems.sort(sortByDeadline));
        setError(null);
      } catch (err) {
        setError('Failed to load deadlines');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchDeadlines();
  }, []);

  const handleChase = (serviceId: string) => {
    // TODO: Implement chase functionality
    console.log('Chase service:', serviceId);
  };

  const formatDeadline = (deadline: string, daysUntil: number) => {
    if (daysUntil < 0) {
      return `${Math.abs(daysUntil)} day${Math.abs(daysUntil) !== 1 ? 's' : ''} overdue`;
    }
    if (daysUntil === 0) return 'Due today';
    if (daysUntil === 1) return 'Due tomorrow';

    const date = new Date(deadline);
    const dayName = date.toLocaleDateString('en-GB', { weekday: 'short' });
    const dayNum = date.getDate();
    return `${dayName} ${dayNum}`;
  };

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>📆</span> Deadlines
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

  const totalDeadlines = overdue.length + thisWeek.length + next30Days.length;

  return (
    <div className="h-full flex flex-col bg-white dark:bg-slate-800 rounded-lg">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>📆</span> Deadlines
          {totalDeadlines > 0 && (
            <span className="text-sm font-normal text-gray-500 dark:text-gray-400">
              ({totalDeadlines})
            </span>
          )}
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

        {totalDeadlines === 0 && !error && (
          <div className="text-center py-8">
            <span className="text-4xl">✅</span>
            <p className="mt-2 text-gray-600 dark:text-gray-400">No upcoming deadlines!</p>
          </div>
        )}

        {/* Overdue Section */}
        {overdue.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-red-600 dark:text-red-400 mb-3 flex items-center gap-2">
              🔴 OVERDUE ({overdue.length})
            </h3>
            <div className="space-y-2">
              {overdue.map((item) => (
                <DeadlineRow
                  key={item.id}
                  item={item}
                  urgency="overdue"
                  formatDeadline={formatDeadline}
                  onChase={handleChase}
                />
              ))}
            </div>
          </div>
        )}

        {/* This Week Section */}
        {thisWeek.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-yellow-600 dark:text-yellow-400 mb-3 flex items-center gap-2">
              🟡 THIS WEEK ({thisWeek.length})
            </h3>
            <div className="space-y-2">
              {thisWeek.map((item) => (
                <DeadlineRow
                  key={item.id}
                  item={item}
                  urgency="soon"
                  formatDeadline={formatDeadline}
                  onChase={handleChase}
                />
              ))}
            </div>
          </div>
        )}

        {/* Next 30 Days Section */}
        {next30Days.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-green-600 dark:text-green-400 mb-3 flex items-center gap-2">
              🟢 NEXT 30 DAYS ({next30Days.length})
            </h3>
            <div className="space-y-2">
              {next30Days.map((item) => (
                <DeadlineRow
                  key={item.id}
                  item={item}
                  urgency="upcoming"
                  formatDeadline={formatDeadline}
                  onChase={handleChase}
                />
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700">
        <Link
          href="/dashboard/services"
          className="w-full flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
        >
          View All Services →
        </Link>
      </div>
    </div>
  );
}

function DeadlineRow({
  item,
  urgency,
  formatDeadline,
  onChase,
}: {
  item: DeadlineItem;
  urgency: 'overdue' | 'soon' | 'upcoming';
  formatDeadline: (deadline: string, daysUntil: number) => string;
  onChase: (id: string) => void;
}) {
  const bgColor = urgency === 'overdue'
    ? 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'
    : urgency === 'soon'
    ? 'bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-800'
    : 'bg-gray-50 dark:bg-slate-700 border-gray-200 dark:border-gray-600';

  return (
    <div className={`flex items-center justify-between p-3 rounded-lg border ${bgColor}`}>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="text-xs font-medium px-1.5 py-0.5 rounded bg-gray-200 dark:bg-slate-600 text-gray-700 dark:text-gray-300">
            {item.type}
          </span>
          <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
            {item.clientName}
          </p>
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
          {item.name}
        </p>
      </div>
      <div className="flex items-center gap-2 ml-3">
        <span className={`text-xs font-medium ${
          urgency === 'overdue' ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'
        }`}>
          {formatDeadline(item.deadline, item.daysUntil)}
        </span>
        {urgency !== 'upcoming' && (
          <button
            onClick={() => onChase(item.id)}
            className="px-2 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded"
          >
            Chase
          </button>
        )}
      </div>
    </div>
  );
}

export default DeadlinesPanel;
