'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getServices, Service } from '@/lib/api';

interface DeadlineItem {
  id: string;
  clientName: string;
  serviceName: string;
  type: 'VAT' | 'CT600' | 'SA' | 'Other';
  deadline: string;
  daysUntil: number;
  status: string;
}

interface MonthDeadlines {
  month: string;
  year: number;
  items: DeadlineItem[];
}

export function HMRCDeadlinesPanel() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deadlinesByMonth, setDeadlinesByMonth] = useState<MonthDeadlines[]>([]);
  const [view, setView] = useState<'list' | 'calendar'>('list');

  useEffect(() => {
    async function fetchDeadlines() {
      try {
        setLoading(true);
        const data = await getServices({ limit: 200 });
        const services = data.services || [];

        const now = new Date();

        // Filter for HMRC-relevant services with deadlines
        const hmrcServices = services
          .filter((s: Service) => {
            if (!s.deadline || s.status === 'completed' || s.status === 'cancelled') return false;
            const name = s.name.toLowerCase();
            return name.includes('vat') || name.includes('ct600') ||
                   name.includes('self assessment') || name.includes('tax return') ||
                   name.includes('corporation tax');
          })
          .map((s: Service) => {
            const deadline = new Date(s.deadline!);
            const daysUntil = Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
            const name = s.name.toLowerCase();
            let type: 'VAT' | 'CT600' | 'SA' | 'Other' = 'Other';
            if (name.includes('vat')) type = 'VAT';
            else if (name.includes('ct600') || name.includes('corporation tax')) type = 'CT600';
            else if (name.includes('self assessment') || (name.includes('tax return') && !name.includes('corporation'))) type = 'SA';

            return {
              id: s.id,
              clientName: s.client_name || 'Unknown Client',
              serviceName: s.name,
              type,
              deadline: s.deadline!,
              daysUntil,
              status: s.status,
            };
          })
          .sort((a, b) => new Date(a.deadline).getTime() - new Date(b.deadline).getTime());

        // Group by month
        const monthGroups: Record<string, DeadlineItem[]> = {};
        hmrcServices.forEach(item => {
          const date = new Date(item.deadline);
          const key = `${date.getFullYear()}-${date.getMonth()}`;
          if (!monthGroups[key]) {
            monthGroups[key] = [];
          }
          monthGroups[key].push(item);
        });

        const grouped: MonthDeadlines[] = Object.entries(monthGroups)
          .map(([key, items]) => {
            const [year, month] = key.split('-').map(Number);
            const date = new Date(year, month);
            return {
              month: date.toLocaleDateString('en-GB', { month: 'long' }),
              year,
              items,
            };
          })
          .sort((a, b) => {
            const dateA = new Date(a.year, new Date(`${a.month} 1`).getMonth());
            const dateB = new Date(b.year, new Date(`${b.month} 1`).getMonth());
            return dateA.getTime() - dateB.getTime();
          });

        setDeadlinesByMonth(grouped);
        setError(null);
      } catch (err) {
        setError('Failed to load HMRC deadlines');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchDeadlines();
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

  const totalDeadlines = deadlinesByMonth.reduce((acc, m) => acc + m.items.length, 0);

  return (
    <div className="h-full overflow-y-auto">
      {/* Header */}
      <div className="sticky top-0 bg-gray-50 dark:bg-slate-900 p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h3 className="font-semibold text-gray-900 dark:text-white">
            HMRC Deadlines ({totalDeadlines})
          </h3>
          <div className="flex rounded-lg border border-gray-200 dark:border-gray-600 overflow-hidden">
            <button
              onClick={() => setView('list')}
              className={`px-3 py-1.5 text-sm font-medium ${
                view === 'list'
                  ? 'bg-blue-600 text-white'
                  : 'bg-white dark:bg-slate-800 text-gray-700 dark:text-gray-300'
              }`}
            >
              List
            </button>
            <button
              onClick={() => setView('calendar')}
              className={`px-3 py-1.5 text-sm font-medium ${
                view === 'calendar'
                  ? 'bg-blue-600 text-white'
                  : 'bg-white dark:bg-slate-800 text-gray-700 dark:text-gray-300'
              }`}
            >
              Calendar
            </button>
          </div>
        </div>
        <div className="flex items-center gap-4">
          <TypeLegend />
        </div>
      </div>

      {/* Content */}
      <div className="p-4">
        {totalDeadlines === 0 ? (
          <div className="text-center py-12">
            <span className="text-4xl">✨</span>
            <p className="mt-2 text-gray-600 dark:text-gray-400">No upcoming HMRC deadlines</p>
          </div>
        ) : view === 'list' ? (
          <div className="space-y-6">
            {deadlinesByMonth.map((monthGroup) => (
              <div key={`${monthGroup.month}-${monthGroup.year}`}>
                <h4 className="text-sm font-semibold text-gray-500 dark:text-gray-400 uppercase mb-3">
                  {monthGroup.month} {monthGroup.year}
                </h4>
                <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-gray-700 divide-y divide-gray-200 dark:divide-gray-700">
                  {monthGroup.items.map((item) => (
                    <DeadlineRow key={item.id} item={item} />
                  ))}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <CalendarView deadlinesByMonth={deadlinesByMonth} />
        )}
      </div>

      {/* Key Dates Reference */}
      <div className="p-4">
        <div className="bg-gray-50 dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
          <h4 className="font-semibold text-gray-900 dark:text-white mb-3">Key HMRC Dates</h4>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="text-center p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
              <p className="text-xs text-blue-600 dark:text-blue-400 font-medium">VAT Quarterly</p>
              <p className="text-sm text-gray-900 dark:text-white mt-1">7th of 2nd month after quarter end</p>
            </div>
            <div className="text-center p-3 bg-purple-50 dark:bg-purple-900/20 rounded-lg">
              <p className="text-xs text-purple-600 dark:text-purple-400 font-medium">CT600</p>
              <p className="text-sm text-gray-900 dark:text-white mt-1">12 months after accounting period</p>
            </div>
            <div className="text-center p-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
              <p className="text-xs text-green-600 dark:text-green-400 font-medium">Self Assessment</p>
              <p className="text-sm text-gray-900 dark:text-white mt-1">31st January (online)</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function TypeLegend() {
  return (
    <div className="flex items-center gap-3 text-xs">
      <span className="flex items-center gap-1">
        <span className="w-3 h-3 rounded-full bg-blue-500"></span>
        VAT
      </span>
      <span className="flex items-center gap-1">
        <span className="w-3 h-3 rounded-full bg-purple-500"></span>
        CT600
      </span>
      <span className="flex items-center gap-1">
        <span className="w-3 h-3 rounded-full bg-green-500"></span>
        SA
      </span>
    </div>
  );
}

function DeadlineRow({ item }: { item: DeadlineItem }) {
  const getTypeColor = () => {
    switch (item.type) {
      case 'VAT': return 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300';
      case 'CT600': return 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300';
      case 'SA': return 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300';
      default: return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300';
    }
  };

  const getUrgencyIndicator = () => {
    if (item.daysUntil < 0) return '🔴';
    if (item.daysUntil <= 7) return '🟠';
    if (item.daysUntil <= 30) return '🟡';
    return '🟢';
  };

  const formatDeadline = () => {
    const date = new Date(item.deadline);
    return date.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' });
  };

  const formatDaysUntil = () => {
    if (item.daysUntil < 0) return `${Math.abs(item.daysUntil)}d overdue`;
    if (item.daysUntil === 0) return 'Today';
    if (item.daysUntil === 1) return 'Tomorrow';
    return `${item.daysUntil}d`;
  };

  return (
    <div className="flex items-center justify-between p-4 hover:bg-gray-50 dark:hover:bg-slate-700">
      <div className="flex items-center gap-3">
        <span>{getUrgencyIndicator()}</span>
        <span className={`px-2 py-1 text-xs font-medium rounded ${getTypeColor()}`}>
          {item.type}
        </span>
        <div>
          <p className="text-sm font-medium text-gray-900 dark:text-white">{item.clientName}</p>
          <p className="text-xs text-gray-500 dark:text-gray-400">{item.serviceName}</p>
        </div>
      </div>
      <div className="flex items-center gap-4">
        <div className="text-right">
          <p className="text-sm text-gray-900 dark:text-white">{formatDeadline()}</p>
          <p className={`text-xs ${
            item.daysUntil < 0 ? 'text-red-600 dark:text-red-400' :
            item.daysUntil <= 7 ? 'text-orange-600 dark:text-orange-400' :
            'text-gray-500 dark:text-gray-400'
          }`}>
            {formatDaysUntil()}
          </p>
        </div>
        <Link
          href={`/dashboard/services/${item.id}`}
          className="px-3 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded"
        >
          View
        </Link>
      </div>
    </div>
  );
}

function CalendarView({ deadlinesByMonth }: { deadlinesByMonth: MonthDeadlines[] }) {
  const [currentMonth, setCurrentMonth] = useState(new Date());

  const monthNames = ['January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December'];
  const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

  const getDaysInMonth = (date: Date) => {
    const year = date.getFullYear();
    const month = date.getMonth();
    const firstDay = new Date(year, month, 1);
    const lastDay = new Date(year, month + 1, 0);
    const daysInMonth = lastDay.getDate();

    // Adjust for Monday start (0 = Monday, 6 = Sunday)
    let startDay = firstDay.getDay() - 1;
    if (startDay < 0) startDay = 6;

    return { daysInMonth, startDay };
  };

  const getDeadlinesForDay = (day: number) => {
    const year = currentMonth.getFullYear();
    const month = currentMonth.getMonth();
    const dateStr = new Date(year, month, day).toDateString();

    return deadlinesByMonth
      .flatMap(m => m.items)
      .filter(item => new Date(item.deadline).toDateString() === dateStr);
  };

  const { daysInMonth, startDay } = getDaysInMonth(currentMonth);
  const today = new Date();

  const prevMonth = () => {
    setCurrentMonth(new Date(currentMonth.getFullYear(), currentMonth.getMonth() - 1));
  };

  const nextMonth = () => {
    setCurrentMonth(new Date(currentMonth.getFullYear(), currentMonth.getMonth() + 1));
  };

  return (
    <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-gray-700">
      {/* Calendar Header */}
      <div className="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
        <button
          onClick={prevMonth}
          className="p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded"
        >
          ←
        </button>
        <h3 className="font-semibold text-gray-900 dark:text-white">
          {monthNames[currentMonth.getMonth()]} {currentMonth.getFullYear()}
        </h3>
        <button
          onClick={nextMonth}
          className="p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded"
        >
          →
        </button>
      </div>

      {/* Day Names */}
      <div className="grid grid-cols-7 border-b border-gray-200 dark:border-gray-700">
        {dayNames.map(day => (
          <div key={day} className="p-2 text-center text-xs font-medium text-gray-500 dark:text-gray-400">
            {day}
          </div>
        ))}
      </div>

      {/* Calendar Grid */}
      <div className="grid grid-cols-7">
        {/* Empty cells for start offset */}
        {Array.from({ length: startDay }).map((_, i) => (
          <div key={`empty-${i}`} className="p-2 min-h-24 border-b border-r border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-slate-900"></div>
        ))}

        {/* Day cells */}
        {Array.from({ length: daysInMonth }).map((_, i) => {
          const day = i + 1;
          const deadlines = getDeadlinesForDay(day);
          const isToday = today.getDate() === day &&
                          today.getMonth() === currentMonth.getMonth() &&
                          today.getFullYear() === currentMonth.getFullYear();

          return (
            <div
              key={day}
              className={`p-2 min-h-24 border-b border-r border-gray-100 dark:border-gray-700 ${
                isToday ? 'bg-blue-50 dark:bg-blue-900/20' : ''
              }`}
            >
              <div className={`text-sm ${isToday ? 'font-bold text-blue-600' : 'text-gray-900 dark:text-white'}`}>
                {day}
              </div>
              <div className="mt-1 space-y-1">
                {deadlines.slice(0, 3).map(d => (
                  <Link
                    key={d.id}
                    href={`/dashboard/services/${d.id}`}
                    className={`block text-xs truncate px-1 py-0.5 rounded ${
                      d.type === 'VAT' ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300' :
                      d.type === 'CT600' ? 'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300' :
                      'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
                    }`}
                  >
                    {d.clientName}
                  </Link>
                ))}
                {deadlines.length > 3 && (
                  <div className="text-xs text-gray-500">+{deadlines.length - 3} more</div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default HMRCDeadlinesPanel;
