'use client';

import { useEffect, useState, useCallback } from 'react';
import { useRouter, useParams } from 'next/navigation';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import {
  Document,
  DocumentVersion,
  getDocument,
  getDocumentVersions,
  approveDocument,
  rejectDocument,
  restoreDocumentVersion,
  updateDocument,
  downloadDocument,
  aiExtractDocument,
  aiClassifyDocument,
  aiSummarizeDocument,
  aiRenameDocument,
} from '@/lib/api';
import { getStatusBadgeClass, formatStatus } from '@/lib/status';
import { useToast } from '@/components';

export default function DocumentDetailClient() {
  const params = useParams();
  const documentId = params.id as string;
  const router = useRouter();
  const { user } = useAuth();
  const toast = useToast();

  const [document, setDocument] = useState<Document | null>(null);
  const [versions, setVersions] = useState<DocumentVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showVersions, setShowVersions] = useState(false);
  const [showRejectModal, setShowRejectModal] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const [rejecting, setRejecting] = useState(false);

  // AI Analysis state
  const [aiAnalyzing, setAiAnalyzing] = useState(false);
  const [aiResult, setAiResult] = useState<{
    classification?: { document_type: string; confidence: number; subcategory?: string };
    summary?: { summary: string; key_points?: string[] };
    suggestedName?: string;
  } | null>(null);

  const fetchDocument = useCallback(async () => {
    try {
      setLoading(true);
      const [docData, versionsData] = await Promise.all([
        getDocument(documentId),
        getDocumentVersions(documentId).catch(() => ({ versions: [] })),
      ]);
      setDocument(docData);
      setVersions(versionsData.versions || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load document');
    } finally {
      setLoading(false);
    }
  }, [documentId]);

  useEffect(() => {
    if (documentId) {
      fetchDocument();
    }
  }, [documentId, fetchDocument]);

  const handleApprove = async () => {
    if (!document) return;
    try {
      await approveDocument(document.id);
      setDocument({ ...document, status: 'approved' });
      toast.success('Document approved');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to approve document');
    }
  };

  const handleRejectClick = () => {
    setShowRejectModal(true);
    setRejectReason('');
  };

  const handleRejectConfirm = async () => {
    if (!document || !rejectReason.trim()) return;

    try {
      setRejecting(true);
      await rejectDocument(document.id, rejectReason.trim());
      setDocument({ ...document, status: 'rejected' });
      toast.success('Document rejected');
      setShowRejectModal(false);
      setRejectReason('');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to reject document');
    } finally {
      setRejecting(false);
    }
  };

  const handleRejectCancel = () => {
    setShowRejectModal(false);
    setRejectReason('');
  };

  const handleRestoreVersion = async (versionId: string) => {
    if (!document) return;
    if (!confirm('Are you sure you want to restore this version?')) return;

    try {
      await restoreDocumentVersion(document.id, versionId);
      toast.success('Version restored');
      fetchDocument();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to restore version');
    }
  };

  const handleAIAnalyze = async () => {
    if (!document || !document.file_path) {
      toast.error('No file available for analysis');
      return;
    }

    setAiAnalyzing(true);
    setAiResult(null);

    try {
      // Step 1: Extract text
      const extractResult = await aiExtractDocument({ file_key: document.file_path });

      // Handle async job response vs immediate result
      if ('job_id' in extractResult) {
        toast.error('Document extraction is processing. Please try again later.');
        setAiAnalyzing(false);
        return;
      }

      const text = (extractResult.extracted_data?.text as string) || '';

      if (!text) {
        toast.error('Could not extract text from document');
        setAiAnalyzing(false);
        return;
      }

      // Step 2: Run all AI operations in parallel
      const [classifyResult, summaryResult, renameResult] = await Promise.all([
        aiClassifyDocument({ file_key: document.file_path, text }),
        aiSummarizeDocument({ text, file_key: document.file_path }),
        aiRenameDocument({
          text,
          original_filename: document.original_name,
          client_name: document.client_name,
          file_key: document.file_path,
        }),
      ]);

      setAiResult({
        classification: {
          document_type: classifyResult.document_type,
          confidence: classifyResult.confidence,
          subcategory: classifyResult.subcategory,
        },
        summary: {
          summary: summaryResult.summary,
          key_points: summaryResult.key_points,
        },
        suggestedName: renameResult.suggested_name,
      });

      toast.success('AI analysis complete');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'AI analysis failed');
    } finally {
      setAiAnalyzing(false);
    }
  };

  const handleAcceptAISuggestions = async () => {
    if (!document || !aiResult) return;

    try {
      const updates: { name?: string; ai_summary?: string } = {};

      if (aiResult.suggestedName) {
        updates.name = aiResult.suggestedName;
      }
      if (aiResult.summary?.summary) {
        updates.ai_summary = aiResult.summary.summary;
      }

      if (Object.keys(updates).length > 0) {
        await updateDocument(document.id, updates);
        setDocument({ ...document, ...updates });
        toast.success('Document updated with AI suggestions');
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update document');
    }
  };

  const handleDownload = async () => {
    if (!document) return;
    try {
      const { download_url } = await downloadDocument(document.id);
      window.open(download_url, '_blank');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to download document');
    }
  };

  const formatFileSize = (bytes?: number) => {
    if (!bytes) return '-';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const canReview = user?.role === 'super_admin' || user?.role === 'tenant_admin';

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-slate-900 flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  if (error || !document) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
        <div className="max-w-4xl mx-auto px-4 py-8">
          <div className="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg p-6">
            <h2 className="text-lg font-semibold text-red-700 dark:text-red-300">Error</h2>
            <p className="text-red-600 dark:text-red-400 mt-2">{error || 'Document not found'}</p>
            <Link
              href="/dashboard/documents"
              className="mt-4 inline-block text-blue-600 dark:text-blue-400 hover:underline"
            >
              &larr; Back to documents
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <header className="bg-white dark:bg-slate-800 shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <Link
                href="/dashboard/documents"
                className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
              </Link>
              <div>
                <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{document.name}</h1>
                <p className="text-sm text-gray-500 dark:text-gray-400">{document.original_name}</p>
              </div>
            </div>
            <span className={`px-3 py-1 text-sm font-medium rounded-full ${getStatusBadgeClass(document.status)}`}>
              {formatStatus(document.status)}
            </span>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Document Info */}
          <div className="lg:col-span-2 space-y-6">
            {/* Details Card */}
            <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Document Details</h2>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Client</dt>
                  <dd className="mt-1 text-sm text-gray-900 dark:text-white">{document.client_name || '-'}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Type</dt>
                  <dd className="mt-1 text-sm text-gray-900 dark:text-white">{document.type_name || '-'}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">File Size</dt>
                  <dd className="mt-1 text-sm text-gray-900 dark:text-white">{formatFileSize(document.file_size)}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">MIME Type</dt>
                  <dd className="mt-1 text-sm text-gray-900 dark:text-white">{document.mime_type || '-'}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Version</dt>
                  <dd className="mt-1 text-sm text-gray-900 dark:text-white">{document.version}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Access</dt>
                  <dd className="mt-1 text-sm text-gray-900 dark:text-white capitalize">{document.access?.replace('_', ' ') || '-'}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Uploaded By</dt>
                  <dd className="mt-1 text-sm text-gray-900 dark:text-white">{document.uploaded_by_name || '-'}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Created</dt>
                  <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                    {new Date(document.created_at).toLocaleString()}
                  </dd>
                </div>
                {document.expiry_date && (
                  <div>
                    <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Expiry Date</dt>
                    <dd className="mt-1 text-sm text-gray-900 dark:text-white">
                      {new Date(document.expiry_date).toLocaleDateString()}
                    </dd>
                  </div>
                )}
                {document.review_note && (
                  <div className="sm:col-span-2">
                    <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Review Note</dt>
                    <dd className="mt-1 text-sm text-gray-900 dark:text-white">{document.review_note}</dd>
                  </div>
                )}
              </dl>
            </div>

            {/* AI Summary Section */}
            {(document.ai_summary || aiResult) && (
              <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
                <div className="flex items-center mb-4">
                  <span className="px-2 py-1 text-xs font-medium bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400 rounded mr-2">
                    AI
                  </span>
                  <h2 className="text-lg font-semibold text-gray-900 dark:text-white">AI Analysis</h2>
                </div>

                {/* Existing AI Summary */}
                {document.ai_summary && !aiResult && (
                  <div className="prose dark:prose-invert max-w-none">
                    <p className="text-sm text-gray-700 dark:text-gray-300">{document.ai_summary}</p>
                  </div>
                )}

                {/* New AI Result */}
                {aiResult && (
                  <div className="space-y-4">
                    {/* Classification */}
                    {aiResult.classification && (
                      <div>
                        <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Document Type</h3>
                        <div className="flex items-center">
                          <span className="text-gray-900 dark:text-white font-medium">
                            {aiResult.classification.document_type}
                          </span>
                          <span className="ml-2 text-xs text-gray-500">
                            ({Math.round(aiResult.classification.confidence * 100)}% confidence)
                          </span>
                        </div>
                        {aiResult.classification.subcategory && (
                          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            Subcategory: {aiResult.classification.subcategory}
                          </p>
                        )}
                      </div>
                    )}

                    {/* Suggested Name */}
                    {aiResult.suggestedName && (
                      <div>
                        <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Suggested Name</h3>
                        <p className="text-gray-900 dark:text-white">{aiResult.suggestedName}</p>
                      </div>
                    )}

                    {/* Summary */}
                    {aiResult.summary && (
                      <div>
                        <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Summary</h3>
                        <p className="text-sm text-gray-700 dark:text-gray-300">{aiResult.summary.summary}</p>
                        {aiResult.summary.key_points && aiResult.summary.key_points.length > 0 && (
                          <div className="mt-2">
                            <h4 className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Key Points</h4>
                            <ul className="list-disc list-inside text-sm text-gray-600 dark:text-gray-400">
                              {aiResult.summary.key_points.map((point, i) => (
                                <li key={i}>{point}</li>
                              ))}
                            </ul>
                          </div>
                        )}
                      </div>
                    )}

                    {/* Accept Button */}
                    <div className="pt-2 border-t border-gray-200 dark:border-gray-700">
                      <button
                        onClick={handleAcceptAISuggestions}
                        className="px-4 py-2 bg-purple-600 text-white text-sm font-medium rounded-md hover:bg-purple-700"
                      >
                        Apply AI Suggestions
                      </button>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Version History */}
            <div className="bg-white dark:bg-slate-800 rounded-lg shadow">
              <button
                onClick={() => setShowVersions(!showVersions)}
                className="w-full px-6 py-4 flex items-center justify-between text-left"
              >
                <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
                  Version History ({versions.length})
                </h2>
                <svg
                  className={`w-5 h-5 text-gray-500 transform transition-transform ${showVersions ? 'rotate-180' : ''}`}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                </svg>
              </button>
              {showVersions && versions.length > 0 && (
                <div className="border-t border-gray-200 dark:border-gray-700">
                  <ul className="divide-y divide-gray-200 dark:divide-gray-700">
                    {versions.map((version) => (
                      <li key={version.id} className="px-6 py-4 flex items-center justify-between">
                        <div>
                          <p className="text-sm font-medium text-gray-900 dark:text-white">
                            Version {version.version}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400">
                            {version.uploaded_by_name || 'Unknown'} &bull;{' '}
                            {new Date(version.created_at).toLocaleString()}
                          </p>
                        </div>
                        {version.version !== document.version && canReview && (
                          <button
                            onClick={() => handleRestoreVersion(version.id)}
                            className="text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400"
                          >
                            Restore
                          </button>
                        )}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {showVersions && versions.length === 0 && (
                <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700">
                  <p className="text-sm text-gray-500 dark:text-gray-400">No version history available.</p>
                </div>
              )}
            </div>
          </div>

          {/* Sidebar */}
          <div className="space-y-6">
            {/* Actions Card */}
            <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Actions</h2>
              <div className="space-y-3">
                {document.file_path && (
                  <button
                    onClick={handleDownload}
                    className="w-full flex items-center justify-center px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-slate-700 hover:bg-gray-50 dark:hover:bg-slate-600"
                  >
                    <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                    </svg>
                    Download
                  </button>
                )}

                {/* AI Analyze Button */}
                {document.file_path && (
                  <button
                    onClick={handleAIAnalyze}
                    disabled={aiAnalyzing}
                    className="w-full flex items-center justify-center px-4 py-2 border border-purple-300 dark:border-purple-700 rounded-md text-sm font-medium text-purple-700 dark:text-purple-400 bg-purple-50 dark:bg-purple-900/20 hover:bg-purple-100 dark:hover:bg-purple-900/30 disabled:opacity-50"
                  >
                    {aiAnalyzing ? (
                      <>
                        <svg className="animate-spin w-5 h-5 mr-2" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Analyzing...
                      </>
                    ) : (
                      <>
                        <span className="mr-2">AI</span>
                        Analyze Document
                      </>
                    )}
                  </button>
                )}

                {canReview && document.status === 'pending_review' && (
                  <>
                    <button
                      onClick={handleApprove}
                      className="w-full flex items-center justify-center px-4 py-2 border border-transparent rounded-md text-sm font-medium text-white bg-green-600 hover:bg-green-700"
                    >
                      <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                      </svg>
                      Approve
                    </button>
                    <button
                      onClick={handleRejectClick}
                      className="w-full flex items-center justify-center px-4 py-2 border border-transparent rounded-md text-sm font-medium text-white bg-red-600 hover:bg-red-700"
                    >
                      <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                      Reject
                    </button>
                  </>
                )}
              </div>
            </div>

            {/* Status Timeline */}
            <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Timeline</h2>
              <ul className="space-y-4">
                <li className="flex items-start">
                  <div className="flex-shrink-0">
                    <div className="w-2 h-2 mt-2 rounded-full bg-green-500"></div>
                  </div>
                  <div className="ml-3">
                    <p className="text-sm font-medium text-gray-900 dark:text-white">Created</p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {new Date(document.created_at).toLocaleString()}
                    </p>
                  </div>
                </li>
                {document.updated_at !== document.created_at && (
                  <li className="flex items-start">
                    <div className="flex-shrink-0">
                      <div className="w-2 h-2 mt-2 rounded-full bg-blue-500"></div>
                    </div>
                    <div className="ml-3">
                      <p className="text-sm font-medium text-gray-900 dark:text-white">Updated</p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        {new Date(document.updated_at).toLocaleString()}
                      </p>
                    </div>
                  </li>
                )}
                {document.reviewed_at && (
                  <li className="flex items-start">
                    <div className="flex-shrink-0">
                      <div className={`w-2 h-2 mt-2 rounded-full ${document.status === 'approved' ? 'bg-green-500' : 'bg-red-500'}`}></div>
                    </div>
                    <div className="ml-3">
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {document.status === 'approved' ? 'Approved' : 'Rejected'}
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        {new Date(document.reviewed_at).toLocaleString()}
                      </p>
                    </div>
                  </li>
                )}
              </ul>
            </div>
          </div>
        </div>
      </main>

      {/* Rejection Modal */}
      {showRejectModal && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          <div className="flex min-h-full items-center justify-center p-4">
            {/* Backdrop */}
            <div
              className="fixed inset-0 bg-black/50 transition-opacity"
              onClick={handleRejectCancel}
            />

            {/* Modal */}
            <div className="relative bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-md w-full p-6">
              <div className="flex justify-between items-center mb-4">
                <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
                  Reject Document
                </h2>
                <button
                  onClick={handleRejectCancel}
                  className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Rejection Reason *
                  </label>
                  <textarea
                    value={rejectReason}
                    onChange={(e) => setRejectReason(e.target.value)}
                    placeholder="Please provide a reason for rejecting this document..."
                    rows={4}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-2 focus:ring-red-500 focus:border-transparent"
                    autoFocus
                  />
                </div>

                <div className="flex space-x-3">
                  <button
                    onClick={handleRejectCancel}
                    className="flex-1 py-2 px-4 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-slate-700"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleRejectConfirm}
                    disabled={rejecting || !rejectReason.trim()}
                    className={`flex-1 py-2 px-4 rounded-md text-white font-medium transition-colors ${
                      rejecting || !rejectReason.trim()
                        ? 'bg-gray-400 cursor-not-allowed'
                        : 'bg-red-600 hover:bg-red-700'
                    }`}
                  >
                    {rejecting ? 'Rejecting...' : 'Reject Document'}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
