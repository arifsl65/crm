'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getClients, getServices, Client, Service } from '@/lib/api';

interface AlertItem {
  id: string;
  text: string;
  actionLabel: string;
  actionHref: string;
}

interface AlertsPanelProps {
  onClose?: () => void;
}

export function AlertsPanel({ onClose }: AlertsPanelProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [atRisk, setAtRisk] = useState<AlertItem[]>([]);
  const [quiet, setQuiet] = useState<AlertItem[]>([]);

  useEffect(() => {
    async function fetchAlerts() {
      try {
        setLoading(true);

        const [clientsData, servicesData] = await Promise.all([
          getClients({ limit: 100 }),
          getServices({ limit: 100 }),
        ]);

        const clients = clientsData.clients || [];
        const services = servicesData.services || [];
        const now = new Date();

        const atRiskItems: AlertItem[] = [];
        const quietItems: AlertItem[] = [];

        // Group services by client
        const servicesByClient: Record<string, Service[]> = {};
        services.forEach((service: Service) => {
          const clientId = service.client_id;
          if (!clientId) return;
          if (!servicesByClient[clientId]) {
            servicesByClient[clientId] = [];
          }
          servicesByClient[clientId].push(service);
        });

        clients.forEach((client: Client) => {
          const clientServices = servicesByClient[client.id] || [];

          // Check for overdue services
          const overdueServices = clientServices.filter((s: Service) => {
            if (!s.deadline || s.status === 'completed' || s.status === 'cancelled') return false;
            return new Date(s.deadline) < now;
          });

          // Get oldest overdue days
          let oldestOverdueDays = 0;
          overdueServices.forEach((s: Service) => {
            if (!s.deadline) return;
            const days = Math.floor((now.getTime() - new Date(s.deadline).getTime()) / (1000 * 60 * 60 * 24));
            if (days > oldestOverdueDays) oldestOverdueDays = days;
          });

          // Check for missing documents
          const missingDocs = clientServices.reduce((acc: number, s: Service) => {
            return acc + Math.max(0, (s.docs_required || 0) - (s.docs_received || 0));
          }, 0);

          if (overdueServices.length > 0 || missingDocs > 0) {
            const parts = [];
            if (oldestOverdueDays > 0) parts.push(`${oldestOverdueDays} days`);
            if (missingDocs > 0) parts.push(`${missingDocs} doc${missingDocs > 1 ? 's' : ''}`);

            atRiskItems.push({
              id: client.id,
              text: `${client.company_name} - ${parts.join(', ')}`,
              actionLabel: '[View]',
              actionHref: `/dashboard/clients/${client.id}`,
            });
          }

          // Check for quiet clients (no contact in 14+ days)
          if (client.last_contact_at && client.status === 'active') {
            const lastContact = new Date(client.last_contact_at);
            const daysSinceContact = Math.floor((now.getTime() - lastContact.getTime()) / (1000 * 60 * 60 * 24));

            if (daysSinceContact >= 14) {
              quietItems.push({
                id: client.id,
                text: `${client.company_name} - ${daysSinceContact} days`,
                actionLabel: '[Chase]',
                actionHref: `/dashboard/clients/${client.id}`,
              });
            }
          }
        });

        // Sort quiet clients by days (most days first)
        quietItems.sort((a, b) => {
          const daysA = parseInt(a.text.match(/(\d+) days/)?.[1] || '0');
          const daysB = parseInt(b.text.match(/(\d+) days/)?.[1] || '0');
          return daysB - daysA;
        });

        setAtRisk(atRiskItems.slice(0, 10));
        setQuiet(quietItems.slice(0, 10));
        setError(null);
      } catch (err) {
        setError('Failed to load alerts');
        console.error(err);
      } finally {
        setLoading(false);
      }
    }
    fetchAlerts();
  }, []);

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>🔔</span> ALERTS
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
          <span>🔔</span> ALERTS
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

        {/* ⚠️ AT RISK Section */}
        <div>
          <h3 className="text-sm font-semibold text-orange-600 dark:text-orange-400 mb-3">
            ⚠️ AT RISK ({atRisk.length})
          </h3>
          {atRisk.length > 0 ? (
            <div className="space-y-1">
              {atRisk.map((item) => (
                <div key={item.id} className="flex items-center justify-between py-1.5">
                  <span className="text-sm text-gray-700 dark:text-gray-300">
                    {item.text}
                  </span>
                  <Link
                    href={item.actionHref}
                    className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    {item.actionLabel}
                  </Link>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400 italic">None</p>
          )}
        </div>

        {/* 😶 QUIET CLIENTS Section */}
        <div>
          <h3 className="text-sm font-semibold text-gray-600 dark:text-gray-400 mb-3">
            😶 QUIET CLIENTS ({quiet.length})
          </h3>
          {quiet.length > 0 ? (
            <div className="space-y-1">
              {quiet.map((item) => (
                <div key={item.id} className="flex items-center justify-between py-1.5">
                  <span className="text-sm text-gray-700 dark:text-gray-300">
                    {item.text}
                  </span>
                  <Link
                    href={item.actionHref}
                    className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
                  >
                    {item.actionLabel}
                  </Link>
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

export default AlertsPanel;
