'use client';

import { useState } from 'react';

interface RejectDocumentModalProps {
  isOpen: boolean;
  documentName: string;
  onClose: () => void;
  onSubmit: (note: string) => void;
}

export function RejectDocumentModal({
  isOpen,
  documentName,
  onClose,
  onSubmit,
}: RejectDocumentModalProps) {
  const [note, setNote] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    if (!note.trim()) {
      setError('Please enter a rejection reason');
      return;
    }

    try {
      setSubmitting(true);
      setError(null);
      await onSubmit(note.trim());
      setNote('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reject');
    } finally {
      setSubmitting(false);
    }
  };

  const handleClose = () => {
    setNote('');
    setError(null);
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[70] overflow-y-auto">
      <div className="flex min-h-full items-center justify-center p-4">
        {/* Backdrop */}
        <div
          className="fixed inset-0 bg-black/60 transition-opacity"
          onClick={handleClose}
        />

        {/* Modal */}
        <div className="relative bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-md w-full p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="flex-shrink-0 w-10 h-10 bg-red-100 dark:bg-red-900/30 rounded-full flex items-center justify-center">
              <svg className="w-5 h-5 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </div>
            <div>
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
                Reject Document
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400 truncate max-w-[250px]">
                {documentName}
              </p>
            </div>
          </div>

          <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">
            The client will be notified and asked to re-upload a corrected document.
          </p>

          {error && (
            <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg">
              <p className="text-sm text-red-700 dark:text-red-300">{error}</p>
            </div>
          )}

          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              Rejection Reason <span className="text-red-500">*</span>
            </label>
            <textarea
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="e.g., Document is blurry, please upload a clearer version..."
              rows={4}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-2 focus:ring-red-500 focus:border-red-500"
              autoFocus
            />
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              This message will be sent to the client
            </p>
          </div>

          <div className="flex justify-end gap-3">
            <button
              onClick={handleClose}
              disabled={submitting}
              className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-slate-700 transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleSubmit}
              disabled={submitting || !note.trim()}
              className="px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 disabled:bg-red-400 disabled:cursor-not-allowed rounded-lg transition-colors flex items-center gap-2"
            >
              {submitting ? (
                <>
                  <span className="animate-spin">⏳</span>
                  Rejecting...
                </>
              ) : (
                'Reject Document'
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
