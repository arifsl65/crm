'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import { useToast } from '@/components';
import {
  ServiceType,
  DocumentType,
  getServiceTypes,
  createServiceType,
  updateServiceType,
  deleteServiceType,
  getServiceTypeCategories,
  getDocumentTypes,
} from '@/lib/api';

// Default categories
const DEFAULT_CATEGORIES = ['Tax', 'Accounts', 'Payroll', 'Company', 'VAT', 'Other'];
const PRIORITY_OPTIONS = ['low', 'medium', 'high', 'urgent'];
const RECURRENCE_OPTIONS = [
  { value: '', label: 'One-time' },
  { value: 'monthly', label: 'Monthly' },
  { value: 'quarterly', label: 'Quarterly' },
  { value: 'annually', label: 'Annually' },
];

// Format deadline days for display
function formatDeadline(days?: number, recurrence?: string): string {
  if (!days) return 'No deadline';

  const months = Math.floor(days / 30);
  const remainingDays = days % 30;

  let result = '';
  if (months > 0) result += `${months}M`;
  if (remainingDays > 0) result += `${months > 0 ? '+' : ''}${remainingDays}D`;

  if (recurrence) {
    const period = recurrence === 'quarterly' ? 'quarter' :
                   recurrence === 'monthly' ? 'month' :
                   recurrence === 'annually' ? 'year' : 'period';
    result += ` after ${period}`;
  }

  return result || 'No deadline';
}

