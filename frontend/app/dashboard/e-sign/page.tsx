'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import {
  ESignRequest,
  getESignRequests,
  createESignRequest,
  sendESignRequest,
  deleteESignRequest,
  getClients,
  Client
} from '@/lib/api';
import { useToast } from '@/components';

export default function ESignPage() {
  const { user } = useAuth();
  const toast = useToast();
  const [requests, setRequests] = useState<ESignRequest[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [sending, setSending] = useState<string | null>(null);

  // Create form state
  const [formClientId, setFormClientId] = useState('');
  const [formTemplateType, setFormTemplateType] = useState('engagement_letter');
  const [formSignerEmail, setFormSignerEmail] = useState('');
  const [formSignerName, setFormSignerName] = useState('');
  const [formExpiresInDays, setFormExpiresInDays] = useState(14);
  const [formAutoCreateService, setFormAutoCreateService] = useState(false);
  const [creating, setCreating] = useState(false);

  const isAdmin = user?.role === 'super_admin' || user?.role === 'tenant_admin';
  const canCreate = isAdmin || user?.role === 'staff';

  const fetchRequests = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getESignRequests({
        limit: 50,
        status: statusFilter || undefined,
      });
      setRequests(data.e_sign_requests);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load e-sign requests');
    } finally {
      setLoading(false);
    }
  }, [statusFilter]);

  const fetchClients = useCallback(async () => {
    try {
      const data = await getClients({ limit: 100 });
      setClients(data.clients);
    } catch (err) {
      console.error('Failed to fetch clients:', err);
    }
  }, []);

  useEffect(() => {
    fetchRequests();
    fetchClients();
  }, [fetchRequests, fetchClients]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formClientId || !formSignerEmail || !formTemplateType) {
      toast.error('Please fill in all required fields');
      return;
    }

    try {
      setCreating(true);
      await createESignRequest({
        client_id: formClientId,
        template_type: formTemplateType,
        signer_email: formSignerEmail,
        signer_name: formSignerName || undefined,
        expires_in_days: formExpiresInDays,
        auto_create_service: formAutoCreateService,
      });
      toast.success('E-sign request created successfully');
      setShowCreateModal(false);
      resetForm();
      fetchRequests();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create e-sign request');
    } finally {
      setCreating(false);
    }
  };

  const handleSend = async (id: string) => {
    try {
      setSending(id);
      await sendESignRequest(id);
      toast.success('E-sign request sent successfully');
      fetchRequests();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to send e-sign request');
    } finally {
      setSending(null);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to cancel this e-sign request?')) return;

    try {
      await deleteESignRequest(id);
      toast.success('E-sign request cancelled');
      fetchRequests();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to cancel e-sign request');
    }
  };

  const resetForm = () => {
    setFormClientId('');
    setFormTemplateType('engagement_letter');
    setFormSignerEmail('');
    setFormSignerName('');
    setFormExpiresInDays(14);
    setFormAutoCreateService(false);
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'pending':
        return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300';
      case 'signed':
        return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300';
      case 'expired':
        return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300';
      case 'declined':
        return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300';
      default:
        return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300';
    }
  };

  const formatTemplateType = (type: string) => {
    return type.split('_').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-slate-900 flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <header className="bg-white dark:bg-slate-800 shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
          <div className="flex items-center space-x-4">
            <Link
              href="/dashboard"
              className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
              </svg>
            </Link>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">E-Sign Requests</h1>
          </div>
          {canCreate && (
            <button
              onClick={() => setShowCreateModal(true)}
              className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700"
            >
              <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              New E-Sign Request
            </button>
          )}
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Filters */}
        <div className="mb-6 flex items-center space-x-4">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500"
          >
            <option value="">All Status</option>
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

        {/* Requests List */}
        <div className="bg-white dark:bg-slate-800 shadow rounded-lg overflow-hidden">
          {requests.length === 0 ? (
            <div className="p-8 text-center text-gray-500 dark:text-gray-400">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <p className="mt-2">No e-sign requests found</p>
              {canCreate && (
                <button
                  onClick={() => setShowCreateModal(true)}
                  className="mt-4 text-blue-600 hover:text-blue-500"
                >
                  Create your first e-sign request
                </button>
              )}
            </div>
          ) : (
            <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
              <thead className="bg-gray-50 dark:bg-slate-700">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Client
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Template
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Signer
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Expires
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white dark:bg-slate-800 divide-y divide-gray-200 dark:divide-gray-700">
                {requests.map((request) => (
                  <tr key={request.id} className="hover:bg-gray-50 dark:hover:bg-slate-700">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm font-medium text-gray-900 dark:text-white">
                        {request.client_name || 'Unknown Client'}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-900 dark:text-white">
                        {formatTemplateType(request.template_type)}
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="text-sm text-gray-900 dark:text-white">
                        {request.signer_name || request.signer_email}
                      </div>
                      {request.signer_name && (
                        <div className="text-sm text-gray-500 dark:text-gray-400">
                          {request.signer_email}
                        </div>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getStatusColor(request.status)}`}>
                        {request.status.charAt(0).toUpperCase() + request.status.slice(1)}
                      </span>
                      {request.sent_at && !request.signed_at && (
                        <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                          Sent {new Date(request.sent_at).toLocaleDateString()}
                        </div>
                      )}
                      {request.signed_at && (
                        <div className="text-xs text-green-600 dark:text-green-400 mt-1">
                          Signed {new Date(request.signed_at).toLocaleDateString()}
                        </div>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                      {request.expires_at ? new Date(request.expires_at).toLocaleDateString() : '-'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <div className="flex items-center space-x-2">
                        {request.status === 'pending' && !request.sent_at && (
                          <button
                            onClick={() => handleSend(request.id)}
                            disabled={sending === request.id}
                            className="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300 disabled:opacity-50"
                          >
                            {sending === request.id ? 'Sending...' : 'Send'}
                          </button>
                        )}
                        {request.status === 'pending' && (
                          <button
                            onClick={() => handleDelete(request.id)}
                            className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                          >
                            Cancel
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </main>

      {/* Create Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-md w-full mx-4">
            <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h3 className="text-lg font-medium text-gray-900 dark:text-white">
                New E-Sign Request
              </h3>
            </div>
            <form onSubmit={handleCreate} className="p-6 space-y-4">
              <div>
                <label htmlFor="client" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Client *
                </label>
                <select
                  id="client"
                  value={formClientId}
                  onChange={(e) => {
                    setFormClientId(e.target.value);
                    // Auto-fill signer email from client
                    const client = clients.find(c => c.id === e.target.value);
                    if (client) {
                      setFormSignerEmail(client.email);
                      setFormSignerName(client.contact_name);
                    }
                  }}
                  required
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500"
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
                <label htmlFor="templateType" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Template Type *
                </label>
                <select
                  id="templateType"
                  value={formTemplateType}
                  onChange={(e) => setFormTemplateType(e.target.value)}
                  required
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500"
                >
                  <option value="engagement_letter">Engagement Letter</option>
                  <option value="aml_declaration">AML Declaration</option>
                  <option value="64_8_form">64-8 Form</option>
                  <option value="consent_form">Consent Form</option>
                  <option value="service_agreement">Service Agreement</option>
                </select>
              </div>

              <div>
                <label htmlFor="signerEmail" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Signer Email *
                </label>
                <input
                  id="signerEmail"
                  type="email"
                  value={formSignerEmail}
                  onChange={(e) => setFormSignerEmail(e.target.value)}
                  required
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500"
                />
              </div>

              <div>
                <label htmlFor="signerName" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Signer Name
                </label>
                <input
                  id="signerName"
                  type="text"
                  value={formSignerName}
                  onChange={(e) => setFormSignerName(e.target.value)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500"
                />
              </div>

              <div>
                <label htmlFor="expiresInDays" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Expires In (days)
                </label>
                <input
                  id="expiresInDays"
                  type="number"
                  min="1"
                  max="90"
                  value={formExpiresInDays}
                  onChange={(e) => setFormExpiresInDays(parseInt(e.target.value) || 14)}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-blue-500 focus:border-blue-500"
                />
              </div>

              <div className="flex items-center">
                <input
                  id="autoCreateService"
                  type="checkbox"
                  checked={formAutoCreateService}
                  onChange={(e) => setFormAutoCreateService(e.target.checked)}
                  className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                />
                <label htmlFor="autoCreateService" className="ml-2 block text-sm text-gray-700 dark:text-gray-300">
                  Auto-create service when signed
                </label>
              </div>

              <div className="flex justify-end space-x-3 pt-4">
                <button
                  type="button"
                  onClick={() => {
                    setShowCreateModal(false);
                    resetForm();
                  }}
                  className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-slate-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50"
                >
                  {creating ? 'Creating...' : 'Create Request'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
