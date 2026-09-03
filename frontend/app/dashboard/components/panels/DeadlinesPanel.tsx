'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getServices, Service } from '@/lib/api';

interface DeadlineItem {
  id: string;
  clientName: string;
  type: string;
  daysDisplay: string;
  dateDisplay: string;
  daysUntil: number;
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

          // Determine service type
          const type = service.name.toLowerCase().includes('vat') ? 'VAT' :
                      service.name.toLowerCase().includes('ct600') ? 'Tax' :
                      service.name.toLowerCase().includes('sa') ? 'SA' :
                      service.name.toLowerCase().includes('account') ? 'Accounts' :
                      service.name.toLowerCase().includes('id') ? 'ID' : 'Service';

          const item: DeadlineItem = {
            id: service.id,
            clientName: service.client_name || 'Unknown',
            type,
            daysUntil,
            daysDisplay: `${Math.abs(daysUntil)} days`,
            dateDisplay: deadline.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric' }),
          };

          if (deadline < now) {
            overdueItems.push(item);
          } else if (deadline <= weekFromNow) {
            thisWeekItems.push(item);
          } else if (deadline <= monthFromNow) {
            next30DaysItems.push(item);
          }
        });

        // Sort by deadline (most urgent first)
        const sortByDays = (a: DeadlineItem, b: DeadlineItem) => a.daysUntil - b.daysUntil;

        setOverdue(overdueItems.sort(sortByDays));
        setThisWeek(thisWeekItems.sort(sortByDays));
        setNext30Days(next30DaysItems.sort(sortByDays));
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
    window.location.href = `/dashboard/services/${serviceId}`;
  };

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>📅</span> DEADLINES
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
          <span>📅</span> DEADLINES
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

        {/* 🔴 OVERDUE Section */}
        <div>
          <h3 className="text-sm font-semibold text-red-600 dark:text-red-400 mb-3">
            🔴 OVERDUE ({overdue.length})
          </h3>
          {overdue.length > 0 ? (
            <div className="space-y-1">
              {overdue.map((item) => (
                <div key={item.id} className="flex items-center justify-between py-1.5">
                  <span className="text-sm text-gray-700 dark:text-gray-300">
                    {item.clientName} - {item.type}
                  </span>
                  <div className="flex items-center gap-3">
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                      {item.daysDisplay}
                    </span>
                    <button
                      onClick={() => handleChase(item.id)}
                      className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                    >
                      [Chase]
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400 italic">None</p>
          )}
        </div>

        {/* 🟡 THIS WEEK Section */}
        <div>
          <h3 className="text-sm font-semibold text-yellow-600 dark:text-yellow-400 mb-3">
            🟡 THIS WEEK ({thisWeek.length})
          </h3>
          {thisWeek.length > 0 ? (
            <div className="space-y-1">
              {thisWeek.map((item) => (
                <div key={item.id} className="flex items-center justify-between py-1.5">
                  <span className="text-sm text-gray-700 dark:text-gray-300">
                    {item.clientName} - {item.type}
                  </span>
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    {item.dateDisplay}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400 italic">None</p>
          )}
        </div>

        {/* 🟢 NEXT 30 DAYS Section */}
        <div>
          <h3 className="text-sm font-semibold text-green-600 dark:text-green-400 mb-3">
            🟢 NEXT 30 DAYS ({next30Days.length})
          </h3>
          {next30Days.length > 0 ? (
            <div className="space-y-1">
              {next30Days.map((item) => (
                <div key={item.id} className="flex items-center justify-between py-1.5">
                  <span className="text-sm text-gray-700 dark:text-gray-300">
                    {item.clientName} - {item.type}
                  </span>
                  <span className="text-sm text-gray-500 dark:text-gray-400">
                    {new Date(item.daysUntil * 24 * 60 * 60 * 1000 + Date.now()).toLocaleDateString('en-GB', { month: 'short', day: 'numeric' })}
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400 italic">None</p>
          )}
        </div>
      </div>
    </div>
  );
}

export default DeadlinesPanel;
