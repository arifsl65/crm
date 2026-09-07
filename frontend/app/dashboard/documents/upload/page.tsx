'use client';

import { useState, useCallback, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/components/auth-guard';
import {
  getClients,
  getDocumentTypes,
  getDocumentUploadUrl,
  confirmDocumentUpload,
  updateDocument,
  Client,
  DocumentType,
  aiExtractDocument,
  aiClassifyDocument,
  aiRenameDocument,
} from '@/lib/api';
import { useToast } from '@/components';

interface FileWithAI {
  file: File;
  documentId?: string;
  filePath?: string;
  aiProcessing: boolean;
  aiClassification?: {
    document_type: string;
    confidence: number;
    subcategory?: string;
  };
  aiSuggestedName?: string;
  aiAlternativeNames?: string[];
  acceptedName?: string;
  acceptedType?: string;
}

export default function DocumentUploadPage() {
  const { user } = useAuth();
  const router = useRouter();
  const toast = useToast();

  const [clients, setClients] = useState<Client[]>([]);
  const [documentTypes, setDocumentTypes] = useState<DocumentType[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [uploadComplete, setUploadComplete] = useState(false);
  const [dragActive, setDragActive] = useState(false);

  // Form state
  const [selectedClient, setSelectedClient] = useState('');
  const [selectedType, setSelectedType] = useState('');
  const [documentName, setDocumentName] = useState('');
  const [files, setFiles] = useState<FileWithAI[]>([]);
  const [enableAI, setEnableAI] = useState(true);

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
        toast.error('Failed to load data');
      } finally {
        setLoading(false);
      }
    }
    loadData();
  }, [toast]);

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
        aiProcessing: false,
      }));
      setFiles(prev => [...prev, ...newFiles]);
      // Auto-fill document name from first file
      if (!documentName && newFiles[0]) {
        setDocumentName(newFiles[0].file.name.replace(/\.[^/.]+$/, ''));
      }
    }
  }, [documentName]);

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      const newFiles = Array.from(e.target.files).map(file => ({
        file,
        aiProcessing: false,
      }));
      setFiles(prev => [...prev, ...newFiles]);
      // Auto-fill document name from first file
      if (!documentName && newFiles[0]) {
        setDocumentName(newFiles[0].file.name.replace(/\.[^/.]+$/, ''));
      }
    }
  };

  const removeFile = (index: number) => {
    setFiles(prev => prev.filter((_, i) => i !== index));
  };

  // Process AI for a specific file after upload
  const processAI = async (index: number, filePath: string, documentId: string) => {
    const fileInfo = files[index];
    if (!fileInfo || !enableAI) return;

    // Update to show AI is processing
    setFiles(prev => prev.map((f, i) =>
      i === index ? { ...f, aiProcessing: true } : f
    ));

    try {
      // Step 1: Extract text from document
      const extractResult = await aiExtractDocument({ file_key: filePath });

      // Handle async job response vs immediate result
      if ('job_id' in extractResult) {
        setFiles(prev => prev.map((f, i) =>
          i === index ? { ...f, aiProcessing: false } : f
        ));
        return;
      }

      const text = (extractResult.extracted_data?.text as string) || '';

      if (!text) {
        setFiles(prev => prev.map((f, i) =>
          i === index ? { ...f, aiProcessing: false } : f
        ));
        return;
      }

      // Step 2: Classify document
      const classifyResult = await aiClassifyDocument({
        file_key: filePath,
        text,
      });

      // Step 3: Get suggested name
      const selectedClientObj = clients.find(c => c.id === selectedClient);
      const renameResult = await aiRenameDocument({
        text,
        original_filename: fileInfo.file.name,
        document_type: classifyResult.document_type,
        client_name: selectedClientObj?.company_name,
        file_key: filePath,
      });

      // Update file with AI results
      setFiles(prev => prev.map((f, i) =>
        i === index ? {
          ...f,
          aiProcessing: false,
          aiClassification: {
            document_type: classifyResult.document_type,
            confidence: classifyResult.confidence,
            subcategory: classifyResult.subcategory,
          },
          aiSuggestedName: renameResult.suggested_name,
          aiAlternativeNames: renameResult.alternatives,
        } : f
      ));
    } catch (err) {
      console.error('AI processing failed:', err);
      setFiles(prev => prev.map((f, i) =>
        i === index ? { ...f, aiProcessing: false } : f
      ));
    }
  };

  const acceptAISuggestion = (index: number) => {
    const fileInfo = files[index];
    if (!fileInfo) return;

    setFiles(prev => prev.map((f, i) =>
      i === index ? {
        ...f,
        acceptedName: f.aiSuggestedName,
        acceptedType: f.aiClassification?.document_type,
      } : f
    ));

    // Update document with AI suggestions
    if (fileInfo.documentId && fileInfo.aiSuggestedName) {
      updateDocument(fileInfo.documentId, {
        name: fileInfo.aiSuggestedName,
      }).catch(err => console.error('Failed to update document:', err));
    }
  };

  const useAlternativeName = (index: number, name: string) => {
    const fileInfo = files[index];
    if (!fileInfo) return;

    setFiles(prev => prev.map((f, i) =>
      i === index ? { ...f, acceptedName: name } : f
    ));

    if (fileInfo.documentId) {
      updateDocument(fileInfo.documentId, { name }).catch(err =>
        console.error('Failed to update document:', err)
      );
    }
  };

  const handleUpload = async () => {
    if (files.length === 0) {
      toast.error('Please select at least one file');
      return;
    }
    if (!documentName.trim()) {
      toast.error('Please enter a document name');
      return;
    }

    setUploading(true);
    try {
      const uploadedFiles: { index: number; filePath: string; documentId: string }[] = [];

      for (let i = 0; i < files.length; i++) {
        const fileInfo = files[i];
        // Get signed upload URL
        const { upload_url, document_id } = await getDocumentUploadUrl({
          name: documentName || fileInfo.file.name.replace(/\.[^/.]+$/, ''),
          client_id: selectedClient || undefined,
        });

        // Extract file path from upload URL (OSS path pattern)
        const urlObj = new URL(upload_url);
        const filePath = urlObj.pathname.slice(1); // Remove leading slash

        // Upload file directly to storage
        const uploadRes = await fetch(upload_url, {
          method: 'PUT',
          body: fileInfo.file,
          headers: {
            'Content-Type': fileInfo.file.type || 'application/octet-stream',
          },
        });

        if (!uploadRes.ok) {
          throw new Error(`Failed to upload ${fileInfo.file.name}`);
        }

        // Confirm upload with metadata
        await confirmDocumentUpload(document_id, {
          name: documentName || fileInfo.file.name.replace(/\.[^/.]+$/, ''),
          original_name: fileInfo.file.name,
          file_size: fileInfo.file.size,
          mime_type: fileInfo.file.type || 'application/octet-stream',
          type_id: selectedType || undefined,
        });

        // Store for AI processing
        uploadedFiles.push({ index: i, filePath, documentId: document_id });

        // Update file state with document info
        setFiles(prev => prev.map((f, idx) =>
          idx === i ? { ...f, documentId: document_id, filePath } : f
        ));
      }

      toast.success(`Successfully uploaded ${files.length} document(s)`);

      if (enableAI && uploadedFiles.length > 0) {
        setUploadComplete(true);
        // Process AI for each file in background
        for (const { index, filePath, documentId } of uploadedFiles) {
          processAI(index, filePath, documentId);
        }
      } else {
        router.push('/dashboard/documents');
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  if (loading) {
    return (
      <div className="p-6 flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Upload Document</h1>
        <p className="text-gray-500 dark:text-gray-400 mt-1">
          Upload documents for clients or firm use
        </p>
      </div>

      <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6 space-y-6">
        {/* Drag & Drop Zone */}
        <div
          onDragEnter={handleDrag}
          onDragLeave={handleDrag}
          onDragOver={handleDrag}
          onDrop={handleDrop}
          className={`
            border-2 border-dashed rounded-lg p-8 text-center transition-colors cursor-pointer
            ${dragActive
              ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
              : 'border-gray-300 dark:border-gray-600 hover:border-blue-400 dark:hover:border-blue-500'
            }
          `}
          onClick={() => document.getElementById('file-input')?.click()}
        >
          <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
          </svg>
          <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
            <span className="font-medium text-blue-600 dark:text-blue-400">Click to upload</span> or drag and drop
          </p>
          <p className="text-xs text-gray-500 dark:text-gray-500 mt-1">
            PDF, DOC, DOCX, XLS, XLSX, PNG, JPG up to 50MB
          </p>
          <input
            id="file-input"
            type="file"
            multiple
            onChange={handleFileSelect}
            className="hidden"
            accept=".pdf,.doc,.docx,.xls,.xlsx,.png,.jpg,.jpeg"
          />
        </div>

        {/* Selected Files */}
        {files.length > 0 && (
          <div className="space-y-2">
            <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300">
              {uploadComplete ? 'Uploaded Files' : 'Selected Files'}
            </h3>
            <ul className="divide-y divide-gray-200 dark:divide-gray-700 border border-gray-200 dark:border-gray-700 rounded-lg">
              {files.map((fileInfo, index) => (
                <li key={index} className="p-4">
                  <div className="flex items-start justify-between">
                    <div className="flex items-start space-x-3 flex-1">
                      <svg className="h-8 w-8 text-gray-400 flex-shrink-0 mt-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                      </svg>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-gray-900 dark:text-white">{fileInfo.file.name}</p>
                        <p className="text-xs text-gray-500">{formatFileSize(fileInfo.file.size)}</p>

                        {/* AI Processing Indicator */}
                        {fileInfo.aiProcessing && (
                          <div className="mt-2 flex items-center text-purple-600 dark:text-purple-400">
                            <svg className="animate-spin h-4 w-4 mr-2" fill="none" viewBox="0 0 24 24">
                              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                            </svg>
                            <span className="text-sm">AI analyzing document...</span>
                          </div>
                        )}

                        {/* AI Classification Result */}
                        {fileInfo.aiClassification && !fileInfo.aiProcessing && (
                          <div className="mt-2 p-3 bg-purple-50 dark:bg-purple-900/20 rounded-lg border border-purple-200 dark:border-purple-800">
                            <div className="flex items-center mb-2">
                              <span className="text-purple-600 dark:text-purple-400 mr-2">AI</span>
                              <span className="text-sm font-medium text-gray-900 dark:text-white">
                                Classified as: {fileInfo.aiClassification.document_type}
                              </span>
                              <span className="ml-2 text-xs text-gray-500">
                                ({Math.round(fileInfo.aiClassification.confidence * 100)}% confidence)
                              </span>
                            </div>

                            {fileInfo.aiSuggestedName && (
                              <div className="mt-2">
                                <p className="text-xs text-gray-600 dark:text-gray-400 mb-1">Suggested name:</p>
                                <div className="flex items-center gap-2 flex-wrap">
                                  <span className="px-2 py-1 bg-white dark:bg-slate-700 rounded text-sm font-medium text-gray-900 dark:text-white">
                                    {fileInfo.aiSuggestedName}
                                  </span>
                                  {!fileInfo.acceptedName && (
                                    <button
                                      onClick={() => acceptAISuggestion(index)}
                                      className="px-2 py-1 text-xs bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400 rounded hover:bg-green-200 dark:hover:bg-green-900/50"
                                    >
                                      Accept
                                    </button>
                                  )}
                                  {fileInfo.acceptedName && (
                                    <span className="text-xs text-green-600 dark:text-green-400 flex items-center">
                                      <svg className="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                      </svg>
                                      Accepted
                                    </span>
                                  )}
                                </div>

                                {/* Alternative names */}
                                {fileInfo.aiAlternativeNames && fileInfo.aiAlternativeNames.length > 0 && !fileInfo.acceptedName && (
                                  <div className="mt-2">
                                    <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">Alternatives:</p>
                                    <div className="flex flex-wrap gap-1">
                                      {fileInfo.aiAlternativeNames.map((altName, altIdx) => (
                                        <button
                                          key={altIdx}
                                          onClick={() => useAlternativeName(index, altName)}
                                          className="px-2 py-0.5 text-xs bg-gray-100 dark:bg-slate-600 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-200 dark:hover:bg-slate-500"
                                        >
                                          {altName}
                                        </button>
                                      ))}
                                    </div>
                                  </div>
                                )}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                    {!uploadComplete && (
                      <button
                        onClick={() => removeFile(index)}
                        className="text-red-500 hover:text-red-700 ml-3"
                      >
                        <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Form Fields */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Document Name
            </label>
            <input
              type="text"
              value={documentName}
              onChange={(e) => setDocumentName(e.target.value)}
              disabled={uploadComplete}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50"
              placeholder="Enter document name"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Document Type
            </label>
            <select
              value={selectedType}
              onChange={(e) => setSelectedType(e.target.value)}
              disabled={uploadComplete}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50"
            >
              <option value="">Select type (optional)</option>
              {documentTypes.map((type) => (
                <option key={type.id} value={type.id}>{type.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Client (Optional)
            </label>
            <select
              value={selectedClient}
              onChange={(e) => setSelectedClient(e.target.value)}
              disabled={uploadComplete}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50"
            >
              <option value="">No client (Firm Document)</option>
              {clients.map((client) => (
                <option key={client.id} value={client.id}>{client.company_name}</option>
              ))}
            </select>
          </div>

          {/* AI Toggle */}
          <div className="flex items-center">
            <label className="flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={enableAI}
                onChange={(e) => setEnableAI(e.target.checked)}
                disabled={uploadComplete}
                className="w-4 h-4 text-purple-600 border-gray-300 rounded focus:ring-purple-500"
              />
              <span className="ml-2 text-sm text-gray-700 dark:text-gray-300">
                Enable AI classification
              </span>
            </label>
            <span className="ml-2 px-2 py-0.5 text-xs bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400 rounded">
              AI
            </span>
          </div>
        </div>

        {/* Actions */}
        <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200 dark:border-gray-700">
          {uploadComplete ? (
            <>
              <button
                onClick={() => {
                  setFiles([]);
                  setUploadComplete(false);
                  setDocumentName('');
                }}
                className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-md"
              >
                Upload More
              </button>
              <button
                onClick={() => router.push('/dashboard/documents')}
                disabled={files.some(f => f.aiProcessing)}
                className="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
              >
                {files.some(f => f.aiProcessing) ? (
                  <>
                    <svg className="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    AI Processing...
                  </>
                ) : (
                  <>
                    <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                    Done
                  </>
                )}
              </button>
            </>
          ) : (
            <>
              <button
                onClick={() => router.push('/dashboard/documents')}
                className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-md"
              >
                Cancel
              </button>
              <button
                onClick={handleUpload}
                disabled={uploading || files.length === 0}
                className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center"
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
                  <>
                    <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                    </svg>
                    Upload {enableAI && '& Analyze'}
                  </>
                )}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
