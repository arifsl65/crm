'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/components/auth-guard';
import {
  searchCompaniesHouse,
  getCompanyFromCH,
  createClient,
  CHCompanySearchResult,
  CHCompanyProfile,
} from '@/lib/api';

export default function CompaniesHouseSearchPage() {
  const { user } = useAuth();
  const router = useRouter();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<CHCompanySearchResult[]>([]);
  const [totalResults, setTotalResults] = useState(0);
  const [searching, setSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [importing, setImporting] = useState<string | null>(null);
  const [selectedCompany, setSelectedCompany] = useState<CHCompanyProfile | null>(null);
  const [showImportModal, setShowImportModal] = useState(false);

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;

    setSearching(true);
    setError(null);
    try {
      const data = await searchCompaniesHouse(query.trim());
      setResults(data.items || []);
      setTotalResults(data.total_results || 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      setResults([]);
    } finally {
      setSearching(false);
    }
  };

  const handleViewCompany = async (companyNumber: string) => {
    setImporting(companyNumber);
    try {
      const company = await getCompanyFromCH(companyNumber);
      setSelectedCompany(company);
      setShowImportModal(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch company details');
    } finally {
      setImporting(null);
    }
  };

  const handleImportCompany = async () => {
    if (!selectedCompany) return;

    setImporting(selectedCompany.company_number);
    try {
      const result = await createClient({
        company_name: selectedCompany.company_name,
        company_number: selectedCompany.company_number,
        company_type: selectedCompany.company_type,
        incorporation_date: selectedCompany.date_of_creation,
        address: formatAddress(selectedCompany.registered_office_address),
        status: 'active',
        contact_name: '',
        email: '',
      });
      router.push(`/dashboard/clients/${result.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to import company');
      setImporting(null);
    }
  };

  const formatAddress = (address?: CHCompanyProfile['registered_office_address']) => {
    if (!address) return '';
    const parts = [
      address.address_line_1,
      address.address_line_2,
      address.locality,
      address.region,
      address.postal_code,
    ].filter(Boolean);
    return parts.join(', ');
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-';
    const date = new Date(dateStr);
    return date.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
  };

  const getStatusColor = (status?: string) => {
    switch (status?.toLowerCase()) {
      case 'active':
        return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400';
      case 'dissolved':
        return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400';
      case 'liquidation':
        return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400';
      default:
        return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300';
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <header className="bg-white dark:bg-slate-800 shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <Link
                href="/dashboard/clients"
                className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
              </Link>
              <div>
                <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center">
                  <span className="mr-2">🏛️</span>
                  Companies House Search
                </h1>
                <p className="text-sm text-gray-500 dark:text-gray-400">Search and import UK companies</p>
              </div>
            </div>
          </div>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Search Form */}
        <form onSubmit={handleSearch} className="mb-8">
          <div className="flex space-x-4">
            <div className="flex-1 relative">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <svg className="h-5 w-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              </div>
              <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search by company name or number..."
                className="block w-full pl-10 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
            </div>
            <button
              type="submit"
              disabled={searching || !query.trim()}
              className="px-6 py-3 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {searching ? 'Searching...' : 'Search'}
            </button>
          </div>
        </form>

        {/* Error Message */}
        {error && (
          <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
            <p className="text-red-600 dark:text-red-400">{error}</p>
          </div>
        )}

        {/* Results */}
        {results.length > 0 && (
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Found <span className="font-medium text-gray-900 dark:text-white">{totalResults}</span> companies
              </p>
            </div>
            <ul className="divide-y divide-gray-200 dark:divide-gray-700">
              {results.map((company) => (
                <li key={company.company_number} className="p-6 hover:bg-gray-50 dark:hover:bg-slate-700">
                  <div className="flex items-start justify-between">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center space-x-3">
                        <span className="text-xl">🏢</span>
                        <div>
                          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                            {company.company_name}
                          </h3>
                          <div className="mt-1 flex items-center space-x-3 text-sm text-gray-500 dark:text-gray-400">
                            <span>#{company.company_number}</span>
                            <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${getStatusColor(company.company_status)}`}>
                              {company.company_status || 'Unknown'}
                            </span>
                          </div>
                        </div>
                      </div>
                      {company.date_of_creation && (
                        <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                          Incorporated: {formatDate(company.date_of_creation)}
                        </p>
                      )}
                      {company.address_snippet && (
                        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400 truncate">
                          {company.address_snippet}
                        </p>
                      )}
                    </div>
                    <button
                      onClick={() => handleViewCompany(company.company_number)}
                      disabled={importing === company.company_number}
                      className="ml-4 inline-flex items-center px-4 py-2 text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 bg-blue-50 hover:bg-blue-100 dark:bg-blue-900/20 dark:hover:bg-blue-900/30 rounded-lg disabled:opacity-50"
                    >
                      {importing === company.company_number ? (
                        <>
                          <svg className="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                          </svg>
                          Loading...
                        </>
                      ) : (
                        <>
                          Import
                          <svg className="ml-1.5 w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                          </svg>
                        </>
                      )}
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Empty State */}
        {!searching && results.length === 0 && query && (
          <div className="text-center py-12">
            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
            </svg>
            <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white">No companies found</h3>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">Try a different search term</p>
          </div>
        )}

        {/* Initial State */}
        {!searching && results.length === 0 && !query && (
          <div className="text-center py-12">
            <svg className="mx-auto h-16 w-16 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <h3 className="mt-4 text-lg font-medium text-gray-900 dark:text-white">Search Companies House</h3>
            <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
              Enter a company name or number to search the UK Companies House register
            </p>
          </div>
        )}
      </main>

      {/* Import Modal */}
      {showImportModal && selectedCompany && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          <div className="flex min-h-screen items-center justify-center p-4">
            <div className="fixed inset-0 bg-black/50" onClick={() => setShowImportModal(false)} />
            <div className="relative bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-lg w-full p-6">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Import Company</h3>
                <button
                  onClick={() => setShowImportModal(false)}
                  className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">Company Name</label>
                  <p className="mt-1 text-gray-900 dark:text-white font-medium">{selectedCompany.company_name}</p>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">Company Number</label>
                    <p className="mt-1 text-gray-900 dark:text-white">{selectedCompany.company_number}</p>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">Status</label>
                    <p className="mt-1">
                      <span className={`px-2 py-1 text-xs font-medium rounded-full ${getStatusColor(selectedCompany.company_status)}`}>
                        {selectedCompany.company_status || 'Unknown'}
                      </span>
                    </p>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">Type</label>
                    <p className="mt-1 text-gray-900 dark:text-white">{selectedCompany.company_type || '-'}</p>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">Incorporated</label>
                    <p className="mt-1 text-gray-900 dark:text-white">{formatDate(selectedCompany.date_of_creation)}</p>
                  </div>
                </div>
                {selectedCompany.registered_office_address && (
                  <div>
                    <label className="block text-sm font-medium text-gray-500 dark:text-gray-400">Registered Address</label>
                    <p className="mt-1 text-gray-900 dark:text-white">
                      {formatAddress(selectedCompany.registered_office_address)}
                    </p>
                  </div>
                )}
              </div>

              <div className="mt-6 flex justify-end space-x-3">
                <button
                  onClick={() => setShowImportModal(false)}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  onClick={handleImportCompany}
                  disabled={importing !== null}
                  className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 rounded-lg"
                >
                  {importing ? 'Importing...' : 'Import as Client'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
