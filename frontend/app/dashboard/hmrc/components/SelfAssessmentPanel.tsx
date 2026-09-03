'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getServices, Service } from '@/lib/api';

interface SAReturn {
  id: string;
  clientName: string;
  clientId?: string;
  utr?: string;
  taxYear: string;
  deadline: string;
  daysUntil: number;
  status: string;
  docsReceived: number;
  docsRequired: number;
}

export function SelfAssessmentPanel() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saReturns, setSAReturns] = useState<SAReturn[]>([]);
  const [filter, setFilter] = useState<'all' | 'pending' | 'overdue' | 'completed'>('all');

  useEffect(() => {
    async function fetchSAReturns() {
      try {
        setLoading(true);
        const data = await getServices({ limit: 200 });
        const services = data.services || [];

        const now = new Date();

        // Filter for Self Assessment services
        const saServices = services
          .filter((s: Service) => {
            const name = s.name.toLowerCase();
            return name.includes('self assessment') ||
                   (name.includes('tax return') && !name.includes('corporation') && !name.includes('vat'));
          })
          .map((s: Service) => {
            const deadline = s.deadline ? new Date(s.deadline) : null;
            const daysUntil = deadline
              ? Math.ceil((deadline.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
              : 999;

            // Try to extract tax year from service name
            const yearMatch = s.name.match(/\d{4}[/-]\d{2,4}|\d{4}/g);
            const taxYear = yearMatch ? yearMatch[0] : 'N/A';

            return {
              id: s.id,
              clientName: s.client_name || 'Unknown Client',
              clientId: s.client_id,
              utr: undefined,
              taxYear,
              deadline: s.deadline || '',
              daysUntil,
              status: s.status,
              docsReceived: s.docs_received || 0,
              docsRequired: s.docs_required || 0,
            };
          })
          .sort((a, b) => a.daysUntil - b.daysUntil);

        setSAReturns(saServices);
        setError(null);
      } catch (err) {
        setError('Failed to load Self Assessment returns');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchSAReturns();
  }, []);

  const filteredReturns = saReturns.filter(r => {
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
                  ? 'bg-green-600 text-white'
                  : 'bg-white dark:bg-slate-800 text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-600'
              }`}
            >
              {f.charAt(0).toUpperCase() + f.slice(1)}
              {f !== 'all' && (
                <span className="ml-1 text-xs">
                  ({saReturns.filter(r => {
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
        <button className="px-4 py-2 bg-green-600 text-white text-sm font-medium rounded-lg hover:bg-green-700">
          + New SA Return
        </button>
      </div>

      {/* SA Returns Table */}
      <div className="p-4">
        {filteredReturns.length === 0 ? (
          <div className="text-center py-12">
            <span className="text-4xl">👤</span>
            <p className="mt-2 text-gray-600 dark:text-gray-400">No Self Assessment returns found</p>
          </div>
        ) : (
          <div className="bg-white dark:bg-slate-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="w-full">
              <thead className="bg-gray-50 dark:bg-slate-700">
                <tr>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Client</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Tax Year</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Deadline</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Documents</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Status</th>
                  <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                {filteredReturns.map((sa) => (
                  <SAReturnRow key={sa.id} sa={sa} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* SA Deadlines Info */}
      <div className="p-4 grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-green-50 dark:bg-green-900/20 rounded-lg p-4 border border-green-200 dark:border-green-800">
          <h4 className="font-medium text-green-900 dark:text-green-100 flex items-center gap-2">
            <span>📅</span> Paper Filing Deadline
          </h4>
          <p className="text-2xl font-bold text-green-600 dark:text-green-400 mt-1">31 October</p>
          <p className="text-sm text-green-700 dark:text-green-300 mt-1">
            For paper Self Assessment returns
          </p>
        </div>
        <div className="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4 border border-blue-200 dark:border-blue-800">
          <h4 className="font-medium text-blue-900 dark:text-blue-100 flex items-center gap-2">
            <span>💻</span> Online Filing Deadline
          </h4>
          <p className="text-2xl font-bold text-blue-600 dark:text-blue-400 mt-1">31 January</p>
          <p className="text-sm text-blue-700 dark:text-blue-300 mt-1">
            For online Self Assessment returns
          </p>
        </div>
      </div>
    </div>
  );
}

function SAReturnRow({ sa }: { sa: SAReturn }) {
  const getStatusBadge = () => {
    if (sa.status === 'completed') {
      return <span className="px-2 py-1 text-xs font-medium rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300">Filed</span>;
    }
    if (sa.daysUntil < 0) {
      return <span className="px-2 py-1 text-xs font-medium rounded bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300">Overdue</span>;
    }
    if (sa.daysUntil <= 30) {
      return <span className="px-2 py-1 text-xs font-medium rounded bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300">Due Soon</span>;
    }
    if (sa.status === 'in_progress') {
      return <span className="px-2 py-1 text-xs font-medium rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300">In Progress</span>;
    }
    return <span className="px-2 py-1 text-xs font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">Not Started</span>;
  };

  const formatDeadline = () => {
    if (!sa.deadline) return 'No deadline';
    const date = new Date(sa.deadline);
    const formatted = date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });

    if (sa.status === 'completed') return formatted;
    if (sa.daysUntil < 0) return `${formatted} (${Math.abs(sa.daysUntil)}d overdue)`;
    if (sa.daysUntil === 0) return `${formatted} (Today)`;
    return `${formatted} (${sa.daysUntil}d)`;
  };

  const docsProgress = sa.docsRequired > 0 ? Math.round((sa.docsReceived / sa.docsRequired) * 100) : 0;

  return (
    <tr className="hover:bg-gray-50 dark:hover:bg-slate-700">
      <td className="px-4 py-3">
        <Link href={`/dashboard/clients/${sa.clientId}`} className="text-sm font-medium text-gray-900 dark:text-white hover:text-green-600">
          {sa.clientName}
        </Link>
      </td>
      <td className="px-4 py-3 text-sm text-gray-600 dark:text-gray-400">
        {sa.taxYear}
      </td>
      <td className="px-4 py-3">
        <span className={`text-sm ${
          sa.daysUntil < 0 ? 'text-red-600 dark:text-red-400' :
          sa.daysUntil <= 30 ? 'text-orange-600 dark:text-orange-400' :
          'text-gray-600 dark:text-gray-400'
        }`}>
          {formatDeadline()}
        </span>
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <div className="flex-1 h-2 bg-gray-200 dark:bg-slate-600 rounded-full max-w-20">
            <div
              className={`h-full rounded-full ${docsProgress === 100 ? 'bg-green-500' : 'bg-green-400'}`}
              style={{ width: `${docsProgress}%` }}
            />
          </div>
          <span className="text-xs text-gray-500 dark:text-gray-400">
            {sa.docsReceived}/{sa.docsRequired}
          </span>
        </div>
      </td>
      <td className="px-4 py-3">
        {getStatusBadge()}
      </td>
      <td className="px-4 py-3 text-right">
        <div className="flex justify-end gap-2">
          <Link
            href={`/dashboard/services/${sa.id}`}
            className="px-3 py-1 text-xs font-medium text-green-600 dark:text-green-400 hover:bg-green-100 dark:hover:bg-green-900/30 rounded"
          >
            View
          </Link>
          {sa.status !== 'completed' && (
            <button className="px-3 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded">
              File
            </button>
          )}
        </div>
      </td>
    </tr>
  );
}

export default SelfAssessmentPanel;
