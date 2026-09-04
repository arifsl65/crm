'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import {
  getFirmDocuments,
  updateFirmDocumentAccess,
  Document
} from '@/lib/api';
import { getStatusBadgeClass, formatStatus } from '@/lib/status';
import { SkeletonTable, useToast } from '@/components';

export default function FirmDocumentsPage() {
  const { user } = useAuth();
  const toast = useToast();

  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');

  // Access control modal state
  const [selectedDoc, setSelectedDoc] = useState<Document | null>(null);
  const [showAccessModal, setShowAccessModal] = useState(false);
  const [accessLevel, setAccessLevel] = useState<'private' | 'staff' | 'all'>('private');

  const fetchDocuments = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getFirmDocuments({
        search: search || undefined,
        limit: 50,
      });
      setDocuments(data.documents || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load firm documents');
    } finally {
      setLoading(false);
    }
  }, [search]);

  useEffect(() => {
    fetchDocuments();
  }, [fetchDocuments]);

  const formatFileSize = (bytes?: number) => {
    if (!bytes) return '-';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const openAccessModal = (doc: Document) => {
    setSelectedDoc(doc);
    setAccessLevel((doc.access as 'private' | 'staff' | 'all') || 'private');
    setShowAccessModal(true);
  };

  const handleAccessUpdate = async () => {
    if (!selectedDoc) return;

    try {
      await updateFirmDocumentAccess(selectedDoc.id, {
        access: accessLevel,
      });
      setDocuments(prev =>
        prev.map(d => d.id === selectedDoc.id ? { ...d, access: accessLevel } : d)
      );
      toast.success('Access updated');
      setShowAccessModal(false);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update access');
    }
  };

  const getAccessBadge = (access: string) => {
    switch (access) {
      case 'private':
        return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300';
      case 'staff':
        return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300';
      case 'all':
        return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300';
      default:
        return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-300';
    }
  };

  const canManage = user?.role === 'super_admin' || user?.role === 'tenant_admin';

  return (
    <div className="p-6">
      {/* Header */}
      <div className="mb-6 flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Firm Documents</h1>
          <p className="text-gray-500 dark:text-gray-400 mt-1">
            Internal documents not associated with any client
          </p>
        </div>
        {canManage && (
          <Link
            href="/dashboard/documents/upload"
            className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
          >
            <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
            </svg>
            Upload
          </Link>
        )}
      </div>

      {/* Search */}
      <div className="mb-6">
        <input
          type="text"
          placeholder="Search firm documents..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-full max-w-md px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
        />
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
            <SkeletonTable rows={5} columns={5} />
          </div>
        ) : documents.length === 0 ? (
          <div className="p-12 text-center">
            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
            </svg>
            <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white">No firm documents</h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Upload internal documents for your firm.
            </p>
            {canManage && (
              <Link
                href="/dashboard/documents/upload"
                className="mt-4 inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700"
              >
                Upload Document
              </Link>
            )}
          </div>
        ) : (
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-slate-700">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Name</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Type</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Access</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Size</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Uploaded</th>
                {canManage && (
                  <th className="relative px-6 py-3">
                    <span className="sr-only">Actions</span>
                  </th>
                )}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {documents.map((doc) => (
                <tr key={doc.id} className="hover:bg-gray-50 dark:hover:bg-slate-700">
                  <td className="px-6 py-4">
                    <div className="flex items-center">
                      <svg className="w-8 h-8 text-gray-400 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                      </svg>
                      <div>
                        <div className="text-sm font-medium text-gray-900 dark:text-white">{doc.name}</div>
                        <div className="text-xs text-gray-500 dark:text-gray-400">{doc.original_name}</div>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {doc.type_name || '-'}
                  </td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs font-medium rounded-full ${getAccessBadge(doc.access)}`}>
                      {doc.access === 'private' ? 'Admin Only' :
                       doc.access === 'staff' ? 'Staff' :
                       doc.access === 'all' ? 'All Users' : doc.access}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {formatFileSize(doc.file_size)}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {new Date(doc.created_at).toLocaleDateString()}
                  </td>
                  {canManage && (
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <button
                        onClick={() => openAccessModal(doc)}
                        className="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300"
                      >
                        Manage Access
                      </button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Access Control Modal */}
      {showAccessModal && selectedDoc && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-md mx-4 p-6">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              Manage Document Access
            </h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              {selectedDoc.name}
            </p>

            <div className="space-y-3">
              <label className="flex items-center p-3 border border-gray-200 dark:border-gray-700 rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700">
                <input
                  type="radio"
                  name="access"
                  value="private"
                  checked={accessLevel === 'private'}
                  onChange={() => setAccessLevel('private')}
                  className="w-4 h-4 text-blue-600"
                />
                <div className="ml-3">
                  <span className="text-sm font-medium text-gray-900 dark:text-white">Admin Only</span>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Only admins can view this document</p>
                </div>
              </label>

              <label className="flex items-center p-3 border border-gray-200 dark:border-gray-700 rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700">
                <input
                  type="radio"
                  name="access"
                  value="staff"
                  checked={accessLevel === 'staff'}
                  onChange={() => setAccessLevel('staff')}
                  className="w-4 h-4 text-blue-600"
                />
                <div className="ml-3">
                  <span className="text-sm font-medium text-gray-900 dark:text-white">All Staff</span>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Admins and staff can view this document</p>
                </div>
              </label>

              <label className="flex items-center p-3 border border-gray-200 dark:border-gray-700 rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700">
                <input
                  type="radio"
                  name="access"
                  value="all"
                  checked={accessLevel === 'all'}
                  onChange={() => setAccessLevel('all')}
                  className="w-4 h-4 text-blue-600"
                />
                <div className="ml-3">
                  <span className="text-sm font-medium text-gray-900 dark:text-white">All Users</span>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Everyone including clients can view this document</p>
                </div>
              </label>
            </div>

            <div className="mt-6 flex justify-end space-x-3">
              <button
                onClick={() => setShowAccessModal(false)}
                className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-md"
              >
                Cancel
              </button>
              <button
                onClick={handleAccessUpdate}
                className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
              >
                Update Access
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
