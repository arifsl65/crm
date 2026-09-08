'use client';

import { useEffect, useState, useCallback } from 'react';
import { useRouter, useParams, usePathname } from 'next/navigation';
import {
  Document,
  DocumentVersion,
  getDocument,
  getDocumentVersions,
  downloadDocument,
  approveDocument,
  rejectDocument,
} from '@/lib/api';
import { getStatusBadgeClass, formatStatus } from '@/lib/status';
import { DocumentPreviewModal } from '@/components';

type Tab = 'info' | 'versions';

export default function DocumentDetail() {
  const router = useRouter();
  const params = useParams();
  const pathname = usePathname();

  // Extract document ID from pathname for static export compatibility
  // useParams() returns 'placeholder' during static export, so we parse the URL instead
  const documentId = pathname?.split('/').pop() || (params.id as string);

  const [document, setDocument] = useState<Document | null>(null);
  const [versions, setVersions] = useState<DocumentVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('info');
  const [reviewNote, setReviewNote] = useState('');
  const [actionLoading, setActionLoading] = useState(false);
  const [downloadLoading, setDownloadLoading] = useState(false);
  const [showPreview, setShowPreview] = useState(false);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const fetchDocument = useCallback(async () => {
    try {
      setLoading(true);
      const doc = await getDocument(documentId);
      setDocument(doc);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load document');
    } finally {
      setLoading(false);
    }
  }, [documentId]);

  const fetchVersions = useCallback(async () => {
    try {
      const data = await getDocumentVersions(documentId);
      setVersions(data.versions || []);
    } catch (err) {
      console.error('Failed to load versions:', err);
    }
  }, [documentId]);

  useEffect(() => {
    // Skip fetch if documentId is the static placeholder (happens during hydration)
    if (!documentId || documentId === 'placeholder') return;

    fetchDocument();
    fetchVersions();
  }, [documentId, fetchDocument, fetchVersions]);

  const formatRelativeTime = (dateStr: string): string => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);
    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return diffMins + 'm ago';
    if (diffHours < 24) return diffHours + 'h ago';
    if (diffDays < 7) return diffDays + 'd ago';
    return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return Math.round(bytes / 1024) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  };

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

  const handleClose = () => router.push('/dashboard/documents');

  const handleDownload = async () => {
    try {
      setDownloadLoading(true);
      const { download_url } = await downloadDocument(documentId);
      window.open(download_url, '_blank');
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to download');
    } finally {
      setDownloadLoading(false);
    }
  };

  const handlePreview = async () => {
    try {
      setPreviewLoading(true);
      setShowPreview(true);
      const { download_url } = await downloadDocument(documentId);
      setPreviewUrl(download_url);
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to preview');
      setShowPreview(false);
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleClosePreview = () => {
    setShowPreview(false);
    setPreviewUrl(null);
  };

  const handleApprove = async () => {
    if (!confirm('Approve this document?')) return;
    try {
      setActionLoading(true);
      await approveDocument(documentId, reviewNote || undefined);
      await fetchDocument();
      setReviewNote('');
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to approve');
    } finally {
      setActionLoading(false);
    }
  };

  const handleReject = async () => {
    if (!reviewNote.trim()) { alert('Please provide a reason for rejection'); return; }
    if (!confirm('Reject this document?')) return;
    try {
      setActionLoading(true);
      await rejectDocument(documentId, reviewNote);
      await fetchDocument();
      setReviewNote('');
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to reject');
    } finally {
      setActionLoading(false);
    }
  };

  const canReview = document?.status === 'pending_review' || document?.status === 'uploaded';

  return (
    <div className="h-full flex items-center justify-center p-4 bg-gray-100 dark:bg-slate-900">
      <div className="w-full max-w-2xl bg-white dark:bg-slate-800 rounded-lg shadow-xl overflow-hidden flex flex-col max-h-[90vh]">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h1 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center truncate">
            <span className="mr-2">📄</span>
            <span className="truncate">{document?.name || 'Loading...'}</span>
          </h1>
          <button onClick={handleClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-2xl font-light ml-4">✕</button>
        </div>

        <div className="px-6 pt-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex gap-6">
            <button onClick={() => setActiveTab('info')} className={`pb-3 text-sm font-medium border-b-2 transition-colors ${activeTab === 'info' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'}`}>Info</button>
            <button onClick={() => setActiveTab('versions')} className={`pb-3 text-sm font-medium border-b-2 transition-colors ${activeTab === 'versions' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'}`}>Versions</button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4">
          {error && <div className="p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md mb-4"><p className="text-red-700 dark:text-red-300">{error}</p></div>}

          {loading ? (
            <div className="space-y-4">
              <div className="animate-pulse bg-gray-100 dark:bg-slate-700 rounded-lg p-4 h-48"></div>
              <div className="animate-pulse bg-gray-100 dark:bg-slate-700 rounded-lg p-4 h-32"></div>
            </div>
          ) : document && activeTab === 'info' ? (
            <div className="space-y-6">
              <div>
                <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 mb-3 flex items-center"><span className="mr-2">📌</span> DOCUMENT INFO</h2>
                <div className="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-4 space-y-3">
                  <div className="flex justify-between"><span className="text-gray-500 dark:text-gray-400">Client</span><span className="text-gray-900 dark:text-white font-medium">{document.client_name || 'N/A'}</span></div>
                  <div className="flex justify-between"><span className="text-gray-500 dark:text-gray-400">Category</span><span className="text-gray-900 dark:text-white">{document.type_name || 'Uncategorized'}</span></div>
                  <div className="flex justify-between items-center"><span className="text-gray-500 dark:text-gray-400">Status</span><span className={`px-2 py-0.5 rounded-full text-xs font-medium ${getStatusBadgeClass(document.status)}`}>{getStatusEmoji(document.status)} {formatStatus(document.status)}</span></div>
                  <div className="flex justify-between"><span className="text-gray-500 dark:text-gray-400">Uploaded</span><span className="text-gray-900 dark:text-white">{new Date(document.created_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })}<span className="text-gray-400 ml-1">({formatRelativeTime(document.created_at)})</span></span></div>
                  <div className="flex justify-between"><span className="text-gray-500 dark:text-gray-400">By</span><span className="text-gray-900 dark:text-white">{document.uploaded_by_name || 'Unknown'}</span></div>
                  <div className="flex justify-between"><span className="text-gray-500 dark:text-gray-400">Size</span><span className="text-gray-900 dark:text-white">{formatFileSize(document.file_size || 0)}</span></div>
                  <div className="flex justify-between"><span className="text-gray-500 dark:text-gray-400">Version</span><span className="text-gray-900 dark:text-white">v{document.version || 1}</span></div>
                </div>
                <div className="flex justify-end gap-3 mt-4">
                  <button onClick={handleDownload} disabled={downloadLoading} className="px-4 py-2 text-sm bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-200 dark:hover:bg-slate-600 transition-colors disabled:opacity-50">{downloadLoading ? '...' : '⬇️ Download'}</button>
                  <button onClick={handlePreview} disabled={previewLoading} className="px-4 py-2 text-sm bg-gray-100 dark:bg-slate-700 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-200 dark:hover:bg-slate-600 transition-colors disabled:opacity-50">{previewLoading ? '...' : '👁️ Preview'}</button>
                </div>
              </div>

              {document.ai_summary && (
                <div>
                  <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 mb-3 flex items-center"><span className="mr-2">🤖</span> AI SUMMARY</h2>
                  <div className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4"><p className="text-gray-700 dark:text-gray-300 text-sm leading-relaxed">{document.ai_summary}</p></div>
                </div>
              )}

              {canReview && (
                <div>
                  <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 mb-3 flex items-center"><span className="mr-2">✏️</span> REVIEW</h2>
                  <div className="space-y-3">
                    <div>
                      <label className="block text-sm text-gray-600 dark:text-gray-400 mb-1">Note:</label>
                      <textarea value={reviewNote} onChange={(e) => setReviewNote(e.target.value)} placeholder="Add review note (required for rejection)..." className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none" rows={2} />
                    </div>
                    <div className="flex justify-end gap-3">
                      <button onClick={handleReject} disabled={actionLoading} className="px-4 py-2 text-sm bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 rounded-md hover:bg-red-200 dark:hover:bg-red-900/50 transition-colors disabled:opacity-50">{actionLoading ? '...' : '❌ Reject'}</button>
                      <button onClick={handleApprove} disabled={actionLoading} className="px-4 py-2 text-sm bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded-md hover:bg-green-200 dark:hover:bg-green-900/50 transition-colors disabled:opacity-50">{actionLoading ? '...' : '✅ Approve'}</button>
                    </div>
                  </div>
                </div>
              )}

              {document.status === 'approved' && <div className="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-4"><p className="text-green-700 dark:text-green-300 text-sm">✅ This document has been approved.</p></div>}
              {document.status === 'rejected' && <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4"><p className="text-red-700 dark:text-red-300 text-sm">❌ This document has been rejected.</p></div>}
            </div>
          ) : activeTab === 'versions' ? (
            <div>
              <h2 className="text-sm font-semibold text-gray-500 dark:text-gray-400 mb-3 flex items-center"><span className="mr-2">📜</span> VERSION HISTORY</h2>
              {versions.length === 0 ? (
                <div className="text-center py-8 text-gray-500 dark:text-gray-400">No version history available</div>
              ) : (
                <div className="space-y-3">
                  {versions.map((version, index) => (
                    <div key={version.id} className="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-4 border border-gray-200 dark:border-gray-600">
                      <div className="flex items-center justify-between mb-2">
                        <span className="font-medium text-gray-900 dark:text-white">v{version.version} {index === 0 && <span className="text-xs text-blue-500">(Current)</span>}</span>
                        <span className="text-sm text-gray-500 dark:text-gray-400">{new Date(version.created_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })}</span>
                      </div>
                      <div className="text-sm text-gray-600 dark:text-gray-400 space-y-1">
                        <div>By: {version.uploaded_by_name || 'Unknown'}</div>
                        <div>Size: {formatFileSize(version.file_size || 0)}</div>
                      </div>
                      <div className="flex justify-end gap-2 mt-3">
                        <button onClick={handleDownload} className="px-3 py-1 text-xs bg-gray-100 dark:bg-slate-600 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-200 dark:hover:bg-slate-500 transition-colors">Download</button>
                        <button onClick={handlePreview} className="px-3 py-1 text-xs bg-gray-100 dark:bg-slate-600 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-200 dark:hover:bg-slate-500 transition-colors">View</button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
              {versions.length > 0 && <div className="mt-4 text-sm text-gray-500 dark:text-gray-400 text-center">ℹ️ {versions.length} version{versions.length !== 1 ? 's' : ''} · First: {new Date(versions[versions.length - 1]?.created_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })}</div>}
            </div>
          ) : null}
        </div>
      </div>

      {/* Document Preview Modal */}
      <DocumentPreviewModal
        isOpen={showPreview}
        document={document}
        previewUrl={previewUrl}
        loading={previewLoading}
        onClose={handleClosePreview}
        onApprove={handleApprove}
        onReject={() => {
          handleClosePreview();
          if (!reviewNote.trim()) {
            alert('Please provide a reason for rejection');
            return;
          }
          handleReject();
        }}
      />
    </div>
  );
}
