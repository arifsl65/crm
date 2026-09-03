'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import { Service, getServices, updateServiceStatus } from '@/lib/api';
import { getStatusBadgeClass } from '@/lib/status';

type ViewMode = 'kanban' | 'list';

const STATUS_COLUMNS = [
  { id: 'not_started', label: 'Not Started', color: 'gray' },
  { id: 'in_progress', label: 'In Progress', color: 'blue' },
  { id: 'review', label: 'Review', color: 'purple' },
  { id: 'waiting', label: 'Waiting', color: 'yellow' },
  { id: 'completed', label: 'Completed', color: 'green' },
];

export default function ServicesPage() {
  const { user } = useAuth();
  const [services, setServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [priorityFilter, setPriorityFilter] = useState('');
  const [viewMode, setViewMode] = useState<ViewMode>('kanban');
  const [selectedService, setSelectedService] = useState<Service | null>(null);
  const [draggedService, setDraggedService] = useState<Service | null>(null);

  const fetchServices = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getServices({
        search: search || undefined,
        priority: priorityFilter || undefined,
        limit: 100,
      });
      setServices(data.services || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load services');
    } finally {
      setLoading(false);
    }
  }, [search, priorityFilter]);

  useEffect(() => {
    fetchServices();
  }, [fetchServices]);

  const handleStatusChange = async (serviceId: string, newStatus: string) => {
    const previousServices = [...services];
    setServices((prev) =>
      prev.map((s) => (s.id === serviceId ? { ...s, status: newStatus } : s))
    );

    try {
      await updateServiceStatus(serviceId, newStatus);
    } catch (err) {
      setServices(previousServices);
      setError(err instanceof Error ? err.message : 'Failed to update status');
    }
  };

  const handleDragStart = (service: Service) => {
    setDraggedService(service);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
  };

  const handleDrop = (status: string) => {
    if (draggedService && draggedService.status !== status) {
      handleStatusChange(draggedService.id, status);
    }
    setDraggedService(null);
  };

  const getServicesByStatus = (status: string) => {
    return services.filter((s) => s.status === status);
  };

  const getPriorityIcon = (priority: string) => {
    switch (priority) {
      case 'urgent': return '🔴';
      case 'high': return '🟠';
      case 'normal': return '🟡';
      case 'low': return '🟢';
      default: return '⚪';
    }
  };

  const isOverdue = (deadline: string | undefined) => {
    if (!deadline) return false;
    return new Date(deadline) < new Date();
  };

  const formatDeadline = (deadline: string | undefined) => {
    if (!deadline) return null;
    const date = new Date(deadline);
    const now = new Date();
    const daysUntil = Math.ceil((date.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));

    if (daysUntil < 0) return `${Math.abs(daysUntil)}d overdue`;
    if (daysUntil === 0) return 'Today';
    if (daysUntil === 1) return 'Tomorrow';
    if (daysUntil <= 7) return `${daysUntil}d`;
    return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
  };

  return (
    <div className="h-full flex flex-col bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <div className="bg-white dark:bg-slate-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <span>📋</span> Services
            </h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              {services.length} services
            </p>
          </div>
          <div className="flex items-center gap-2">
            {/* View Toggle */}
            <div className="flex rounded-lg border border-gray-200 dark:border-gray-600 overflow-hidden">
              <button
                onClick={() => setViewMode('kanban')}
                className={`px-3 py-1.5 text-sm font-medium ${
                  viewMode === 'kanban'
                    ? 'bg-blue-600 text-white'
                    : 'bg-white dark:bg-slate-800 text-gray-700 dark:text-gray-300'
                }`}
              >
                Kanban
              </button>
              <button
                onClick={() => setViewMode('list')}
                className={`px-3 py-1.5 text-sm font-medium ${
                  viewMode === 'list'
                    ? 'bg-blue-600 text-white'
                    : 'bg-white dark:bg-slate-800 text-gray-700 dark:text-gray-300'
                }`}
              >
                List
              </button>
            </div>
            <Link
              href="/dashboard/services/new"
              className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700"
            >
              + Add Service
            </Link>
          </div>
        </div>

        {/* Filters */}
        <div className="mt-4 flex gap-3">
          <input
            type="text"
            placeholder="Search services..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
          />
          <select
            value={priorityFilter}
            onChange={(e) => setPriorityFilter(e.target.value)}
            className="px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
          >
            <option value="">All Priorities</option>
            <option value="urgent">Urgent</option>
            <option value="high">High</option>
            <option value="normal">Normal</option>
            <option value="low">Low</option>
          </select>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="mx-6 mt-4 p-3 bg-red-50 dark:bg-red-900/30 rounded-lg text-red-700 dark:text-red-300 text-sm">
          {error}
        </div>
      )}

      {/* Content */}
      <div className="flex-1 flex overflow-hidden">
        {/* Main View */}
        <div className={`${selectedService ? 'w-2/3' : 'flex-1'} overflow-hidden`}>
          {loading ? (
            <div className="flex items-center justify-center h-64">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
          ) : viewMode === 'kanban' ? (
            <KanbanView
              columns={STATUS_COLUMNS}
              getServicesByStatus={getServicesByStatus}
              onDragStart={handleDragStart}
              onDragOver={handleDragOver}
              onDrop={handleDrop}
              onServiceClick={setSelectedService}
              selectedService={selectedService}
              getPriorityIcon={getPriorityIcon}
              formatDeadline={formatDeadline}
              isOverdue={isOverdue}
            />
          ) : (
            <ListView
              services={services}
              onServiceClick={setSelectedService}
              selectedService={selectedService}
              handleStatusChange={handleStatusChange}
              getPriorityIcon={getPriorityIcon}
              formatDeadline={formatDeadline}
              isOverdue={isOverdue}
            />
          )}
        </div>

        {/* Detail Panel */}
        {selectedService && (
          <div className="w-1/3 bg-white dark:bg-slate-800 border-l border-gray-200 dark:border-gray-700 overflow-y-auto">
            <ServiceDetailPanel
              service={selectedService}
              onClose={() => setSelectedService(null)}
              onStatusChange={handleStatusChange}
            />
          </div>
        )}
      </div>
    </div>
  );
}

