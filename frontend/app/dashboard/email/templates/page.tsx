'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import {
  EmailTemplate,
  getEmailTemplates,
  createEmailTemplate,
  updateEmailTemplate,
  deleteEmailTemplate,
} from '@/lib/api';
import { SkeletonTable, useToast } from '@/components';

export default function EmailTemplatesPage() {
  const { user } = useAuth();
  const toast = useToast();
  const [templates, setTemplates] = useState<EmailTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<EmailTemplate | null>(null);

  const canManage = user?.role === 'super_admin' || user?.role === 'tenant_admin';

  const fetchTemplates = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getEmailTemplates({
        type: typeFilter || undefined,
        search: search || undefined,
        active: true,
        limit: 50,
      });
      setTemplates(data.email_templates || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load templates');
    } finally {
      setLoading(false);
    }
  }, [search, typeFilter]);

  useEffect(() => {
    fetchTemplates();
  }, [fetchTemplates]);

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this template?')) return;

    try {
      await deleteEmailTemplate(id);
      setTemplates((prev) => prev.filter((t) => t.id !== id));
      toast.success('Template deleted');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete template');
    }
  };

  const getTypeBadgeClass = (type: string) => {
    switch (type) {
      case 'chase':
        return 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200';
      case 'notification':
        return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200';
      case 'welcome':
        return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
      default:
        return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200';
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <header className="bg-white dark:bg-slate-800 shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex justify-between items-center">
            <div className="flex items-center space-x-4">
              <Link
                href="/dashboard/email"
                className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
              </Link>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Email Templates</h1>
            </div>
            {canManage && (
              <button
                onClick={() => {
                  setEditingTemplate(null);
                  setShowModal(true);
                }}
                className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
              >
                <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                </svg>
                New Template
              </button>
            )}
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Filters */}
        <div className="mb-6 flex flex-col sm:flex-row gap-4">
          <div className="flex-1">
            <input
              type="text"
              placeholder="Search templates..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
          >
            <option value="">All types</option>
            <option value="chase">Chase</option>
            <option value="notification">Notification</option>
            <option value="welcome">Welcome</option>
            <option value="custom">Custom</option>
          </select>
        </div>

        {/* Error */}
        {error && (
          <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
            <p className="text-red-700 dark:text-red-300">{error}</p>
          </div>
        )}

        {/* Templates Grid */}
        <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
          {loading ? (
            <div className="p-6">
              <SkeletonTable rows={5} columns={4} />
            </div>
          ) : templates.length === 0 ? (
            <div className="p-12 text-center">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white">No templates</h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Get started by creating your first email template.
              </p>
              {canManage && (
                <button
                  onClick={() => {
                    setEditingTemplate(null);
                    setShowModal(true);
                  }}
                  className="mt-4 inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
                >
                  Create Template
                </button>
              )}
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 p-4">
              {templates.map((template) => (
                <div
                  key={template.id}
                  className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:shadow-md transition-shadow"
                >
                  <div className="flex items-start justify-between mb-2">
                    <h3 className="text-lg font-medium text-gray-900 dark:text-white truncate flex-1">
                      {template.name}
                    </h3>
                    <span className={`ml-2 px-2 py-0.5 text-xs font-medium rounded-full ${getTypeBadgeClass(template.type)}`}>
                      {template.type}
                    </span>
                  </div>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mb-2 truncate">
                    Subject: {template.subject}
                  </p>
                  {template.is_default && (
                    <span className="inline-flex items-center px-2 py-0.5 text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200 rounded-full mb-2">
                      Default
                    </span>
                  )}
                  {template.placeholders && template.placeholders.length > 0 && (
                    <div className="mb-3">
                      <p className="text-xs text-gray-400 dark:text-gray-500 mb-1">Placeholders:</p>
                      <div className="flex flex-wrap gap-1">
                        {template.placeholders.slice(0, 3).map((p, i) => (
                          <span
                            key={i}
                            className="text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 px-1.5 py-0.5 rounded"
                          >
                            {`{{${p}}}`}
                          </span>
                        ))}
                        {template.placeholders.length > 3 && (
                          <span className="text-xs text-gray-400 dark:text-gray-500">
                            +{template.placeholders.length - 3} more
                          </span>
                        )}
                      </div>
                    </div>
                  )}
                  {canManage && (
                    <div className="flex gap-2 pt-2 border-t border-gray-200 dark:border-gray-700">
                      <button
                        onClick={() => {
                          setEditingTemplate(template);
                          setShowModal(true);
                        }}
                        className="flex-1 px-3 py-1.5 text-sm text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded transition-colors"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(template.id)}
                        className="flex-1 px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
                      >
                        Delete
                      </button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </main>

      {/* Create/Edit Modal */}
      {showModal && (
        <TemplateModal
          template={editingTemplate}
          onClose={() => {
            setShowModal(false);
            setEditingTemplate(null);
          }}
          onSaved={() => {
            setShowModal(false);
            setEditingTemplate(null);
            fetchTemplates();
            toast.success(editingTemplate ? 'Template updated' : 'Template created');
          }}
        />
      )}
    </div>
  );
}

// Template Modal Component
function TemplateModal({
  template,
  onClose,
  onSaved,
}: {
  template: EmailTemplate | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(template?.name || '');
  const [subject, setSubject] = useState(template?.subject || '');
  const [bodyHtml, setBodyHtml] = useState(template?.body_html || '');
  const [type, setType] = useState<'chase' | 'notification' | 'welcome' | 'custom'>(
    (template?.type as 'chase' | 'notification' | 'welcome' | 'custom') || 'custom'
  );
  const [placeholders, setPlaceholders] = useState(template?.placeholders?.join(', ') || '');
  const [isDefault, setIsDefault] = useState(template?.is_default || false);
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!name || !subject || !bodyHtml) {
      alert('Please fill in all required fields');
      return;
    }

    setSaving(true);
    try {
      const data = {
        name,
        subject,
        body_html: bodyHtml,
        type,
        placeholders: placeholders.split(',').map((p) => p.trim()).filter(Boolean),
        is_default: isDefault,
      };

      if (template) {
        await updateEmailTemplate(template.id, data);
      } else {
        await createEmailTemplate(data);
      }
      onSaved();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to save template');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {template ? 'Edit Template' : 'New Template'}
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Form */}
        <div className="p-4 flex-1 overflow-auto">
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Name *
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                  placeholder="Template name"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Type *
                </label>
                <select
                  value={type}
                  onChange={(e) => setType(e.target.value as typeof type)}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                >
                  <option value="chase">Chase</option>
                  <option value="notification">Notification</option>
                  <option value="welcome">Welcome</option>
                  <option value="custom">Custom</option>
                </select>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Subject *
              </label>
              <input
                type="text"
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                placeholder="Email subject line"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Body (HTML) *
              </label>
              <textarea
                value={bodyHtml}
                onChange={(e) => setBodyHtml(e.target.value)}
                rows={10}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                placeholder="<p>Hello {{client_name}},</p><p>Your email content here...</p>"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Placeholders (comma-separated)
              </label>
              <input
                type="text"
                value={placeholders}
                onChange={(e) => setPlaceholders(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                placeholder="client_name, company_name, deadline"
              />
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Use {`{{placeholder_name}}`} in subject or body to insert dynamic values
              </p>
            </div>

            <div className="flex items-center">
              <input
                type="checkbox"
                id="isDefault"
                checked={isDefault}
                onChange={(e) => setIsDefault(e.target.checked)}
                className="h-4 w-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
              />
              <label htmlFor="isDefault" className="ml-2 text-sm text-gray-700 dark:text-gray-300">
                Set as default template for this type
              </label>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 text-sm font-medium rounded-md hover:bg-gray-50 dark:hover:bg-slate-700 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? 'Saving...' : 'Save Template'}
          </button>
        </div>
      </div>
    </div>
  );
}