export default function ServiceTypesPage() {
  const { user } = useAuth();
  const toast = useToast();

  // List state
  const [serviceTypes, setServiceTypes] = useState<ServiceType[]>([]);
  const [documentTypes, setDocumentTypes] = useState<DocumentType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filter state
  const [search, setSearch] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [categories, setCategories] = useState<string[]>(DEFAULT_CATEGORIES);

  // Modal state
  const [showModal, setShowModal] = useState(false);
  const [editingItem, setEditingItem] = useState<ServiceType | null>(null);
  const [saving, setSaving] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);

  // Requirements modal state
  const [showRequirementsModal, setShowRequirementsModal] = useState(false);
  const [requirementsItem, setRequirementsItem] = useState<ServiceType | null>(null);
  const [selectedRequirements, setSelectedRequirements] = useState<string[]>([]);
  const [savingRequirements, setSavingRequirements] = useState(false);

  // Delete confirmation state
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Form state
  const [formData, setFormData] = useState({
    name: '',
    category: '',
    description: '',
    default_priority: 'medium',
    default_deadline_days: 30,
    is_recurring: false,
    recurrence_pattern: '',
    hmrc_relevant: false,
  });

  const isAdmin = user?.role === 'super_admin' || user?.role === 'tenant_admin';

  // Fetch service types
  const fetchServiceTypes = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getServiceTypes({
        search: search || undefined,
        category: categoryFilter || undefined,
        limit: 100,
      });
      setServiceTypes(data.service_types || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load service types');
    } finally {
      setLoading(false);
    }
  }, [search, categoryFilter]);

  // Fetch document types for requirements
  const fetchDocumentTypes = useCallback(async () => {
    try {
      const data = await getDocumentTypes({ limit: 100, active: true });
      setDocumentTypes(data.document_types || []);
    } catch {
      // Silent fail - requirements will just not show doc names
    }
  }, []);

  // Fetch categories
  const fetchCategories = useCallback(async () => {
    try {
      const data = await getServiceTypeCategories();
      if (data.categories && data.categories.length > 0) {
        setCategories(data.categories);
      }
    } catch {
      // Use default categories on error
    }
  }, []);

  useEffect(() => {
    fetchServiceTypes();
  }, [fetchServiceTypes]);

  useEffect(() => {
    fetchDocumentTypes();
    fetchCategories();
  }, [fetchDocumentTypes, fetchCategories]);

  // Get document type name by ID
  const getDocTypeName = (id: string): string => {
    const docType = documentTypes.find((dt) => dt.id === id);
    return docType?.name || 'Unknown';
  };

  // Open add modal
  const handleAdd = () => {
    setEditingItem(null);
    setFormData({
      name: '',
      category: categories[0] || 'Tax',
      description: '',
      default_priority: 'medium',
      default_deadline_days: 30,
      is_recurring: false,
      recurrence_pattern: '',
      hmrc_relevant: false,
    });
    setModalError(null);
    setShowModal(true);
  };

  // Open edit modal
  const handleEdit = (item: ServiceType) => {
    setEditingItem(item);
    setFormData({
      name: item.name,
      category: item.category,
      description: item.description || '',
      default_priority: item.default_priority || 'medium',
      default_deadline_days: item.default_deadline_days || 30,
      is_recurring: item.is_recurring,
      recurrence_pattern: item.recurrence_pattern || '',
      hmrc_relevant: item.hmrc_relevant,
    });
    setModalError(null);
    setShowModal(true);
  };

  // Open requirements modal
  const handleOpenRequirements = (item: ServiceType) => {
    setRequirementsItem(item);
    setSelectedRequirements(item.required_docs || []);
    setShowRequirementsModal(true);
  };

  // Close modal
  const handleCloseModal = () => {
    setShowModal(false);
    setEditingItem(null);
    setModalError(null);
  };

  // Close requirements modal
  const handleCloseRequirementsModal = () => {
    setShowRequirementsModal(false);
    setRequirementsItem(null);
  };

  // Save (create or update)
  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!formData.name.trim()) {
      setModalError('Name is required');
      return;
    }

    try {
      setSaving(true);
      setModalError(null);

      const payload = {
        name: formData.name.trim(),
        category: formData.category,
        description: formData.description.trim() || undefined,
        default_priority: formData.default_priority,
        default_deadline_days: formData.default_deadline_days,
        is_recurring: formData.is_recurring,
        recurrence_pattern: formData.is_recurring ? formData.recurrence_pattern : undefined,
        hmrc_relevant: formData.hmrc_relevant,
      };

      if (editingItem) {
        await updateServiceType(editingItem.id, payload);
        toast.success('Service type updated');
      } else {
        await createServiceType(payload);
        toast.success('Service type created');
      }

      handleCloseModal();
      fetchServiceTypes();
    } catch (err) {
      setModalError(err instanceof Error ? err.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  // Save requirements
  const handleSaveRequirements = async () => {
    if (!requirementsItem) return;

    try {
      setSavingRequirements(true);
      await updateServiceType(requirementsItem.id, {
        required_docs: selectedRequirements,
      });
      toast.success('Requirements updated');
      handleCloseRequirementsModal();
      fetchServiceTypes();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update requirements');
    } finally {
      setSavingRequirements(false);
    }
  };

  // Toggle requirement
  const toggleRequirement = (docTypeId: string) => {
    setSelectedRequirements((prev) =>
      prev.includes(docTypeId)
        ? prev.filter((id) => id !== docTypeId)
        : [...prev, docTypeId]
    );
  };

  // Delete
  const handleDelete = async (id: string) => {
    try {
      setDeleting(true);
      await deleteServiceType(id);
      toast.success('Service type deleted');
      setDeleteConfirm(null);
      fetchServiceTypes();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete');
    } finally {
      setDeleting(false);
    }
  };

  // Loading state
  if (loading && serviceTypes.length === 0) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-slate-900 flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <header className="bg-white dark:bg-slate-800 shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4 flex justify-between items-center">
          <div className="flex items-center space-x-4">
            <Link
              href="/dashboard/settings"
              className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
              </svg>
            </Link>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Service Types</h1>
          </div>
          {isAdmin && (
            <button
              onClick={handleAdd}
              className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            >
              <span className="mr-2">+</span>
              Add
            </button>
          )}
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Error */}
        {error && (
          <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg">
            <p className="text-red-700 dark:text-red-300">{error}</p>
          </div>
        )}

        {/* Search and Filter */}
        <div className="mb-6 flex flex-col sm:flex-row gap-4">
          {/* Search */}
          <div className="flex-1">
            <div className="relative">
              <span className="absolute inset-y-0 left-0 pl-3 flex items-center text-gray-400">
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              </span>
              <input
                type="text"
                placeholder="Search service types..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
            </div>
          </div>

          {/* Category Filter */}
          <div className="sm:w-48">
            <select
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            >
              <option value="">All Categories</option>
              {categories.map((cat) => (
                <option key={cat} value={cat}>
                  {cat}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Service Types List */}
        <div className="bg-white dark:bg-slate-800 shadow rounded-lg overflow-hidden">
          {serviceTypes.length === 0 ? (
            <div className="p-8 text-center">
              <div className="text-4xl mb-4">📋</div>
              <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
                No service types found
              </h3>
              <p className="text-gray-500 dark:text-gray-400 mb-4">
                {search || categoryFilter
                  ? 'Try adjusting your search or filter'
                  : 'Get started by creating your first service type'}
              </p>
              {isAdmin && !search && !categoryFilter && (
                <button
                  onClick={handleAdd}
                  className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700"
                >
                  <span className="mr-2">+</span>
                  Add Service Type
                </button>
              )}
            </div>
          ) : (
            <div className="divide-y divide-gray-200 dark:divide-gray-700">
              {serviceTypes.map((serviceType) => (
                <div
                  key={serviceType.id}
                  className="p-4 hover:bg-gray-50 dark:hover:bg-slate-700/50 transition-colors"
                >
                  <div className="flex items-start justify-between">
                    {/* Left side - Info */}
                    <div className="flex items-start gap-3 flex-1 min-w-0">
                      <span className="text-2xl mt-0.5">📋</span>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <p className="text-sm font-medium text-gray-900 dark:text-white">
                            {serviceType.name}
                          </p>
                          {serviceType.hmrc_relevant && (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300">
                              HMRC
                            </span>
                          )}
                          {!serviceType.is_active && (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400">
                              Inactive
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-3 mt-1 flex-wrap">
                          <span className="text-xs text-gray-500 dark:text-gray-400">
                            Deadline: {formatDeadline(serviceType.default_deadline_days, serviceType.recurrence_pattern)}
                          </span>
                          <span className="text-xs text-gray-500 dark:text-gray-400">
                            Requires: {serviceType.required_docs?.length || 0} document{(serviceType.required_docs?.length || 0) !== 1 ? 's' : ''}
                          </span>
                          {serviceType.service_count !== undefined && serviceType.service_count > 0 && (
                            <span className="text-xs text-gray-500 dark:text-gray-400">
                              {serviceType.service_count} service{serviceType.service_count !== 1 ? 's' : ''}
                            </span>
                          )}
                        </div>
                        {serviceType.description && (
                          <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 truncate">
                            {serviceType.description}
                          </p>
                        )}
                      </div>
                    </div>

                    {/* Right side - Actions */}
                    {isAdmin && (
                      <div className="flex items-center gap-2 ml-4">
                        <button
                          onClick={() => handleEdit(serviceType)}
                          className="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-600 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-500 transition-colors"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => handleOpenRequirements(serviceType)}
                          className="px-3 py-1.5 text-sm font-medium text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/20 rounded-lg hover:bg-blue-100 dark:hover:bg-blue-900/40 transition-colors"
                        >
                          Requirements
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Count */}
        {serviceTypes.length > 0 && (
          <p className="mt-4 text-sm text-gray-500 dark:text-gray-400 text-center">
            {serviceTypes.length} service type{serviceTypes.length !== 1 ? 's' : ''}
          </p>
        )}
      </main>

      {/* Add/Edit Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          {/* Backdrop */}
          <div
            className="fixed inset-0 bg-black/50 transition-opacity"
            onClick={handleCloseModal}
          />

          {/* Modal */}
          <div className="relative min-h-screen flex items-center justify-center p-4">
            <div className="relative bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-lg w-full max-h-[90vh] overflow-y-auto">
              {/* Header */}
              <div className="sticky top-0 bg-white dark:bg-slate-800 p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                  {editingItem ? 'Edit Service Type' : 'Add Service Type'}
                </h3>
                <button
                  onClick={handleCloseModal}
                  className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              {/* Form */}
              <form onSubmit={handleSave} className="p-4 space-y-4">
                {/* Error */}
                {modalError && (
                  <div className="p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg">
                    <p className="text-sm text-red-700 dark:text-red-300">{modalError}</p>
                  </div>
                )}

                {/* Name */}
                <div>
                  <label htmlFor="name" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Name <span className="text-red-500">*</span>
                  </label>
                  <input
                    id="name"
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    placeholder="e.g., VAT Return (Quarterly)"
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    autoFocus
                  />
                </div>

                {/* Category */}
                <div>
                  <label htmlFor="category" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Category <span className="text-red-500">*</span>
                  </label>
                  <select
                    id="category"
                    value={formData.category}
                    onChange={(e) => setFormData({ ...formData, category: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  >
                    {categories.map((cat) => (
                      <option key={cat} value={cat}>
                        {cat}
                      </option>
                    ))}
                  </select>
                </div>

                {/* Description */}
                <div>
                  <label htmlFor="description" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                    Description
                  </label>
                  <textarea
                    id="description"
                    value={formData.description}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    placeholder="Optional description..."
                    rows={2}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  />
                </div>

                {/* Priority and Deadline */}
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="priority" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                      Default Priority
                    </label>
                    <select
                      id="priority"
                      value={formData.default_priority}
                      onChange={(e) => setFormData({ ...formData, default_priority: e.target.value })}
                      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    >
                      {PRIORITY_OPTIONS.map((p) => (
                        <option key={p} value={p}>
                          {p.charAt(0).toUpperCase() + p.slice(1)}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label htmlFor="deadline" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                      Deadline (days)
                    </label>
                    <input
                      id="deadline"
                      type="number"
                      min="1"
                      max="365"
                      value={formData.default_deadline_days}
                      onChange={(e) => setFormData({ ...formData, default_deadline_days: parseInt(e.target.value) || 30 })}
                      className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    />
                  </div>
                </div>

                {/* Recurring */}
                <div className="space-y-3">
                  <label className="flex items-center">
                    <input
                      type="checkbox"
                      checked={formData.is_recurring}
                      onChange={(e) => setFormData({ ...formData, is_recurring: e.target.checked })}
                      className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                    />
                    <span className="ml-3 text-sm text-gray-700 dark:text-gray-300">
                      Recurring service
                    </span>
                  </label>

                  {formData.is_recurring && (
                    <div className="ml-7">
                      <label htmlFor="recurrence" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                        Recurrence Pattern
                      </label>
                      <select
                        id="recurrence"
                        value={formData.recurrence_pattern}
                        onChange={(e) => setFormData({ ...formData, recurrence_pattern: e.target.value })}
                        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      >
                        {RECURRENCE_OPTIONS.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                      </select>
                    </div>
                  )}
                </div>

                {/* HMRC Relevant */}
                <label className="flex items-center">
                  <input
                    type="checkbox"
                    checked={formData.hmrc_relevant}
                    onChange={(e) => setFormData({ ...formData, hmrc_relevant: e.target.checked })}
                    className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                  />
                  <span className="ml-3 text-sm text-gray-700 dark:text-gray-300">
                    HMRC relevant (tax filing)
                  </span>
                </label>

                {/* Footer */}
                <div className="pt-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
                  <button
                    type="button"
                    onClick={handleCloseModal}
                    className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={saving}
                    className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 transition-colors inline-flex items-center"
                  >
                    {saving ? (
                      <>
                        <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Saving...
                      </>
                    ) : editingItem ? (
                      'Update'
                    ) : (
                      'Create'
                    )}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      )}

      {/* Requirements Modal */}
      {showRequirementsModal && requirementsItem && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          {/* Backdrop */}
          <div
            className="fixed inset-0 bg-black/50 transition-opacity"
            onClick={handleCloseRequirementsModal}
          />

          {/* Modal */}
          <div className="relative min-h-screen flex items-center justify-center p-4">
            <div className="relative bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-md w-full max-h-[80vh] flex flex-col">
              {/* Header */}
              <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                    Required Documents
                  </h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    {requirementsItem.name}
                  </p>
                </div>
                <button
                  onClick={handleCloseRequirementsModal}
                  className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              {/* Document Types List */}
              <div className="flex-1 overflow-y-auto p-4">
                {documentTypes.length === 0 ? (
                  <p className="text-sm text-gray-500 dark:text-gray-400 text-center py-4">
                    No document types available. Create some first.
                  </p>
                ) : (
                  <div className="space-y-2">
                    {documentTypes.map((docType) => (
                      <label
                        key={docType.id}
                        className="flex items-center p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg hover:bg-gray-100 dark:hover:bg-slate-700 cursor-pointer transition-colors"
                      >
                        <input
                          type="checkbox"
                          checked={selectedRequirements.includes(docType.id)}
                          onChange={() => toggleRequirement(docType.id)}
                          className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                        />
                        <div className="ml-3 flex-1">
                          <p className="text-sm font-medium text-gray-900 dark:text-white">
                            {docType.name}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400">
                            {docType.category}
                          </p>
                        </div>
                      </label>
                    ))}
                  </div>
                )}
              </div>

              {/* Footer */}
              <div className="p-4 border-t border-gray-200 dark:border-gray-700 flex justify-between items-center">
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  {selectedRequirements.length} selected
                </p>
                <div className="flex gap-3">
                  <button
                    type="button"
                    onClick={handleCloseRequirementsModal}
                    className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    onClick={handleSaveRequirements}
                    disabled={savingRequirements}
                    className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 transition-colors inline-flex items-center"
                  >
                    {savingRequirements ? (
                      <>
                        <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Saving...
                      </>
                    ) : (
                      'Save'
                    )}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteConfirm && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          {/* Backdrop */}
          <div
            className="fixed inset-0 bg-black/50 transition-opacity"
            onClick={() => setDeleteConfirm(null)}
          />

          {/* Modal */}
          <div className="relative min-h-screen flex items-center justify-center p-4">
            <div className="relative bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-sm w-full p-6">
              <div className="text-center">
                <div className="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-red-100 dark:bg-red-900/30 mb-4">
                  <svg className="h-6 w-6 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                </div>
                <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
                  Delete Service Type
                </h3>
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-6">
                  Are you sure you want to delete this service type? This action cannot be undone.
                </p>
                <div className="flex justify-center gap-3">
                  <button
                    type="button"
                    onClick={() => setDeleteConfirm(null)}
                    disabled={deleting}
                    className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    onClick={() => handleDelete(deleteConfirm)}
                    disabled={deleting}
                    className="px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-lg hover:bg-red-700 disabled:opacity-50 transition-colors inline-flex items-center"
                  >
                    {deleting ? (
                      <>
                        <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Deleting...
                      </>
                    ) : (
                      'Delete'
                    )}
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
