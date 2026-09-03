'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { getClients, getServices, Client, Service } from '@/lib/api';

interface AlertItem {
  id: string;
  type: 'at_risk' | 'quiet' | 'anomaly';
  title: string;
  subtitle: string;
  severity: 'high' | 'medium' | 'low';
  actionLabel: string;
  actionHref?: string;
}

interface AlertsPanelProps {
  onClose?: () => void;
}

export function AlertsPanel({ onClose }: AlertsPanelProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [atRisk, setAtRisk] = useState<AlertItem[]>([]);
  const [quiet, setQuiet] = useState<AlertItem[]>([]);
  const [anomalies, setAnomalies] = useState<AlertItem[]>([]);

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

        // At Risk: Clients with overdue services or missing documents
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

          // Check for missing documents
          const missingDocs = clientServices.reduce((acc: number, s: Service) => {
            return acc + ((s.docs_required || 0) - (s.docs_received || 0));
          }, 0);

          if (overdueServices.length > 0 || missingDocs > 0) {
            const details = [];
            if (overdueServices.length > 0) {
              details.push(`${overdueServices.length} overdue`);
            }
            if (missingDocs > 0) {
              details.push(`${missingDocs} docs missing`);
            }

            atRiskItems.push({
              id: client.id,
              type: 'at_risk',
              title: client.company_name,
              subtitle: details.join(', '),
              severity: overdueServices.length > 1 ? 'high' : 'medium',
              actionLabel: 'View',
              actionHref: `/dashboard/clients/${client.id}`,
            });
          }

          // Check for quiet clients (no contact in 14+ days)
          if (client.last_contact_at) {
            const lastContact = new Date(client.last_contact_at);
            const daysSinceContact = Math.floor((now.getTime() - lastContact.getTime()) / (1000 * 60 * 60 * 24));

            if (daysSinceContact >= 14 && client.status === 'active') {
              quietItems.push({
                id: client.id,
                type: 'quiet',
                title: client.company_name,
                subtitle: `${daysSinceContact} days since last contact`,
                severity: daysSinceContact >= 30 ? 'high' : daysSinceContact >= 21 ? 'medium' : 'low',
                actionLabel: 'Chase',
                actionHref: `/dashboard/clients/${client.id}`,
              });
            }
          }
        });

        // Anomalies: Could be staff workload imbalance, unusual patterns, etc.
        // For now, we'll create placeholder anomalies
        const anomalyItems: AlertItem[] = [];

        // Example: Check if any staff has too many clients (placeholder logic)
        // In production, this would come from the backend

        setAtRisk(atRiskItems.slice(0, 10));
        setQuiet(quietItems.sort((a, b) => {
          const daysA = parseInt(a.subtitle.match(/\d+/)?.[0] || '0');
          const daysB = parseInt(b.subtitle.match(/\d+/)?.[0] || '0');
          return daysB - daysA;
        }).slice(0, 10));
        setAnomalies(anomalyItems);
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

  const handleChase = (clientId: string) => {
    console.log('Chase client:', clientId);
    // TODO: Implement chase functionality
  };

  if (loading) {
    return (
      <div className="h-full flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <span>🔔</span> Alerts
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

  const totalAlerts = atRisk.length + quiet.length + anomalies.length;

  return (
    <div className="h-full flex flex-col bg-white dark:bg-slate-800 rounded-lg">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>🔔</span> Alerts
          {totalAlerts > 0 && (
            <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
              {totalAlerts}
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

        {totalAlerts === 0 && !error && (
          <div className="text-center py-8">
            <span className="text-4xl">✨</span>
            <p className="mt-2 text-gray-600 dark:text-gray-400">All clear! No alerts.</p>
          </div>
        )}

        {/* At Risk Section */}
        {atRisk.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-orange-600 dark:text-orange-400 mb-3 flex items-center gap-2">
              ⚠️ AT RISK ({atRisk.length})
            </h3>
            <div className="space-y-2">
              {atRisk.map((item) => (
                <AlertRow key={item.id} item={item} onAction={handleChase} />
              ))}
            </div>
          </div>
        )}

        {/* Quiet Clients Section */}
        {quiet.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-gray-600 dark:text-gray-400 mb-3 flex items-center gap-2">
              😶 QUIET CLIENTS ({quiet.length})
            </h3>
            <div className="space-y-2">
              {quiet.map((item) => (
                <AlertRow key={item.id} item={item} onAction={handleChase} />
              ))}
            </div>
          </div>
        )}

        {/* Anomalies Section */}
        {anomalies.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-purple-600 dark:text-purple-400 mb-3 flex items-center gap-2">
              🔍 ANOMALIES ({anomalies.length})
            </h3>
            <div className="space-y-2">
              {anomalies.map((item) => (
                <AlertRow key={item.id} item={item} onAction={handleChase} />
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700">
        <Link
          href="/dashboard/clients"
          className="w-full flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
        >
          View All Clients →
        </Link>
      </div>
    </div>
  );
}

function AlertRow({
  item,
  onAction,
}: {
  item: AlertItem;
  onAction: (id: string) => void;
}) {
  const getBgColor = () => {
    if (item.type === 'at_risk') {
      return item.severity === 'high'
        ? 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'
        : 'bg-orange-50 dark:bg-orange-900/20 border-orange-200 dark:border-orange-800';
    }
    if (item.type === 'quiet') {
      return 'bg-gray-50 dark:bg-slate-700 border-gray-200 dark:border-gray-600';
    }
    return 'bg-purple-50 dark:bg-purple-900/20 border-purple-200 dark:border-purple-800';
  };

  const getIcon = () => {
    if (item.type === 'at_risk') return item.severity === 'high' ? '🔴' : '🟠';
    if (item.type === 'quiet') return '😶';
    return '🔍';
  };

  return (
    <div className={`flex items-center justify-between p-3 rounded-lg border ${getBgColor()}`}>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span>{getIcon()}</span>
          <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
            {item.title}
          </p>
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 ml-6">
          {item.subtitle}
        </p>
      </div>
      <div className="ml-3">
        {item.actionHref ? (
          <Link
            href={item.actionHref}
            className="px-3 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded"
          >
            {item.actionLabel}
          </Link>
        ) : (
          <button
            onClick={() => onAction(item.id)}
            className="px-3 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded"
          >
            {item.actionLabel}
          </button>
        )}
      </div>
    </div>
  );
}

export default AlertsPanel;
