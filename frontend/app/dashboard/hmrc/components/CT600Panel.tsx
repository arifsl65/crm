'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getServices, Service } from '@/lib/api';

interface CT600Return {
  id: string;
  clientName: string;
  clientId?: string;
  companyNumber?: string;
  accountingPeriod: string;
  deadline: string;
  daysUntil: number;
  status: string;
  docsReceived: number;
  docsRequired: number;
}

export function CT600Panel() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [ct600Returns, setCT600Returns] = useState<CT600Return[]>([]);
  const [filter, setFilter] = useState<'all' | 'pending' | 'overdue' | 'completed'>('all');

  useEffect(() => {
    async function fetchCT600Returns() {
      try {
        setLoading(true);
        const data = await getServices({ limit: 200 });
        const services = data.services || [];

        const now = new Date();

        // Filter for CT600/Corporation Tax services
        const ct600Services = services
          .filter((s: Service) => {
            const name = s.name.toLowerCase();
            return name.includes('ct600') || name.includes('corporation tax');
          })
          .map((s: Service) => {
            const deadline = s.deadline ? new Date(s.deadline) : null;
            const daysUntil = deadline
              ? Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
              : 999;

            // Try to extract accounting period from service name
            const yearMatch = s.name.match(/\d{4}/g);
            const accountingPeriod = yearMatch ? yearMatch.join('-') : 'N/A';

            return {
              id: s.id,
              clientName: s.client_name || 'Unknown Client',
              clientId: s.client_id,
              companyNumber: undefined,
              accountingPeriod,
              deadline: s.deadline || '',
              daysUntil,
              status: s.status,
              docsReceived: s.docs_received || 0,
              docsRequired: s.docs_required || 0,
            };
          })
          .sort((a, b) => a.daysUntil - b.daysUntil);

        setCT600Returns(ct600Services);
        setError(null);
      } catch (err) {
        setError('Failed to load CT600 returns');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchCT600Returns();
  }, []);

  const filteredReturns = ct600Returns.filter(r => {
    if (filter === 'all') return true;
    if (filter === 'pending') return r.status === 'in_progress' || r.status === 'not_started';
    if (filter === 'overdue') return r.daysUntil < 0 && r.status !== 'completed';
    if (filter === 'completed') return r.status === 'completed';
    return true;
  });

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
    <div className="h-full overflow-y-auto">
      {/* Filter Bar */}
      <div className="sticky top-0 bg-gray-50 dark:bg-slate-900 p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <div className="flex gap-2">
          {(['all', 'pending', 'overdue', 'completed'] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`px-3 py-1.5 text-sm font-medium rounded-lg ${
                filter === f
                  ? 'bg-purple-600 text-white'
                  : 'bg-white dark:bg-slate-800 text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-600'
              }`}
            >
              {f.charAt(0).toUpperCase() + f.slice(1)}
              {f !== 'all' && (
                <span className="ml-1 text-xs">
                  ({ct600Returns.filter(r => {
                    if (f === 'pending') return r.status === 'in_progress' || r.status === 'not_started';
                    if (f === 'overdue') return r.daysUntil < 0 && r.status !== 'completed';
                    if (f === 'completed') return r.status === 'completed';
                    return false;
                  }).length})
                </span>
              )}
            </button>
          ))}
        </div>
        <button className="px-4 py-2 bg-purple-600 text-white text-sm font-medium rounded-lg hover:bg-purple-700">
          + New CT600
        </button>
      </div>

      {/* CT600 Returns Table */}
      <div className="p-4">
        {filteredReturns.length === 0 ? (
          <div className="text-center py-12">
            <span className="text-4xl">🏢</span>
            <p className="mt-2 text-gray-600 dark:text-gray-400">No CT600 returns found</p>
          </div>
        ) : (
          <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="w-full">
              <thead className="bg-gray-50 dark:bg-slate-700">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Company</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Period</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Deadline</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Documents</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Status</th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {filteredReturns.map((ct600) => (
                  <CT600ReturnRow key={ct600.id} ct600={ct600} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* CT600 Info Card */}
      <div className="p-4">
        <div className="bg-purple-50 dark:bg-purple-900/20 rounded-lg p-4 border border-purple-200 dark:border-purple-800">
          <h4 className="font-medium text-purple-900 dark:text-purple-100 flex items-center gap-2">
            <span>ℹ️</span> CT600 Filing Reminder
          </h4>
          <p className="text-sm text-purple-700 dark:text-purple-300 mt-1">
            Corporation Tax returns must be filed within 12 months of the accounting period end date.
            Payment is due 9 months and 1 day after the accounting period ends.
          </p>
        </div>
      </div>
    </div>
  );
}

function CT600ReturnRow({ ct600 }: { ct600: CT600Return }) {
  const getStatusBadge = () => {
    if (ct600.status === 'completed') {
      return <span className="px-2 py-1 text-xs font-medium rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300">Filed</span>;
    }
    if (ct600.daysUntil < 0) {
      return <span className="px-2 py-1 text-xs font-medium rounded bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300">Overdue</span>;
    }
    if (ct600.daysUntil <= 30) {
      return <span className="px-2 py-1 text-xs font-medium rounded bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300">Due Soon</span>;
    }
    if (ct600.status === 'in_progress') {
      return <span className="px-2 py-1 text-xs font-medium rounded bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300">In Progress</span>;
    }
    return <span className="px-2 py-1 text-xs font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">Not Started</span>;
  };

  const formatDeadline = () => {
    if (!ct600.deadline) return 'No deadline';
    const date = new Date(ct600.deadline);
    const formatted = date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });

    if (ct600.status === 'completed') return formatted;
    if (ct600.daysUntil < 0) return `${formatted} (${Math.abs(ct600.daysUntil)}d overdue)`;
    if (ct600.daysUntil === 0) return `${formatted} (Today)`;
    return `${formatted} (${ct600.daysUntil}d)`;
  };

  const docsProgress = ct600.docsRequired > 0 ? Math.round((ct600.docsReceived / ct600.docsRequired) * 100) : 0;

  return (
    <tr className="hover:bg-gray-50 dark:hover:bg-slate-700">
      <td className="px-4 py-3">
        <Link href={`/dashboard/clients/${ct600.clientId}`} className="text-sm font-medium text-gray-900 dark:text-white hover:text-purple-600">
          {ct600.clientName}
        </Link>
      </td>
      <td className="px-4 py-3 text-sm text-gray-600 dark:text-gray-400">
        {ct600.accountingPeriod}
      </td>
      <td className="px-4 py-3">
        <span className={`text-sm ${
          ct600.daysUntil < 0 ? 'text-red-600 dark:text-red-400' :
          ct600.daysUntil <= 30 ? 'text-orange-600 dark:text-orange-400' :
          'text-gray-600 dark:text-gray-400'
        }`}>
          {formatDeadline()}
        </span>
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <div className="flex-1 h-2 bg-gray-200 dark:bg-slate-600 rounded-full max-w-20">
            <div
              className={`h-full rounded-full ${docsProgress === 100 ? 'bg-green-500' : 'bg-purple-500'}`}
              style={{ width: `${docsProgress}%` }}
            />
          </div>
          <span className="text-xs text-gray-500 dark:text-gray-400">
            {ct600.docsReceived}/{ct600.docsRequired}
          </span>
        </div>
      </td>
      <td className="px-4 py-3">
        {getStatusBadge()}
      </td>
      <td className="px-4 py-3 text-right">
        <div className="flex justify-end gap-2">
          <Link
            href={`/dashboard/services/${ct600.id}`}
            className="px-3 py-1 text-xs font-medium text-purple-600 dark:text-purple-400 hover:bg-purple-100 dark:hover:bg-purple-900/30 rounded"
          >
            View
          </Link>
          {ct600.status !== 'completed' && (
            <button className="px-3 py-1 text-xs font-medium text-green-600 dark:text-green-400 hover:bg-green-100 dark:hover:bg-green-900/30 rounded">
              File
            </button>
          )}
        </div>
      </td>
    </tr>
  );
}

export default CT600Panel;
