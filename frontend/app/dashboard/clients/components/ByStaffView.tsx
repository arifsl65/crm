'use client';

import { useState, useEffect, useCallback } from 'react';
import { getClients, getUsers, Client, User } from '@/lib/api';

interface ClientWithRisk extends Client {
  riskLevel?: 'high' | 'medium' | 'low' | 'none';
  isQuiet?: boolean;
}

interface StaffWithClients {
  staff: User;
  clients: ClientWithRisk[];
  isOverloaded: boolean;
}

interface ByStaffViewProps {
  onClientSelect: (client: ClientWithRisk) => void;
  selectedClientId?: string;
}

export function ByStaffView({ onClientSelect, selectedClientId }: ByStaffViewProps) {
  const [staffData, setStaffData] = useState<StaffWithClients[]>([]);
  const [unassignedClients, setUnassignedClients] = useState<ClientWithRisk[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedStaff, setExpandedStaff] = useState<Set<string>>(new Set());

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      const [clientsData, usersData] = await Promise.all([
        getClients({ limit: 200 }),
        getUsers({ role: 'staff', limit: 50 }),
      ]);

      const clients = clientsData.clients || [];
      const staff = usersData.users || [];

      // Calculate which clients are "quiet" (no contact in 14+ days)
      const now = new Date();
      const fourteenDaysAgo = new Date(now.getTime() - 14 * 24 * 60 * 60 * 1000);

      const clientsWithRisk: ClientWithRisk[] = clients.map((client) => {
        const lastContact = client.last_contact_at ? new Date(client.last_contact_at) : null;
        const isQuiet = !lastContact || lastContact < fourteenDaysAgo;

        // Determine risk level based on status and other factors
        let riskLevel: 'high' | 'medium' | 'low' | 'none' = 'none';
        if (client.risk_score && client.risk_score >= 70) riskLevel = 'high';
        else if (client.risk_score && client.risk_score >= 40) riskLevel = 'medium';
        else if (client.risk_score && client.risk_score > 0) riskLevel = 'low';

        return {
          ...client,
          riskLevel,
          isQuiet,
        };
      });

      // Group clients by staff
      const staffMap = new Map<string, ClientWithRisk[]>();
      const unassigned: ClientWithRisk[] = [];

      clientsWithRisk.forEach((client) => {
        if (client.user_id) {
          const existing = staffMap.get(client.user_id) || [];
          existing.push(client);
          staffMap.set(client.user_id, existing);
        } else {
          unassigned.push(client);
        }
      });

      // Calculate average client count for overload detection
      const totalAssigned = clientsWithRisk.length - unassigned.length;
      const avgClientCount = staff.length > 0 ? totalAssigned / staff.length : 0;

      // Build staff with clients data
      const staffWithClients: StaffWithClients[] = staff
        .filter((s) => s.is_active || s.status === 'active')
        .map((s) => {
          const staffClients = staffMap.get(s.id) || [];
          return {
            staff: s,
            clients: staffClients,
            isOverloaded: staffClients.length > avgClientCount * 1.5, // 50% above average
          };
        })
        .sort((a, b) => b.clients.length - a.clients.length);

      setStaffData(staffWithClients);
      setUnassignedClients(unassigned);

      // Expand all staff by default
      setExpandedStaff(new Set(staffWithClients.map(s => s.staff.id)));

      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const toggleStaffExpand = (staffId: string) => {
    setExpandedStaff(prev => {
      const next = new Set(prev);
      if (next.has(staffId)) {
        next.delete(staffId);
      } else {
        next.add(staffId);
      }
      return next;
    });
  };

  const getStatusIcon = (client: ClientWithRisk) => {
    if (client.riskLevel === 'high') return '🔴';
    if (client.isQuiet) return '😶';
    if (client.status === 'pending') return '🟡';
    return '🟢';
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4">
        <div className="p-3 bg-red-50 dark:bg-red-900/30 rounded-lg text-red-700 dark:text-red-300 text-sm">
          {error}
        </div>
      </div>
    );
  }

  return (
    <div className="divide-y divide-gray-200 dark:divide-gray-700">
      {/* Workload Summary */}
      <div className="p-4 bg-gray-50 dark:bg-slate-800/50">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-medium text-gray-900 dark:text-white">Workload Distribution</h3>
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
              {staffData.length} staff members • {unassignedClients.length} unassigned
            </p>
          </div>
          <button className="px-3 py-1.5 text-xs font-medium text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-900/30 rounded hover:bg-blue-200 dark:hover:bg-blue-900/50">
            Rebalance Workload
          </button>
        </div>
      </div>

      {/* Staff Groups */}
      {staffData.map(({ staff, clients, isOverloaded }) => (
        <div key={staff.id} className="bg-white dark:bg-slate-800">
          {/* Staff Header */}
          <button
            onClick={() => toggleStaffExpand(staff.id)}
            className="w-full p-4 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-slate-700"
          >
            <div className="flex items-center gap-3">
              <span className="text-lg">👤</span>
              <div className="text-left">
                <p className="text-sm font-medium text-gray-900 dark:text-white">
                  {staff.name}
                </p>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  {clients.length} client{clients.length !== 1 ? 's' : ''}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {isOverloaded && (
                <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300">
                  ⚠️ Overloaded
                </span>
              )}
              <span className="text-gray-400 text-xs">
                {expandedStaff.has(staff.id) ? '▼' : '▶'}
              </span>
            </div>
          </button>

          {/* Clients under this staff */}
          {expandedStaff.has(staff.id) && clients.length > 0 && (
            <div className="border-t border-gray-100 dark:border-gray-700">
              {clients.map((client) => (
                <div
                  key={client.id}
                  onClick={() => onClientSelect(client)}
                  className={`pl-12 pr-4 py-3 flex items-center justify-between cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700 ${
                    selectedClientId === client.id ? 'bg-blue-50 dark:bg-blue-900/20' : ''
                  }`}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="text-sm">{getStatusIcon(client)}</span>
                    <span className="text-sm text-gray-900 dark:text-white truncate">
                      {client.company_name}
                    </span>
                  </div>
                  <div className="flex items-center gap-1 ml-2">
                    <button
                      onClick={(e) => { e.stopPropagation(); /* TODO: Email action */ }}
                      className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                      title="Send Email"
                    >
                      📧
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); /* TODO: Request docs */ }}
                      className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                      title="Request Documents"
                    >
                      📄
                    </button>
                    {(client.riskLevel === 'high' || client.isQuiet) && (
                      <button
                        onClick={(e) => { e.stopPropagation(); /* TODO: Chase */ }}
                        className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                        title="Chase"
                      >
                        🔔
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}

          {expandedStaff.has(staff.id) && clients.length === 0 && (
            <div className="pl-12 pr-4 py-3 text-sm text-gray-400 dark:text-gray-500 italic">
              No clients assigned
            </div>
          )}
        </div>
      ))}

      {/* Unassigned Clients */}
      {unassignedClients.length > 0 && (
        <div className="bg-white dark:bg-slate-800">
          <button
            onClick={() => toggleStaffExpand('unassigned')}
            className="w-full p-4 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-slate-700"
          >
            <div className="flex items-center gap-3">
              <span className="text-lg">❓</span>
              <div className="text-left">
                <p className="text-sm font-medium text-gray-900 dark:text-white">
                  Unassigned
                </p>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  {unassignedClients.length} client{unassignedClients.length !== 1 ? 's' : ''}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300">
                ⚠️ Needs Assignment
              </span>
              <span className="text-gray-400 text-xs">
                {expandedStaff.has('unassigned') ? '▼' : '▶'}
              </span>
            </div>
          </button>

          {expandedStaff.has('unassigned') && (
            <div className="border-t border-gray-100 dark:border-gray-700">
              {unassignedClients.map((client) => (
                <div
                  key={client.id}
                  onClick={() => onClientSelect(client)}
                  className={`pl-12 pr-4 py-3 flex items-center justify-between cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700 ${
                    selectedClientId === client.id ? 'bg-blue-50 dark:bg-blue-900/20' : ''
                  }`}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="text-sm">{getStatusIcon(client)}</span>
                    <span className="text-sm text-gray-900 dark:text-white truncate">
                      {client.company_name}
                    </span>
                  </div>
                  <div className="flex items-center gap-1 ml-2">
                    <button
                      onClick={(e) => { e.stopPropagation(); }}
                      className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                      title="Send Email"
                    >
                      📧
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); }}
                      className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                      title="Request Documents"
                    >
                      📄
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default ByStaffView;
