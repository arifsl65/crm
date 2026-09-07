'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  Document,
  DocumentType,
  Client,
  User,
  getDocuments,
  getDocumentTypes,
  getClients,
  getUsers,
} from '@/lib/api';
import { getStatusBadgeClass, formatStatus } from '@/lib/status';

export default function DocumentsPage() {
  const router = useRouter();
  const [documents, setDocuments] = useState<Document[]>([]);
  const [documentTypes, setDocumentTypes] = useState<DocumentType[]>([]);
  const [clients, setClients] = useState<Client[]>([]);
  const [staff, setStaff] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [clientFilter, setClientFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');

  // Load filter options on mount
  useEffect(() => {
    const loadFilterOptions = async () => {
      try {
        const [typesData, clientsData, staffData] = await Promise.all([
          getDocumentTypes({ active: true, limit: 100 }),
          getClients({ limit: 200 }),
          getUsers({ limit: 100, role: 'staff' }),
        ]);
        setDocumentTypes(typesData.document_types || []);
        setClients(clientsData.clients || []);
        setStaff(staffData.users || []);
      } catch (err) {
        console.error('Failed to load filter options:', err);
      }
    };
    loadFilterOptions();
  }, []);

  const fetchDocuments = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getDocuments({
        search: search || undefined,
        status: statusFilter || undefined,
        client_id: clientFilter || undefined,
        type_id: typeFilter || undefined,
        limit: 100,
      });
      setDocuments(data.documents || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load documents');
    } finally {
      setLoading(false);
    }
  }, [search, statusFilter, clientFilter, typeFilter]);

  useEffect(() => {
    fetchDocuments();
  }, [fetchDocuments]);

  // Get staff name by ID
  const getStaffName = (staffId?: string): string | null => {
    if (!staffId) return null;
    const staffMember = staff.find(s => s.id === staffId);
    return staffMember?.name || null;
  };

  // Format relative time
  const formatRelativeTime = (dateStr: string): string => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
  };

  // Get status emoji
  const getStatusEmoji = (status: string): string => {
    switch (status) {
      case 'requested': return '🔴';
      case 'uploaded': return '🟡';
      case 'pending_review': return '🟡';
      case 'approved': return '🟢';
      case 'rejected': return '⚫';
      default: return '⚪';
    }
  };

  // Handle close - go back to dashboard
  const handleClose = () => {
    router.push('/dashboard');
  };

  return (
    <div className="h-full flex items-center justify-center p-4 bg-gray-100 dark:bg-slate-900">
      {/* Overlay Panel */}
      <div className="w-full max-w-2xl bg-white dark:bg-slate-800 rounded-lg shadow-xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h1 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center">
            <span className="mr-2">📄</span>
            ALL DOCUMENTS
          </h1>
          <button
            onClick={handleClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-2xl font-light"
          >
            ✕
          </button>
        </div>

        {/* Search & Filters */}
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700 space-y-3">
          {/* Search */}
          <div className="relative">
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">🔍</span>
            <input
              type="text"
              placeholder="Search..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
          </div>

          {/* Filter Dropdowns */}
          <div className="flex gap-2">
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white text-sm"
            >
              <option value="">Status ▼</option>
              <option value="requested">🔴 Requested</option>
              <option value="uploaded">🟡 Uploaded</option>
              <option value="pending_review">🟡 Pending Review</option>
              <option value="approved">🟢 Approved</option>
              <option value="rejected">⚫ Rejected</option>
            </select>

            <select
              value={clientFilter}
              onChange={(e) => setClientFilter(e.target.value)}
              className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white text-sm"
            >
              <option value="">Client ▼</option>
              {clients.map((client) => (
                <option key={client.id} value={client.id}>
                  {client.company_name}
                </option>
              ))}
            </select>

            <select
              value={typeFilter}
              onChange={(e) => setTypeFilter(e.target.value)}
              className="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white text-sm"
            >
              <option value="">Category ▼</option>
              {documentTypes.map((type) => (
                <option key={type.id} value={type.id}>
                  {type.name}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Document Cards */}
        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-3">
          {error && (
            <div className="p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
              <p className="text-red-700 dark:text-red-300">{error}</p>
            </div>
          )}

          {loading ? (
            <div className="space-y-3">
              {[1, 2, 3, 4, 5].map((i) => (
                <div key={i} className="animate-pulse bg-gray-100 dark:bg-slate-700 rounded-lg p-4">
                  <div className="h-4 bg-gray-200 dark:bg-slate-600 rounded w-3/4 mb-2"></div>
                  <div className="h-3 bg-gray-200 dark:bg-slate-600 rounded w-1/2"></div>
                </div>
              ))}
            </div>
          ) : documents.length === 0 ? (
            <div className="text-center py-12">
              <div className="text-4xl mb-3">📄</div>
              <h3 className="text-sm font-medium text-gray-900 dark:text-white">No documents</h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Documents will appear here when requested or uploaded.
              </p>
            </div>
          ) : (
            documents.map((doc) => {
              const assignedStaff = getStaffName(doc.uploaded_by);
              return (
                <Link
                  key={doc.id}
                  href={`/dashboard/documents/${doc.id}`}
                  className="block bg-gray-50 dark:bg-slate-700/50 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg p-4 transition-colors border border-gray-200 dark:border-gray-600"
                >
                  {/* Document Name */}
                  <div className="flex items-start">
                    <span className="text-gray-400 mr-2">📄</span>
                    <span className="font-medium text-gray-900 dark:text-white">
                      {doc.name}
                    </span>
                  </div>

                  {/* Client, Status, Staff */}
                  <div className="mt-2 flex items-center gap-3 text-sm">
                    <span className="text-gray-600 dark:text-gray-300">
                      {doc.client_name || 'No client'}
                    </span>
                    <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(doc.status)}`}>
                      {getStatusEmoji(doc.status)} {formatStatus(doc.status)}
                    </span>
                    {assignedStaff && (
                      <span className="text-gray-500 dark:text-gray-400">
                        → {assignedStaff}
                      </span>
                    )}
                  </div>

                  {/* Type & Time */}
                  <div className="mt-1 flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
                    <span>{doc.type_name || 'Uncategorized'}</span>
                    <span>{formatRelativeTime(doc.created_at)}</span>
                  </div>
                </Link>
              );
            })
          )}
        </div>

        {/* Footer - Document Count */}
        {!loading && documents.length > 0 && (
          <div className="px-6 py-3 border-t border-gray-200 dark:border-gray-700 text-sm text-gray-500 dark:text-gray-400">
            Showing {documents.length} document{documents.length !== 1 ? 's' : ''}
          </div>
        )}
      </div>
    </div>
  );
}
