'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/components/auth-guard';
import {
  getClients,
  getDocumentTypes,
  bulkRequestDocuments,
  Client,
  DocumentType
} from '@/lib/api';
import { useToast } from '@/components';

interface DocumentRequest {
  id: string;
  name: string;
  type_id: string;
  expiry_date: string;
}

export default function DocumentRequestPage() {
  const { user } = useAuth();
  const router = useRouter();
  const toast = useToast();

  const [clients, setClients] = useState<Client[]>([]);
  const [documentTypes, setDocumentTypes] = useState<DocumentType[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  // Form state
  const [selectedClients, setSelectedClients] = useState<string[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [requestNote, setRequestNote] = useState('');
  const [documentRequests, setDocumentRequests] = useState<DocumentRequest[]>([
    { id: '1', name: '', type_id: '', expiry_date: '' }
  ]);

  useEffect(() => {
    async function loadData() {
      try {
        const [clientsRes, typesRes] = await Promise.all([
          getClients({ limit: 200 }),
          getDocumentTypes({ limit: 100, active: true }),
        ]);
        setClients(clientsRes.clients || []);
        setDocumentTypes(typesRes.document_types || []);
      } catch (err) {
        toast.error('Failed to load data');
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [toast]);

  const filteredClients = clients.filter(client =>
    client.company_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    client.email.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const toggleClient = (clientId: string) => {
    setSelectedClients(prev =>
      prev.includes(clientId)
        ? prev.filter(id => id !== clientId)
        : [...prev, clientId]
    );
  };

  const selectAllClients = () => {
    if (selectedClients.length === filteredClients.length) {
      setSelectedClients([]);
    } else {
      setSelectedClients(filteredClients.map(c => c.id));
    }
  };

  const addDocumentRequest = () => {
    setDocumentRequests(prev => [
      ...prev,
      { id: Date.now().toString(), name: '', type_id: '', expiry_date: '' }
    ]);
  };

  const removeDocumentRequest = (id: string) => {
    if (documentRequests.length > 1) {
      setDocumentRequests(prev => prev.filter(r => r.id !== id));
    }
  };

  const updateDocumentRequest = (id: string, field: keyof DocumentRequest, value: string) => {
    setDocumentRequests(prev =>
      prev.map(r => r.id === id ? { ...r, [field]: value } : r)
    );
  };

  const handleSubmit = async () => {
    if (selectedClients.length === 0) {
      toast.error('Please select at least one client');
      return;
    }

    const validRequests = documentRequests.filter(r => r.name.trim());
    if (validRequests.length === 0) {
      toast.error('Please add at least one document request');
      return;
    }

    setSubmitting(true);
    try {
      const result = await bulkRequestDocuments({
        client_ids: selectedClients,
        document_requests: validRequests.map(r => ({
          name: r.name,
          type_id: r.type_id || undefined,
          expiry_date: r.expiry_date || undefined,
        })),
        request_note: requestNote || undefined,
      });

      toast.success(`Created ${result.created} document request(s)`);
      router.push('/dashboard/documents');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create requests');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="p-6 flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Request Documents</h1>
        <p className="text-gray-500 dark:text-gray-400 mt-1">
          Request documents from multiple clients at once
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Left Column - Client Selection */}
        <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Select Clients
          </h2>

          {/* Search */}
          <div className="mb-4">
            <input
              type="text"
              placeholder="Search clients..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
            />
          </div>

          {/* Select All */}
          <div className="mb-3 flex items-center justify-between">
            <label className="flex items-center space-x-2 cursor-pointer">
              <input
                type="checkbox"
                checked={selectedClients.length === filteredClients.length && filteredClients.length > 0}
                onChange={selectAllClients}
                className="w-4 h-4 text-blue-600 rounded border-gray-300 dark:border-gray-600"
              />
              <span className="text-sm text-gray-600 dark:text-gray-400">Select All</span>
            </label>
            <span className="text-sm text-gray-500 dark:text-gray-400">
              {selectedClients.length} selected
            </span>
          </div>

          {/* Client List */}
          <div className="border border-gray-200 dark:border-gray-700 rounded-lg max-h-96 overflow-y-auto">
            {filteredClients.length === 0 ? (
              <div className="p-4 text-center text-gray-500 dark:text-gray-400">
                No clients found
              </div>
            ) : (
              <ul className="divide-y divide-gray-200 dark:divide-gray-700">
                {filteredClients.map((client) => (
                  <li
                    key={client.id}
                    onClick={() => toggleClient(client.id)}
                    className={`p-3 cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700 transition-colors ${
                      selectedClients.includes(client.id) ? 'bg-blue-50 dark:bg-blue-900/20' : ''
                    }`}
                  >
                    <div className="flex items-center space-x-3">
                      <input
                        type="checkbox"
                        checked={selectedClients.includes(client.id)}
                        onChange={() => {}}
                        className="w-4 h-4 text-blue-600 rounded border-gray-300 dark:border-gray-600"
                      />
                      <div>
                        <p className="text-sm font-medium text-gray-900 dark:text-white">
                          {client.company_name}
                        </p>
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                          {client.email}
                        </p>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>

        {/* Right Column - Document Requests */}
        <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            Documents to Request
          </h2>

          {/* Document Request List */}
          <div className="space-y-4">
            {documentRequests.map((request, index) => (
              <div key={request.id} className="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
                <div className="flex justify-between items-center mb-3">
                  <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                    Document {index + 1}
                  </span>
                  {documentRequests.length > 1 && (
                    <button
                      onClick={() => removeDocumentRequest(request.id)}
                      className="text-red-500 hover:text-red-700"
                    >
                      <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  )}
                </div>

                <div className="space-y-3">
                  <input
                    type="text"
                    placeholder="Document name *"
                    value={request.name}
                    onChange={(e) => updateDocumentRequest(request.id, 'name', e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                  />

                  <select
                    value={request.type_id}
                    onChange={(e) => updateDocumentRequest(request.id, 'type_id', e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                  >
                    <option value="">Select type (optional)</option>
                    {documentTypes.map((type) => (
                      <option key={type.id} value={type.id}>{type.name}</option>
                    ))}
                  </select>

                  <input
                    type="date"
                    value={request.expiry_date}
                    onChange={(e) => updateDocumentRequest(request.id, 'expiry_date', e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                    placeholder="Expiry date (optional)"
                  />
                </div>
              </div>
            ))}
          </div>

          <button
            onClick={addDocumentRequest}
            className="mt-4 w-full py-2 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg text-gray-600 dark:text-gray-400 hover:border-blue-500 hover:text-blue-500 transition-colors"
          >
            + Add Another Document
          </button>

          {/* Request Note */}
          <div className="mt-6">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Note to Clients (Optional)
            </label>
            <textarea
              value={requestNote}
              onChange={(e) => setRequestNote(e.target.value)}
              rows={3}
              placeholder="Add a message that will be included in the request email..."
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
            />
          </div>

          {/* Summary */}
          <div className="mt-6 p-4 bg-gray-50 dark:bg-slate-700 rounded-lg">
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Summary</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              {selectedClients.length} client(s) x {documentRequests.filter(r => r.name.trim()).length} document(s)
              = <strong>{selectedClients.length * documentRequests.filter(r => r.name.trim()).length}</strong> total requests
            </p>
          </div>

          {/* Actions */}
          <div className="mt-6 flex justify-end space-x-3">
            <button
              onClick={() => router.push('/dashboard/documents')}
              className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-md"
            >
              Cancel
            </button>
            <button
              onClick={handleSubmit}
              disabled={submitting || selectedClients.length === 0}
              className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
            >
              {submitting ? (
                <>
                  <svg className="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Sending...
                </>
              ) : (
                <>
                  <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                  </svg>
                  Send Requests
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
