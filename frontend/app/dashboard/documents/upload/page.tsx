'use client';

import { useState, useCallback, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import {
  getClients,
  getDocumentTypes,
  getDocumentUploadUrl,
  confirmDocumentUpload,
  Client,
  DocumentType,
} from '@/lib/api';
import { useToast } from '@/components';

interface UploadedFile {
  file: File;
  preview?: string;
  progress: number;
  status: 'pending' | 'uploading' | 'complete' | 'error';
  documentId?: string;
  error?: string;
}

export default function DocumentUploadPage() {
  const router = useRouter();
  const toast = useToast();

  const [clients, setClients] = useState<Client[]>([]);
  const [documentTypes, setDocumentTypes] = useState<DocumentType[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [dragActive, setDragActive] = useState(false);

  // Form state
  const [selectedClient, setSelectedClient] = useState('');
  const [selectedType, setSelectedType] = useState('');
  const [files, setFiles] = useState<UploadedFile[]>([]);

  useEffect(() => {
    async function loadData() {
      try {
        const [clientsRes, typesRes] = await Promise.all([
          getClients({ limit: 100 }),
          getDocumentTypes({ limit: 100, active: true }),
        ]);
        setClients(clientsRes.clients || []);
        setDocumentTypes(typesRes.document_types || []);
      } catch (err) {
        console.error('Failed to load data:', err);
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, []);

  const handleDrag = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);

    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const newFiles = Array.from(e.dataTransfer.files).map(file => ({
        file,
        progress: 0,
        status: 'pending' as const,
      }));
      setFiles(prev => [...prev, ...newFiles]);
    }
  }, []);

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      const newFiles = Array.from(e.target.files).map(file => ({
        file,
        progress: 0,
        status: 'pending' as const,
      }));
      setFiles(prev => [...prev, ...newFiles]);
    }
  };

  const removeFile = (index: number) => {
    setFiles(prev => prev.filter((_, i) => i !== index));
  };

  const handleUpload = async () => {
    if (files.length === 0) {
      toast.error('Please select at least one file');
      return;
    }

    setUploading(true);
    let successCount = 0;

    for (let i = 0; i < files.length; i++) {
      const fileInfo = files[i];

      // Update status to uploading
      setFiles(prev => prev.map((f, idx) =>
        idx === i ? { ...f, status: 'uploading' as const, progress: 10 } : f
      ));

      try {
        // Get upload URL (creates pending document record)
        const { upload_url, document_id } = await getDocumentUploadUrl({
          name: fileInfo.file.name.replace(/\.[^/.]+$/, ''),
          client_id: selectedClient || undefined,
          type_id: selectedType || undefined,
        });

        setFiles(prev => prev.map((f, idx) =>
          idx === i ? { ...f, progress: 30 } : f
        ));

        // Upload file via multipart POST to backend endpoint
        const formData = new FormData();
        formData.append('file', fileInfo.file);

        const uploadRes = await fetch(upload_url, {
          method: 'POST',
          credentials: 'include',
          body: formData,
        });

        if (!uploadRes.ok) {
          const errorData = await uploadRes.json().catch(() => ({}));
          throw new Error(errorData.message || `Upload failed: ${uploadRes.statusText}`);
        }

        setFiles(prev => prev.map((f, idx) =>
          idx === i ? { ...f, progress: 70 } : f
        ));

        // Update to complete
        setFiles(prev => prev.map((f, idx) =>
          idx === i ? { ...f, status: 'complete' as const, progress: 100, documentId: document_id } : f
        ));
        successCount++;

      } catch (err) {
        setFiles(prev => prev.map((f, idx) =>
          idx === i ? {
            ...f,
            status: 'error' as const,
            error: err instanceof Error ? err.message : 'Upload failed'
          } : f
        ));
      }
    }

    setUploading(false);

    if (successCount > 0) {
      toast.success(`Successfully uploaded ${successCount} document${successCount > 1 ? 's' : ''}`);
      // Navigate after short delay
      setTimeout(() => {
        router.push('/dashboard/documents');
      }, 1500);
    } else {
      toast.error('All uploads failed');
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const handleClose = () => {
    router.push('/dashboard/documents');
  };

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center p-4 bg-gray-100 dark:bg-slate-900">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="h-full flex items-center justify-center p-4 bg-gray-100 dark:bg-slate-900">
      {/* Overlay Panel */}
      <div className="w-full max-w-lg bg-white dark:bg-slate-800 rounded-lg shadow-xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <h1 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center">
            <span className="mr-2">⬆️</span>
            UPLOAD DOCUMENT
          </h1>
          <button
            onClick={handleClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 text-2xl font-light"
            data-testid="close-upload"
          >
            ✕
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
          {/* Drag & Drop Zone */}
          <div
            onDragEnter={handleDrag}
            onDragLeave={handleDrag}
            onDragOver={handleDrag}
            onDrop={handleDrop}
            onClick={() => document.getElementById('file-input')?.click()}
            className={`
              border-2 border-dashed rounded-lg p-6 text-center transition-colors cursor-pointer
              ${dragActive
                ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                : 'border-gray-300 dark:border-gray-600 hover:border-blue-400 dark:hover:border-blue-500'
              }
            `}
            data-testid="drop-zone"
          >
            <div className="text-3xl mb-2">📁</div>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              <span className="font-medium text-blue-600 dark:text-blue-400">Drop file here</span>
              <br />or click to browse
            </p>
            <p className="text-xs text-gray-500 dark:text-gray-500 mt-2">
              PDF, DOC, XLS, PNG, JPG up to 50MB
            </p>
            <input
              id="file-input"
              type="file"
              multiple
              onChange={handleFileSelect}
              className="hidden"
              accept=".pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg"
              data-testid="file-input"
            />
          </div>

          {/* Selected Files */}
          {files.length > 0 && (
            <div className="space-y-2">
              {files.map((fileInfo, index) => (
                <div
                  key={index}
                  className={`
                    flex items-center justify-between p-3 rounded-lg border
                    ${fileInfo.status === 'complete'
                      ? 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800'
                      : fileInfo.status === 'error'
                      ? 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'
                      : 'bg-gray-50 dark:bg-slate-700/50 border-gray-200 dark:border-gray-600'
                    }
                  `}
                  data-testid={`file-item-${index}`}
                >
                  <div className="flex items-center space-x-3 flex-1 min-w-0">
                    <span className="text-xl">
                      {fileInfo.status === 'complete' ? '✅' : fileInfo.status === 'error' ? '❌' : '📄'}
                    </span>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                        {fileInfo.file.name}
                      </p>
                      <p className="text-xs text-gray-500">
                        {formatFileSize(fileInfo.file.size)}
                        {fileInfo.status === 'uploading' && ` · ${fileInfo.progress}%`}
                        {fileInfo.status === 'error' && ` · ${fileInfo.error}`}
                      </p>
                      {fileInfo.status === 'uploading' && (
                        <div className="mt-1 w-full bg-gray-200 dark:bg-gray-600 rounded-full h-1">
                          <div
                            className="bg-blue-600 h-1 rounded-full transition-all duration-300"
                            style={{ width: `${fileInfo.progress}%` }}
                          />
                        </div>
                      )}
                    </div>
                  </div>
                  {fileInfo.status === 'pending' && (
                    <button
                      onClick={(e) => { e.stopPropagation(); removeFile(index); }}
                      className="text-gray-400 hover:text-red-500 ml-2"
                      data-testid={`remove-file-${index}`}
                    >
                      ✕
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Client Selection */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Client
            </label>
            <select
              value={selectedClient}
              onChange={(e) => setSelectedClient(e.target.value)}
              disabled={uploading}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50"
              data-testid="client-select"
            >
              <option value="">No client (Firm Document)</option>
              {clients.map((client) => (
                <option key={client.id} value={client.id}>{client.company_name}</option>
              ))}
            </select>
          </div>

          {/* Category Selection */}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Category
            </label>
            <select
              value={selectedType}
              onChange={(e) => setSelectedType(e.target.value)}
              disabled={uploading}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50"
              data-testid="type-select"
            >
              <option value="">Select category (optional)</option>
              {documentTypes.map((type) => (
                <option key={type.id} value={type.id}>{type.name}</option>
              ))}
            </select>
          </div>
        </div>

        {/* Footer Actions */}
        <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end space-x-3">
          <button
            onClick={handleClose}
            disabled={uploading}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-md disabled:opacity-50"
            data-testid="cancel-btn"
          >
            Cancel
          </button>
          <button
            onClick={handleUpload}
            disabled={uploading || files.length === 0 || files.every(f => f.status === 'complete')}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
            data-testid="upload-btn"
          >
            {uploading ? (
              <>
                <svg className="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Uploading...
              </>
            ) : (
              'Upload'
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
