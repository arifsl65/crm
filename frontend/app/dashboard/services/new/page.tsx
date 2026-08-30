'use client';

import { useState, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { createService, getClients, getServiceTypes, Client, ServiceType } from '@/lib/api';

interface FormData {
  client_id: string;
  name: string;
  type_id: string;
  period: string;
  priority: string;
  risk_level: string;
  deadline: string;
  docs_required: string;
}

export default function NewServicePage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const preselectedClientId = searchParams.get('client_id');

  const [loading, setLoading] = useState(false);
  const [loadingData, setLoadingData] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [clients, setClients] = useState<Client[]>([]);
  const [serviceTypes, setServiceTypes] = useState<ServiceType[]>([]);

  const [formData, setFormData] = useState<FormData>({
    client_id: preselectedClientId || '',
    name: '',
    type_id: '',
    period: '',
    priority: 'normal',
    risk_level: 'low',
    deadline: '',
    docs_required: '0',
  });

  // Load clients and service types
  useEffect(() => {
    const loadData = async () => {
      try {
        const [clientsRes, typesRes] = await Promise.all([
          getClients({ limit: 100, status: 'active' }),
          getServiceTypes({ active: true }),
        ]);
        setClients(clientsRes.clients || []);
        setServiceTypes(typesRes.service_types || []);
      } catch (err) {
        setError('Failed to load form data');
      } finally {
        setLoadingData(false);
      }
    };
    loadData();
  }, []);

  // Auto-fill name and settings when service type is selected
  useEffect(() => {
    if (formData.type_id) {
      const selectedType = serviceTypes.find((t) => t.id === formData.type_id);
      if (selectedType) {
        setFormData((prev) => ({
          ...prev,
          name: prev.name || selectedType.name,
          priority: selectedType.default_priority || prev.priority,
        }));
      }
    }
  }, [formData.type_id, serviceTypes]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const data: Record<string, string | number> = {
        client_id: formData.client_id,
        name: formData.name,
        priority: formData.priority,
      };

      if (formData.type_id) data.type_id = formData.type_id;
      if (formData.period) data.period = formData.period;
      if (formData.risk_level) data.risk_level = formData.risk_level;
      if (formData.deadline) data.deadline = formData.deadline;
      if (formData.docs_required && parseInt(formData.docs_required) > 0) {
        data.docs_required = parseInt(formData.docs_required);
      }

      const result = await createService(data as Parameters<typeof createService>[0]);

      // Redirect to client page or services list
      if (preselectedClientId) {
        router.push(`/dashboard/clients/${preselectedClientId}`);
      } else {
        router.push('/dashboard/services');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create service');
    } finally {
      setLoading(false);
    }
  };

  if (loadingData) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-slate-900 flex items-center justify-center">
        <div className="text-gray-500 dark:text-gray-400">Loading...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <header className="bg-white dark:bg-slate-800 shadow">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex items-center space-x-4">
          <Link
            href={preselectedClientId ? `/dashboard/clients/${preselectedClientId}` : '/dashboard/services'}
            className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
            </svg>
          </Link>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">New Service</h1>
        </div>
      </header>

      {/* Form */}
      <main className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <form onSubmit={handleSubmit} className="space-y-8">
          {error && (
            <div className="p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
              <p className="text-red-700 dark:text-red-300">{error}</p>
            </div>
          )}

          <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Service Details</h2>
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <div className="sm:col-span-2">
                <label htmlFor="client_id" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Client *
                </label>
                <select
                  name="client_id"
                  id="client_id"
                  required
                  value={formData.client_id}
                  onChange={handleChange}
                  disabled={!!preselectedClientId}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500 sm:text-sm disabled:opacity-50"
                >
                  <option value="">Select a client</option>
                  {clients.map((client) => (
                    <option key={client.id} value={client.id}>
                      {client.company_name}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label htmlFor="type_id" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Service Type
                </label>
                <select
                  name="type_id"
                  id="type_id"
                  value={formData.type_id}
                  onChange={handleChange}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
                >
                  <option value="">Select a type (optional)</option>
                  {serviceTypes.map((type) => (
                    <option key={type.id} value={type.id}>
                      {type.name} ({type.category})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label htmlFor="name" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Service Name *
                </label>
                <input
                  type="text"
                  name="name"
                  id="name"
                  required
                  value={formData.name}
                  onChange={handleChange}
                  placeholder="e.g., Annual Accounts 2024"
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
                />
              </div>

              <div>
                <label htmlFor="period" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Period
                </label>
                <input
                  type="text"
                  name="period"
                  id="period"
                  value={formData.period}
                  onChange={handleChange}
                  placeholder="e.g., 2024-25, Q1 2024"
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
                />
              </div>

              <div>
                <label htmlFor="deadline" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Deadline
                </label>
                <input
                  type="date"
                  name="deadline"
                  id="deadline"
                  value={formData.deadline}
                  onChange={handleChange}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
                />
              </div>

              <div>
                <label htmlFor="priority" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Priority
                </label>
                <select
                  name="priority"
                  id="priority"
                  value={formData.priority}
                  onChange={handleChange}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
                >
                  <option value="low">Low</option>
                  <option value="normal">Normal</option>
                  <option value="high">High</option>
                  <option value="urgent">Urgent</option>
                </select>
              </div>

              <div>
                <label htmlFor="risk_level" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Risk Level
                </label>
                <select
                  name="risk_level"
                  id="risk_level"
                  value={formData.risk_level}
                  onChange={handleChange}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
                >
                  <option value="low">Low</option>
                  <option value="medium">Medium</option>
                  <option value="high">High</option>
                </select>
              </div>

              <div>
                <label htmlFor="docs_required" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Documents Required
                </label>
                <input
                  type="number"
                  name="docs_required"
                  id="docs_required"
                  min="0"
                  value={formData.docs_required}
                  onChange={handleChange}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
                />
              </div>
            </div>
          </div>

          {/* Actions */}
          <div className="flex justify-end space-x-4">
            <Link
              href={preselectedClientId ? `/dashboard/clients/${preselectedClientId}` : '/dashboard/services'}
              className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-slate-700 hover:bg-gray-50 dark:hover:bg-slate-600"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Creating...' : 'Create Service'}
            </button>
          </div>
        </form>
      </main>
    </div>
  );
}
