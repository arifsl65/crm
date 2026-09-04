'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import {
  getESignRequests,
  createESignRequest,
  sendESignRequest,
  deleteESignRequest,
  getClients,
  getServiceTypes,
  ESignRequest,
  Client,
  ServiceType
} from '@/lib/api';
import { SkeletonTable, useToast } from '@/components';

const statusColors: Record<string, string> = {
  pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300',
  signed: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300',
  expired: 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300',
  declined: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300',
};

export default function ESignPage() {
  const { user } = useAuth();
  const toast = useToast();

  const [requests, setRequests] = useState<ESignRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState('');

  // Create modal state
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [clients, setClients] = useState<Client[]>([]);
  const [serviceTypes, setServiceTypes] = useState<ServiceType[]>([]);
  const [creating, setCreating] = useState(false);

  // Create form state
  const [formData, setFormData] = useState({
    client_id: '',
    template_type: 'engagement',
    signer_email: '',
    signer_name: '',
    expires_in_days: 7,
    auto_create_service: false,
    service_type_id: '',
  });

  const fetchRequests = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getESignRequests({
        status: statusFilter || undefined,
        limit: 50,
      });
      setRequests(data.e_sign_requests || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load e-sign requests');
    } finally {
      setLoading(false);
    }
  }, [statusFilter]);

  useEffect(() => {
    fetchRequests();
  }, [fetchRequests]);

  const loadCreateFormData = async () => {
    try {
      const [clientsRes, typesRes] = await Promise.all([
        getClients({ limit: 200 }),
        getServiceTypes({ limit: 100, active: true }),
      ]);
      setClients(clientsRes.clients || []);
      setServiceTypes(typesRes.service_types || []);
    } catch (err) {
      toast.error('Failed to load form data');
    }
  };

  const openCreateModal = async () => {
    await loadCreateFormData();
    setShowCreateModal(true);
  };

  const handleCreate = async () => {
    if (!formData.client_id || !formData.signer_email) {
      toast.error('Please fill in required fields');
      return;
    }

    setCreating(true);
    try {
      const result = await createESignRequest({
        client_id: formData.client_id,
        template_type: formData.template_type,
        signer_email: formData.signer_email,
        signer_name: formData.signer_name || undefined,
        expires_in_days: formData.expires_in_days,
        auto_create_service: formData.auto_create_service,
        service_type_id: formData.auto_create_service ? formData.service_type_id : undefined,
      });

      toast.success('E-Sign request created');
      setShowCreateModal(false);
      setFormData({
        client_id: '',
        template_type: 'engagement',
        signer_email: '',
        signer_name: '',
        expires_in_days: 7,
        auto_create_service: false,
        service_type_id: '',
      });
      fetchRequests();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create request');
    } finally {
      setCreating(false);
    }
  };

  const handleResend = async (id: string) => {
    try {
      await sendESignRequest(id);
      toast.success('E-Sign request resent');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to resend');
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to cancel this e-sign request?')) return;

    try {
      await deleteESignRequest(id);
      setRequests(prev => prev.filter(r => r.id !== id));
      toast.success('E-Sign request cancelled');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to cancel');
    }
  };

  const canManage = user?.role === 'super_admin' || user?.role === 'tenant_admin';

  return (
    <div className="p-6">
      {/* Header */}
      <div className="mb-6 flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">E-Sign Requests</h1>
          <p className="text-gray-500 dark:text-gray-400 mt-1">
            Manage electronic signature requests for clients
          </p>
        </div>
        {canManage && (
          <button
            onClick={openCreateModal}
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
          >
            <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            New E-Sign Request
          </button>
        )}
      </div>

      {/* Filters */}
      <div className="mb-6">
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
        >
          <option value="">All statuses</option>
          <option value="pending">Pending</option>
          <option value="signed">Signed</option>
          <option value="expired">Expired</option>
          <option value="declined">Declined</option>
        </select>
      </div>

      {/* Error */}
      {error && (
        <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
          <p className="text-red-700 dark:text-red-300">{error}</p>
        </div>
      )}

      {/* Table */}
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
        {loading ? (
          <div className="p-6">
            <SkeletonTable rows={5} columns={6} />
          </div>
        ) : requests.length === 0 ? (
          <div className="p-12 text-center">
            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white">No e-sign requests</h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Get started by creating a new e-signature request.
            </p>
            {canManage && (
              <button
                onClick={openCreateModal}
                className="mt-4 inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700"
              >
                New E-Sign Request
              </button>
            )}
          </div>
        ) : (
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-slate-700">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Client</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Template</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Signer</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Status</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Sent</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Expires</th>
                {canManage && (
                  <th className="relative px-6 py-3">
                    <span className="sr-only">Actions</span>
                  </th>
                )}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {requests.map((request) => (
                <tr key={request.id} className="hover:bg-gray-50 dark:hover:bg-slate-700">
                  <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">
                    {request.client_name || '-'}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400 capitalize">
                    {request.template_type}
                  </td>
                  <td className="px-6 py-4">
                    <div className="text-sm text-gray-900 dark:text-white">{request.signer_name || '-'}</div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">{request.signer_email}</div>
                  </td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs font-medium rounded-full ${statusColors[request.status] || ''}`}>
                      {request.status.charAt(0).toUpperCase() + request.status.slice(1)}
                    </span>
                    {request.auto_create_service && (
                      <span className="ml-2 px-2 py-1 text-xs font-medium rounded-full bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300">
                        Auto-Service
                      </span>
                    )}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {request.sent_at ? new Date(request.sent_at).toLocaleDateString() : '-'}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {request.expires_at ? new Date(request.expires_at).toLocaleDateString() : '-'}
                  </td>
                  {canManage && (
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      {request.status === 'pending' && (
                        <div className="flex space-x-2 justify-end">
                          <button
                            onClick={() => handleResend(request.id)}
                            className="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300"
                          >
                            Resend
                          </button>
                          <button
                            onClick={() => handleDelete(request.id)}
                            className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                          >
                            Cancel
                          </button>
                        </div>
                      )}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Create Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-lg mx-4 p-6">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              New E-Sign Request
            </h2>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Client *
                </label>
                <select
                  value={formData.client_id}
                  onChange={(e) => {
                    const client = clients.find(c => c.id === e.target.value);
                    setFormData({
                      ...formData,
                      client_id: e.target.value,
                      signer_email: client?.email || formData.signer_email,
                      signer_name: client?.contact_name || formData.signer_name,
                    });
                  }}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                >
                  <option value="">Select client</option>
                  {clients.map((client) => (
                    <option key={client.id} value={client.id}>{client.company_name}</option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Template Type
                </label>
                <select
                  value={formData.template_type}
                  onChange={(e) => setFormData({ ...formData, template_type: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                >
                  <option value="engagement">Engagement Letter</option>
                  <option value="nda">NDA</option>
                  <option value="authorization">Authorization</option>
                  <option value="consent">Consent Form</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Signer Email *
                </label>
                <input
                  type="email"
                  value={formData.signer_email}
                  onChange={(e) => setFormData({ ...formData, signer_email: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Signer Name
                </label>
                <input
                  type="text"
                  value={formData.signer_name}
                  onChange={(e) => setFormData({ ...formData, signer_name: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Expires In (days)
                </label>
                <input
                  type="number"
                  min="1"
                  max="30"
                  value={formData.expires_in_days}
                  onChange={(e) => setFormData({ ...formData, expires_in_days: parseInt(e.target.value) || 7 })}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>

              <div className="flex items-center">
                <input
                  type="checkbox"
                  id="auto_create_service"
                  checked={formData.auto_create_service}
                  onChange={(e) => setFormData({ ...formData, auto_create_service: e.target.checked })}
                  className="w-4 h-4 text-blue-600 rounded border-gray-300 dark:border-gray-600"
                />
                <label htmlFor="auto_create_service" className="ml-2 text-sm text-gray-700 dark:text-gray-300">
                  Auto-create service when signed
                </label>
              </div>

              {formData.auto_create_service && (
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Service Type
                  </label>
                  <select
                    value={formData.service_type_id}
                    onChange={(e) => setFormData({ ...formData, service_type_id: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                  >
                    <option value="">Select service type</option>
                    {serviceTypes.map((type) => (
                      <option key={type.id} value={type.id}>{type.name}</option>
                    ))}
                  </select>
                </div>
              )}
            </div>

            <div className="mt-6 flex justify-end space-x-3">
              <button
                onClick={() => setShowCreateModal(false)}
                className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-md"
              >
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={creating}
                className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {creating ? 'Creating...' : 'Create & Send'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
