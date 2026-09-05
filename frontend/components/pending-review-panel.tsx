'use client';

import { useEffect, useState, useCallback } from 'react';
import { Document, getDocuments, approveDocument, rejectDocument, bulkApproveDocuments, downloadDocument } from '@/lib/api';
import { useToast } from './toast';
import { getStatusBadgeClass, formatStatus } from '@/lib/status';
import { DocumentPreviewModal } from './document-preview-modal';
import { RejectDocumentModal } from './reject-document-modal';

interface PendingReviewPanelProps {
  isOpen: boolean;
  onClose: () => void;
  onCountChange?: (count: number) => void;
}

export function PendingReviewPanel({ isOpen, onClose, onCountChange }: PendingReviewPanelProps) {
  const toast = useToast();
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedDocs, setSelectedDocs] = useState<Set<string>>(new Set());
  const [approving, setApproving] = useState<string | null>(null);
  const [bulkApproving, setBulkApproving] = useState(false);

  // Preview modal state
  const [previewDoc, setPreviewDoc] = useState<Document | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [loadingPreview, setLoadingPreview] = useState(false);

  // Reject modal state
  const [rejectDoc, setRejectDoc] = useState<Document | null>(null);

  const fetchDocuments = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getDocuments({ status: 'pending_review', limit: 50 });
      setDocuments(data.documents || []);
      onCountChange?.(data.documents?.length || 0);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load documents');
    } finally {
      setLoading(false);
    }
  }, [onCountChange]);

  useEffect(() => {
    if (isOpen) {
      fetchDocuments();
      setSelectedDocs(new Set());
    }
  }, [isOpen, fetchDocuments]);

  const handlePreview = async (doc: Document) => {
    try {
      setLoadingPreview(true);
      setPreviewDoc(doc);
      const result = await downloadDocument(doc.id);
      setPreviewUrl(result.download_url);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to load preview');
      setPreviewDoc(null);
    } finally {
      setLoadingPreview(false);
    }
  };

  const handleApprove = async (doc: Document) => {
    try {
      setApproving(doc.id);
      await approveDocument(doc.id);
      setDocuments(prev => prev.filter(d => d.id !== doc.id));
      onCountChange?.(documents.length - 1);
      toast.success(`Approved: ${doc.name}`);

      // If previewing this doc, close preview
      if (previewDoc?.id === doc.id) {
        setPreviewDoc(null);
        setPreviewUrl(null);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to approve');
    } finally {
      setApproving(null);
    }
  };

  const handleRejectSubmit = async (note: string) => {
    if (!rejectDoc) return;

    try {
      await rejectDocument(rejectDoc.id, note);
      setDocuments(prev => prev.filter(d => d.id !== rejectDoc.id));
      onCountChange?.(documents.length - 1);
      toast.success(`Rejected: ${rejectDoc.name}`);

      // If previewing this doc, close preview
      if (previewDoc?.id === rejectDoc.id) {
        setPreviewDoc(null);
        setPreviewUrl(null);
      }

      setRejectDoc(null);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to reject');
    }
  };

  const handleBulkApprove = async () => {
    if (selectedDocs.size === 0) return;

    try {
      setBulkApproving(true);
      const result = await bulkApproveDocuments(Array.from(selectedDocs));
      setDocuments(prev => prev.filter(d => !selectedDocs.has(d.id)));
      onCountChange?.(documents.length - result.approved);
      setSelectedDocs(new Set());
      toast.success(`Approved ${result.approved} document${result.approved > 1 ? 's' : ''}`);
      if (result.failed > 0) {
        toast.error(`Failed to approve ${result.failed} document${result.failed > 1 ? 's' : ''}`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to bulk approve');
    } finally {
      setBulkApproving(false);
    }
  };

  const handleApproveAll = async () => {
    const allIds = documents.map(d => d.id);
    if (allIds.length === 0) return;

    try {
      setBulkApproving(true);
      const result = await bulkApproveDocuments(allIds);
      setDocuments([]);
      onCountChange?.(0);
      setSelectedDocs(new Set());
      toast.success(`Approved all ${result.approved} document${result.approved > 1 ? 's' : ''}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to approve all');
    } finally {
      setBulkApproving(false);
    }
  };

  const toggleSelect = (id: string) => {
    setSelectedDocs(prev => {
      const newSet = new Set(prev);
      if (newSet.has(id)) {
        newSet.delete(id);
      } else {
        newSet.add(id);
      }
      return newSet;
    });
  };

  const toggleSelectAll = () => {
    if (selectedDocs.size === documents.length) {
      setSelectedDocs(new Set());
    } else {
      setSelectedDocs(new Set(documents.map(d => d.id)));
    }
  };

  const formatFileSize = (bytes?: number) => {
    if (!bytes) return '-';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  if (!isOpen) return null;

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/30 z-40 transition-opacity"
        onClick={onClose}
      />

      {/* Slide-out Panel */}
      <div className="fixed right-0 top-0 h-full w-full max-w-xl bg-white dark:bg-slate-800 shadow-xl z-50 flex flex-col transform transition-transform">
        {/* Header */}
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <span className="text-xl">📄</span> Pending Review
              <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 rounded-full">
                {documents.length}
              </span>
            </h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              Review and approve client documents
            </p>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-700"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Bulk Actions Bar */}
        {documents.length > 0 && (
          <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-slate-700/50 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <input
                type="checkbox"
                checked={selectedDocs.size === documents.length && documents.length > 0}
                onChange={toggleSelectAll}
                className="w-4 h-4 text-blue-600 rounded border-gray-300 dark:border-gray-600 focus:ring-blue-500"
              />
              <span className="text-sm text-gray-600 dark:text-gray-300">
                {selectedDocs.size > 0 ? `${selectedDocs.size} selected` : 'Select all'}
              </span>
            </div>
            <div className="flex items-center gap-2">
              {selectedDocs.size > 0 && (
                <button
                  onClick={handleBulkApprove}
                  disabled={bulkApproving}
                  className="px-3 py-1.5 text-sm font-medium text-white bg-green-600 hover:bg-green-700 disabled:bg-green-400 rounded-md flex items-center gap-1"
                >
                  {bulkApproving ? (
                    <span className="animate-spin">⏳</span>
                  ) : (
                    <span>✓</span>
                  )}
                  Approve Selected
                </button>
              )}
              <button
                onClick={handleApproveAll}
                disabled={bulkApproving || documents.length === 0}
                className="px-3 py-1.5 text-sm font-medium text-green-600 dark:text-green-400 border border-green-600 dark:border-green-400 hover:bg-green-50 dark:hover:bg-green-900/20 disabled:opacity-50 rounded-md"
              >
                Approve All
              </button>
            </div>
          </div>
        )}

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-4">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
          ) : error ? (
            <div className="p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg">
              <p className="text-red-700 dark:text-red-300">{error}</p>
              <button
                onClick={fetchDocuments}
                className="mt-2 text-sm text-red-600 dark:text-red-400 underline"
              >
                Try again
              </button>
            </div>
          ) : documents.length === 0 ? (
            <div className="text-center py-12">
              <div className="mx-auto w-16 h-16 bg-green-100 dark:bg-green-900/30 rounded-full flex items-center justify-center mb-4">
                <span className="text-3xl">✓</span>
              </div>
              <h3 className="text-lg font-medium text-gray-900 dark:text-white">All caught up!</h3>
              <p className="text-gray-500 dark:text-gray-400 mt-1">No documents pending review</p>
            </div>
          ) : (
            <div className="space-y-3">
              {documents.map((doc) => (
                <div
                  key={doc.id}
                  className={`p-4 bg-gray-50 dark:bg-slate-700 rounded-lg border transition-colors ${
                    selectedDocs.has(doc.id)
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                      : 'border-gray-200 dark:border-gray-600'
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <input
                      type="checkbox"
                      checked={selectedDocs.has(doc.id)}
                      onChange={() => toggleSelect(doc.id)}
                      className="mt-1 w-4 h-4 text-blue-600 rounded border-gray-300 dark:border-gray-600 focus:ring-blue-500"
                    />

                    {/* Document Icon */}
                    <div className="flex-shrink-0 w-10 h-10 bg-gray-200 dark:bg-slate-600 rounded-lg flex items-center justify-center">
                      <svg className="w-5 h-5 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                      </svg>
                    </div>

                    {/* Document Info */}
                    <div className="flex-1 min-w-0">
                      <h4 className="text-sm font-medium text-gray-900 dark:text-white truncate">
                        {doc.name}
                      </h4>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                        {doc.client_name || 'Unknown Client'} • {formatFileSize(doc.file_size)} • {doc.type_name || 'Document'}
                      </p>
                      <p className="text-xs text-gray-400 dark:text-gray-500 mt-0.5">
                        Uploaded {new Date(doc.created_at).toLocaleDateString()}
                      </p>
                    </div>

                    {/* Actions */}
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handlePreview(doc)}
                        disabled={loadingPreview && previewDoc?.id === doc.id}
                        className="p-2 text-gray-500 hover:text-blue-600 dark:text-gray-400 dark:hover:text-blue-400 hover:bg-gray-100 dark:hover:bg-slate-600 rounded-lg transition-colors"
                        title="Preview"
                      >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                        </svg>
                      </button>
                      <button
                        onClick={() => handleApprove(doc)}
                        disabled={approving === doc.id}
                        className="p-2 text-gray-500 hover:text-green-600 dark:text-gray-400 dark:hover:text-green-400 hover:bg-green-50 dark:hover:bg-green-900/20 rounded-lg transition-colors"
                        title="Approve"
                      >
                        {approving === doc.id ? (
                          <span className="animate-spin block w-5 h-5">⏳</span>
                        ) : (
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                          </svg>
                        )}
                      </button>
                      <button
                        onClick={() => setRejectDoc(doc)}
                        className="p-2 text-gray-500 hover:text-red-600 dark:text-gray-400 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
                        title="Reject"
                      >
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Preview Modal */}
      <DocumentPreviewModal
        isOpen={!!previewDoc}
        document={previewDoc}
        previewUrl={previewUrl}
        loading={loadingPreview}
        onClose={() => {
          setPreviewDoc(null);
          setPreviewUrl(null);
        }}
        onApprove={() => previewDoc && handleApprove(previewDoc)}
        onReject={() => previewDoc && setRejectDoc(previewDoc)}
        approving={approving === previewDoc?.id}
      />

      {/* Reject Modal */}
      <RejectDocumentModal
        isOpen={!!rejectDoc}
        documentName={rejectDoc?.name || ''}
        onClose={() => setRejectDoc(null)}
        onSubmit={handleRejectSubmit}
      />
    </>
  );
}
