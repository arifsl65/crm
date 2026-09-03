'use client';

import { useState } from 'react';
import { createClient } from '@/lib/api';

interface AddClientPanelProps {
  onClose: () => void;
  onClientCreated: () => void;
}

interface FormData {
  company_name: string;
  contact_name: string;
  email: string;
  phone: string;
  address: string;
  company_number: string;
  company_type: string;
  year_end: string;
  utr: string;
  vat_number: string;
  vat_quarter: string;
}

interface CHCompany {
  company_number: string;
  title: string;
  company_status: string;
  address_snippet: string;
  date_of_creation: string;
  company_type?: string;
}

export function AddClientPanel({ onClose, onClientCreated }: AddClientPanelProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showCHLookup, setShowCHLookup] = useState(false);
  const [chSearchQuery, setChSearchQuery] = useState('');
  const [chSearching, setChSearching] = useState(false);
  const [chResults, setChResults] = useState<CHCompany[]>([]);

  const [formData, setFormData] = useState<FormData>({
    company_name: '',
    contact_name: '',
    email: '',
    phone: '',
    address: '',
    company_number: '',
    company_type: 'limited_company',
    year_end: '',
    utr: '',
    vat_number: '',
    vat_quarter: '',
  });

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleCHSearch = async () => {
    if (!chSearchQuery.trim()) return;
    setChSearching(true);
    // Simulate Companies House search - in production this would call the CH API
    setTimeout(() => {
      setChResults([
        {
          company_number: '12345678',
          title: chSearchQuery.toUpperCase() + ' LTD',
          company_status: 'active',
          address_snippet: '123 Business Street, London, SW1A 1AA',
          date_of_creation: '2020-01-15',
          company_type: 'ltd',
        },
      ]);
      setChSearching(false);
    }, 800);
  };

  const handleCHSelect = (company: CHCompany) => {
    setFormData(prev => ({
      ...prev,
      company_name: company.title,
      company_number: company.company_number,
      address: company.address_snippet,
      company_type: company.company_type === 'ltd' ? 'limited_company' : company.company_type || 'limited_company',
    }));
    setShowCHLookup(false);
    setChResults([]);
    setChSearchQuery('');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const data: Record<string, string> = {};
      Object.entries(formData).forEach(([key, value]) => {
        if (value.trim()) {
          data[key] = value.trim();
        }
      });

      await createClient(data);
      onClientCreated();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create client');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>➕</span> Add New Client
        </h2>
        <button
          onClick={onClose}
          className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        >
          ✕
        </button>
      </div>

      {/* CH Lookup Toggle */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 bg-blue-50 dark:bg-blue-900/20">
        <button
          onClick={() => setShowCHLookup(!showCHLookup)}
          className="flex items-center gap-2 text-sm text-blue-600 dark:text-blue-400 hover:underline"
        >
          <span>🔍</span>
          {showCHLookup ? 'Hide Companies House Lookup' : 'Search Companies House to auto-fill'}
        </button>

        {showCHLookup && (
          <div className="mt-3">
            <div className="flex gap-2">
              <input
                type="text"
                placeholder="Company name or number..."
                value={chSearchQuery}
                onChange={(e) => setChSearchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleCHSearch()}
                className="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
              />
              <button
                onClick={handleCHSearch}
                disabled={chSearching}
                className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {chSearching ? '...' : 'Search'}
              </button>
            </div>

            {chResults.length > 0 && (
              <div className="mt-2 space-y-2">
                {chResults.map((company) => (
                  <div
                    key={company.company_number}
                    onClick={() => handleCHSelect(company)}
                    className="p-3 bg-white dark:bg-slate-700 rounded-lg border border-gray-200 dark:border-gray-600 hover:border-blue-500 cursor-pointer"
                  >
                    <p className="text-sm font-medium text-gray-900 dark:text-white">{company.title}</p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {company.company_number} • {company.address_snippet}
                    </p>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Form */}
      <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-4 space-y-6">
        {error && (
          <div className="p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
            <p className="text-red-700 dark:text-red-300 text-sm">{error}</p>
          </div>
        )}

        {/* Basic Information */}
        <div>
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Basic Information</h3>
          <div className="space-y-4">
            <div>
              <label htmlFor="company_name" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                Company Name *
              </label>
              <input
                type="text"
                name="company_name"
                id="company_name"
                required
                value={formData.company_name}
                onChange={handleChange}
                className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label htmlFor="contact_name" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  Contact Name *
                </label>
                <input
                  type="text"
                  name="contact_name"
                  id="contact_name"
                  required
                  value={formData.contact_name}
                  onChange={handleChange}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label htmlFor="email" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  Email *
                </label>
                <input
                  type="email"
                  name="email"
                  id="email"
                  required
                  value={formData.email}
                  onChange={handleChange}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label htmlFor="phone" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  Phone
                </label>
                <input
                  type="tel"
                  name="phone"
                  id="phone"
                  value={formData.phone}
                  onChange={handleChange}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label htmlFor="company_type" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  Company Type
                </label>
                <select
                  name="company_type"
                  id="company_type"
                  value={formData.company_type}
                  onChange={handleChange}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                >
                  <option value="limited_company">Limited Company</option>
                  <option value="llp">LLP</option>
                  <option value="sole_trader">Sole Trader</option>
                  <option value="partnership">Partnership</option>
                  <option value="charity">Charity</option>
                  <option value="other">Other</option>
                </select>
              </div>
            </div>

            <div>
              <label htmlFor="address" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                Address
              </label>
              <textarea
                name="address"
                id="address"
                rows={2}
                value={formData.address}
                onChange={handleChange}
                className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
              />
            </div>
          </div>
        </div>

        {/* Company Details */}
        <div>
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Company Details</h3>
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label htmlFor="company_number" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  Company Number
                </label>
                <input
                  type="text"
                  name="company_number"
                  id="company_number"
                  placeholder="12345678"
                  value={formData.company_number}
                  onChange={handleChange}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label htmlFor="year_end" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  Year End
                </label>
                <input
                  type="text"
                  name="year_end"
                  id="year_end"
                  placeholder="31 March"
                  value={formData.year_end}
                  onChange={handleChange}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label htmlFor="utr" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  UTR
                </label>
                <input
                  type="text"
                  name="utr"
                  id="utr"
                  placeholder="10 digit number"
                  value={formData.utr}
                  onChange={handleChange}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>

              <div>
                <label htmlFor="vat_number" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                  VAT Number
                </label>
                <input
                  type="text"
                  name="vat_number"
                  id="vat_number"
                  placeholder="GB123456789"
                  value={formData.vat_number}
                  onChange={handleChange}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
                />
              </div>
            </div>

            <div>
              <label htmlFor="vat_quarter" className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">
                VAT Quarter
              </label>
              <select
                name="vat_quarter"
                id="vat_quarter"
                value={formData.vat_quarter}
                onChange={handleChange}
                className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
              >
                <option value="">Not VAT registered</option>
                <option value="Q1">Q1 (Jan-Mar)</option>
                <option value="Q2">Q2 (Apr-Jun)</option>
                <option value="Q3">Q3 (Jul-Sep)</option>
                <option value="Q4">Q4 (Oct-Dec)</option>
              </select>
            </div>
          </div>
        </div>
      </form>

      {/* Actions */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
        <button
          type="button"
          onClick={onClose}
          className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600"
        >
          Cancel
        </button>
        <button
          onClick={handleSubmit}
          disabled={loading}
          className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? 'Creating...' : 'Create Client'}
        </button>
      </div>
    </div>
  );
}

export default AddClientPanel;