function KanbanView({
  columns,
  getServicesByStatus,
  onDragStart,
  onDragOver,
  onDrop,
  onServiceClick,
  selectedService,
  getPriorityIcon,
  formatDeadline,
  isOverdue,
}: {
  columns: { id: string; label: string; color: string }[];
  getServicesByStatus: (status: string) => Service[];
  onDragStart: (service: Service) => void;
  onDragOver: (e: React.DragEvent) => void;
  onDrop: (status: string) => void;
  onServiceClick: (service: Service) => void;
  selectedService: Service | null;
  getPriorityIcon: (priority: string) => string;
  formatDeadline: (deadline: string | undefined) => string | null;
  isOverdue: (deadline: string | undefined) => boolean;
}) {
  const getColumnColor = (color: string) => {
    switch (color) {
      case 'blue': return 'border-blue-400';
      case 'purple': return 'border-purple-400';
      case 'yellow': return 'border-yellow-400';
      case 'green': return 'border-green-400';
      default: return 'border-gray-400';
    }
  };

  return (
    <div className="h-full overflow-x-auto p-4">
      <div className="flex gap-4 h-full min-w-max">
        {columns.map((column) => {
          const columnServices = getServicesByStatus(column.id);
          return (
            <div
              key={column.id}
              className="w-72 flex-shrink-0 flex flex-col"
              onDragOver={onDragOver}
              onDrop={() => onDrop(column.id)}
            >
              {/* Column Header */}
              <div className={`p-3 bg-white dark:bg-slate-800 rounded-t-lg border-t-4 ${getColumnColor(column.color)}`}>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-semibold text-gray-700 dark:text-gray-300">
                    {column.label}
                  </span>
                  <span className="px-2 py-0.5 text-xs font-medium bg-gray-100 dark:bg-slate-700 text-gray-600 dark:text-gray-400 rounded-full">
                    {columnServices.length}
                  </span>
                </div>
              </div>

              {/* Column Content */}
              <div className="flex-1 bg-gray-100 dark:bg-slate-900 rounded-b-lg p-2 space-y-2 overflow-y-auto">
                {columnServices.map((service) => (
                  <div
                    key={service.id}
                    draggable
                    onDragStart={() => onDragStart(service)}
                    onClick={() => onServiceClick(service)}
                    className={`p-3 bg-white dark:bg-slate-800 rounded-lg shadow-sm cursor-pointer hover:shadow-md transition-shadow ${
                      selectedService?.id === service.id ? 'ring-2 ring-blue-500' : ''
                    }`}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <p className="text-sm font-medium text-gray-900 dark:text-white line-clamp-2">
                        {service.name}
                      </p>
                      <span>{getPriorityIcon(service.priority)}</span>
                    </div>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 truncate">
                      {service.client_name || 'No client'}
                    </p>
                    <div className="flex items-center justify-between mt-2">
                      {service.deadline && (
                        <span className={`text-xs ${
                          isOverdue(service.deadline)
                            ? 'text-red-600 dark:text-red-400 font-medium'
                            : 'text-gray-500 dark:text-gray-400'
                        }`}>
                          {formatDeadline(service.deadline)}
                        </span>
                      )}
                      <div className="flex items-center gap-1 text-xs text-gray-400">
                        <span>📄</span>
                        <span>{service.docs_received}/{service.docs_required}</span>
                      </div>
                    </div>
                  </div>
                ))}
                {columnServices.length === 0 && (
                  <div className="p-4 text-center text-xs text-gray-400 dark:text-gray-500">
                    Drop services here
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ListView({
  services,
  onServiceClick,
  selectedService,
  handleStatusChange,
  getPriorityIcon,
  formatDeadline,
  isOverdue,
}: {
  services: Service[];
  onServiceClick: (service: Service) => void;
  selectedService: Service | null;
  handleStatusChange: (serviceId: string, newStatus: string) => void;
  getPriorityIcon: (priority: string) => string;
  formatDeadline: (deadline: string | undefined) => string | null;
  isOverdue: (deadline: string | undefined) => boolean;
}) {
  if (services.length === 0) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <span className="text-4xl">📋</span>
          <p className="mt-2 text-gray-600 dark:text-gray-400">No services found</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto">
      <table className="w-full">
        <thead className="bg-gray-50 dark:bg-slate-700 sticky top-0">
          <tr>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Service</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Client</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Status</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Priority</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Deadline</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Docs</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
          {services.map((service) => (
            <tr
              key={service.id}
              onClick={() => onServiceClick(service)}
              className={`cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-800 ${
                selectedService?.id === service.id ? 'bg-blue-50 dark:bg-blue-900/20' : 'bg-white dark:bg-slate-800'
              }`}
            >
              <td className="px-4 py-3">
                <p className="text-sm font-medium text-gray-900 dark:text-white">{service.name}</p>
                {service.period && (
                  <p className="text-xs text-gray-500 dark:text-gray-400">{service.period}</p>
                )}
              </td>
              <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                {service.client_name || '-'}
              </td>
              <td className="px-4 py-3">
                <select
                  value={service.status}
                  onChange={(e) => { e.stopPropagation(); handleStatusChange(service.id, e.target.value); }}
                  onClick={(e) => e.stopPropagation()}
                  className={`px-2 py-1 text-xs font-medium rounded-full border-0 cursor-pointer ${getStatusBadgeClass(service.status)}`}
                >
                  <option value="not_started">Not Started</option>
                  <option value="in_progress">In Progress</option>
                  <option value="review">Review</option>
                  <option value="waiting">Waiting</option>
                  <option value="completed">Completed</option>
                  <option value="cancelled">Cancelled</option>
                </select>
              </td>
              <td className="px-4 py-3">
                <span className="flex items-center gap-1 text-sm">
                  {getPriorityIcon(service.priority)}
                  <span className="text-gray-600 dark:text-gray-400 capitalize">{service.priority}</span>
                </span>
              </td>
              <td className="px-4 py-3">
                {service.deadline ? (
                  <span className={`text-sm ${
                    isOverdue(service.deadline) ? 'text-red-600 dark:text-red-400 font-medium' : 'text-gray-500 dark:text-gray-400'
                  }`}>
                    {formatDeadline(service.deadline)}
                  </span>
                ) : (
                  <span className="text-gray-400">-</span>
                )}
              </td>
              <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                {service.docs_received}/{service.docs_required}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ServiceDetailPanel({
  service,
  onClose,
  onStatusChange,
}: {
  service: Service;
  onClose: () => void;
  onStatusChange: (serviceId: string, newStatus: string) => void;
}) {
  const [activeTab, setActiveTab] = useState<'details' | 'documents' | 'activity'>('details');

  const docsProgress = service.docs_required > 0
    ? Math.round((service.docs_received / service.docs_required) * 100)
    : 0;

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white truncate">{service.name}</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400">{service.client_name || 'No client'}</p>
        </div>
        <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 ml-2">
          ✕
        </button>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700 px-4">
        <div className="flex gap-4">
          {(['details', 'documents', 'activity'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`py-3 text-sm font-medium border-b-2 ${
                activeTab === tab
                  ? 'border-blue-600 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400'
              }`}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {activeTab === 'details' && (
          <div className="space-y-4">
            {/* Status */}
            <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
              <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Status</h3>
              <select
                value={service.status}
                onChange={(e) => onStatusChange(service.id, e.target.value)}
                className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-600 text-gray-900 dark:text-white"
              >
                <option value="not_started">Not Started</option>
                <option value="in_progress">In Progress</option>
                <option value="review">Review</option>
                <option value="waiting">Waiting</option>
                <option value="completed">Completed</option>
                <option value="cancelled">Cancelled</option>
              </select>
            </div>

            {/* Details */}
            <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
              <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Details</h3>
              <div className="space-y-2 text-sm">
                <p><span className="text-gray-500">Period:</span> {service.period || '-'}</p>
                <p><span className="text-gray-500">Priority:</span> {service.priority}</p>
                <p><span className="text-gray-500">Deadline:</span> {service.deadline || '-'}</p>
                <p><span className="text-gray-500">Assigned to:</span> {service.staff_name || '-'}</p>
              </div>
            </div>

            {/* Documents Progress */}
            <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
              <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Documents</h3>
              <div className="flex items-center gap-3">
                <div className="flex-1 h-2 bg-gray-200 dark:bg-slate-600 rounded-full">
                  <div
                    className={`h-full rounded-full ${docsProgress === 100 ? 'bg-green-500' : 'bg-blue-500'}`}
                    style={{ width: `${docsProgress}%` }}
                  />
                </div>
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  {service.docs_received}/{service.docs_required}
                </span>
              </div>
            </div>

            {/* Quick Actions */}
            <div className="grid grid-cols-2 gap-2">
              <button className="p-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
                📧 Email Client
              </button>
              <button className="p-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
                📄 Request Docs
              </button>
              <button className="p-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
                📝 Add Note
              </button>
              <button className="p-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
                👤 Reassign
              </button>
            </div>
          </div>
        )}

        {activeTab === 'documents' && (
          <div className="text-center py-8 text-gray-500 dark:text-gray-400">
            <span className="text-4xl">📄</span>
            <p className="mt-2">Document list coming soon</p>
          </div>
        )}

        {activeTab === 'activity' && (
          <div className="text-center py-8 text-gray-500 dark:text-gray-400">
            <span className="text-4xl">📜</span>
            <p className="mt-2">Activity log coming soon</p>
          </div>
        )}
      </div>
    </div>
  );
}
