'use client';

import { useEffect, useState } from 'react';
import { useParams, usePathname } from 'next/navigation';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import {
  Client,
  Service,
  Document,
  Director,
  PSC,
  CHFiling,
  getClient,
  getClientDocuments,
  getClientServices,
  getClientDirectors,
  getClientPSC,
  getCHFilings,
  syncClientWithCH,
} from '@/lib/api';
import { getStatusBadgeClass, formatStatus } from '@/lib/status';
import { SkeletonCard } from '@/components';

type MainTab = 'overview' | 'services' | 'documents' | 'companies-house';
type CHTab = 'info' | 'officers' | 'filings' | 'psc';

export default function ClientDetail() {
  const { user } = useAuth();
  const params = useParams();
  const pathname = usePathname();

  // Extract client ID from pathname for static export compatibility
  // useParams() returns 'placeholder' during static export, so we parse the URL instead
  const clientId = pathname?.split('/').pop() || (params.id as string);

  const [client, setClient] = useState<Client | null>(null);
  const [services, setServices] = useState<Service[]>([]);
  const [documents, setDocuments] = useState<Document[]>([]);
  const [directors, setDirectors] = useState<Director[]>([]);
  const [pscList, setPscList] = useState<PSC[]>([]);
  const [filings, setFilings] = useState<CHFiling[]>([]);
  const [filingsTotal, setFilingsTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<MainTab>('overview');
  const [chTab, setChTab] = useState<CHTab>('info');
  const [syncing, setSyncing] = useState(false);
  const [lastSynced, setLastSynced] = useState<string | null>(null);
  const [chLoading, setChLoading] = useState(false);

  useEffect(() => {
    // Skip fetch if clientId is the static placeholder (happens during hydration)
    if (!clientId || clientId === 'placeholder') return;

    async function fetchData() {
      try {
        setLoading(true);
        const [clientData, servicesData, documentsData] = await Promise.all([
          getClient(clientId),
          getClientServices(clientId),
          getClientDocuments(clientId),
        ]);
        setClient(clientData);
        setServices(servicesData.services || []);
        setDocuments(documentsData.documents || []);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load client');
      } finally {
        setLoading(false);
      }
    }
    fetchData();
  }, [clientId]);

  // Load CH data when Companies House tab is selected
  useEffect(() => {
    async function fetchCHData() {
      // Skip if placeholder or missing prerequisites
      if (!clientId || clientId === 'placeholder') return;
      if (activeTab !== 'companies-house' || !client?.company_number) return;

      setChLoading(true);
      try {
        const [directorsData, pscData, filingsData] = await Promise.all([
          getClientDirectors(clientId),
          getClientPSC(clientId),
          getCHFilings(client.company_number, { items_per_page: 10 }),
        ]);
        setDirectors(directorsData.directors || []);
        setPscList(pscData.psc || []);
        setFilings(filingsData.items || []);
        setFilingsTotal(filingsData.total_count || 0);
      } catch (err) {
        console.error('Failed to load CH data:', err);
      } finally {
        setChLoading(false);
      }
    }
    fetchCHData();
  }, [activeTab, clientId, client?.company_number]);

  const handleSyncCH = async () => {
    if (!client) return;
    setSyncing(true);
    try {
      await syncClientWithCH(clientId);
      setLastSynced(new Date().toLocaleTimeString());
      // Reload CH data
      const [directorsData, pscData] = await Promise.all([
        getClientDirectors(clientId),
        getClientPSC(clientId),
      ]);
      setDirectors(directorsData.directors || []);
      setPscList(pscData.psc || []);
      if (client.company_number) {
        const filingsData = await getCHFilings(client.company_number, { items_per_page: 10 });
        setFilings(filingsData.items || []);
        setFilingsTotal(filingsData.total_count || 0);
      }
    } catch (err) {
      console.error('Failed to sync with CH:', err);
    } finally {
      setSyncing(false);
    }
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-';
    const date = new Date(dateStr);
    return date.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: '2-digit' });
  };

  const formatDOB = (month?: number, year?: number) => {
    if (!month || !year) return null;
    const months = ['January', 'February', 'March', 'April', 'May', 'June',
      'July', 'August', 'September', 'October', 'November', 'December'];
    return `${months[month - 1]} ${year}`;
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-slate-900 p-8">
        <div className="max-w-4xl mx-auto space-y-6">
          <SkeletonCard />
          <SkeletonCard />
        </div>
      </div>
    );
  }

  if (error || !client) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-slate-900 flex items-center justify-center">
        <div className="text-center">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            {error || 'Client not found'}
          </h2>
          <Link
            href="/dashboard/clients"
            className="mt-4 inline-flex items-center text-blue-600 hover:text-blue-700"
          >
            <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
            </svg>
            Back to clients
          </Link>
        </div>
      </div>
    );
  }

  const activeDirectors = directors.filter(d => d.is_active && d.role === 'director');
  const secretary = directors.find(d => d.is_active && d.role === 'secretary');
  const activePSC = pscList.filter(p => p.is_active);

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
                <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
                  {client.company_name}
                </h1>
                <p className="text-sm text-gray-500 dark:text-gray-400">{client.contact_name}</p>
              </div>
            </div>
            <span className={`px-3 py-1 text-sm font-medium rounded-full ${getStatusBadgeClass(client.status)}`}>
              {client.status}
            </span>
          </div>
        </div>
      </header>

      {/* Main Tabs */}
      <div className="bg-white dark:bg-slate-800 border-b border-gray-200 dark:border-gray-700">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <nav className="flex space-x-8">
            {([
              { key: 'overview', label: 'Overview', icon: undefined, count: undefined },
              { key: 'services', label: 'Services', icon: undefined, count: services.length },
              { key: 'documents', label: 'Documents', icon: undefined, count: documents.length },
              { key: 'companies-house', label: 'Companies House', icon: '🏛️', count: undefined },
            ] as const).map((tab) => (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={`py-4 px-1 border-b-2 font-medium text-sm flex items-center space-x-2 ${
                  activeTab === tab.key
                    ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
              >
                {tab.icon && <span>{tab.icon}</span>}
                <span>{tab.label}</span>
                {tab.count !== undefined && tab.count > 0 && (
                  <span className="ml-2 px-2 py-0.5 text-xs rounded-full bg-gray-100 dark:bg-gray-700">
                    {tab.count}
                  </span>
                )}
              </button>
            ))}
          </nav>
        </div>
      </div>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {activeTab === 'overview' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Contact Info */}
            <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Contact Information</h3>
              <dl className="space-y-3">
                <div>
                  <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Email</dt>
                  <dd className="text-sm text-gray-900 dark:text-white">{client.email}</dd>
                </div>
                {client.phone && (
                  <div>
                    <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Phone</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">{client.phone}</dd>
                  </div>
                )}
                {client.address && (
                  <div>
                    <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Address</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">{client.address}</dd>
                  </div>
                )}
              </dl>
            </div>

            {/* Company Info */}
            <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">Company Information</h3>
              <dl className="space-y-3">
                {client.company_number && (
                  <div>
                    <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Company Number</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">{client.company_number}</dd>
                  </div>
                )}
                {client.company_type && (
                  <div>
                    <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Company Type</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">{client.company_type}</dd>
                  </div>
                )}
                {client.year_end && (
                  <div>
                    <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Year End</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">{client.year_end}</dd>
                  </div>
                )}
                {client.utr && (
                  <div>
                    <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">UTR</dt>
                    <dd className="text-sm text-gray-900 dark:text-white">{client.utr}</dd>
                  </div>
                )}
              </dl>
            </div>

            {/* VAT Info */}
            {(client.vat_number || client.vat_quarter) && (
              <div className="bg-white dark:bg-slate-800 rounded-lg shadow p-6">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">VAT Information</h3>
                <dl className="space-y-3">
                  {client.vat_number && (
                    <div>
                      <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">VAT Number</dt>
                      <dd className="text-sm text-gray-900 dark:text-white">{client.vat_number}</dd>
                    </div>
                  )}
                  {client.vat_quarter && (
                    <div>
                      <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">VAT Quarter</dt>
                      <dd className="text-sm text-gray-900 dark:text-white capitalize">{client.vat_quarter}</dd>
                    </div>
                  )}
                </dl>
              </div>
            )}
          </div>
        )}

        {activeTab === 'services' && (
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
            {services.length === 0 ? (
              <div className="p-12 text-center">
                <p className="text-gray-500 dark:text-gray-400">No services found for this client.</p>
              </div>
            ) : (
              <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead className="bg-gray-50 dark:bg-slate-700">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Name</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Status</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Priority</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Deadline</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                  {services.map((service) => (
                    <tr key={service.id} className="hover:bg-gray-50 dark:hover:bg-slate-700">
                      <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">{service.name}</td>
                      <td className="px-6 py-4">
                        <span className={`px-2 py-1 text-xs font-medium rounded-full ${getStatusBadgeClass(service.status)}`}>
                          {formatStatus(service.status)}
                        </span>
                      </td>
                      <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400 capitalize">{service.priority}</td>
                      <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">{service.deadline || '-'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {activeTab === 'documents' && (
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
            {documents.length === 0 ? (
              <div className="p-12 text-center">
                <p className="text-gray-500 dark:text-gray-400">No documents found for this client.</p>
              </div>
            ) : (
              <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead className="bg-gray-50 dark:bg-slate-700">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Name</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Status</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Size</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">Uploaded</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                  {documents.map((doc) => (
                    <tr key={doc.id} className="hover:bg-gray-50 dark:hover:bg-slate-700">
                      <td className="px-6 py-4 text-sm text-gray-900 dark:text-white">{doc.name}</td>
                      <td className="px-6 py-4">
                        <span className={`px-2 py-1 text-xs font-medium rounded-full ${getStatusBadgeClass(doc.status)}`}>
                          {formatStatus(doc.status)}
                        </span>
                      </td>
                      <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                        {doc.file_size ? `${Math.round(doc.file_size / 1024)} KB` : '-'}
                      </td>
                      <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                        {new Date(doc.created_at).toLocaleDateString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {activeTab === 'companies-house' && (
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow">
            {!client.company_number ? (
              <div className="p-12 text-center">
                <p className="text-gray-500 dark:text-gray-400">
                  No company number found. Add a company number to view Companies House data.
                </p>
              </div>
            ) : (
              <>
                {/* CH Section Header */}
                <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                  <div className="flex items-center justify-between">
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center space-x-2">
                      <span>🏛️</span>
                      <span>COMPANIES HOUSE</span>
                    </h3>
                    <button
                      onClick={handleSyncCH}
                      disabled={syncing}
                      className="inline-flex items-center px-3 py-1.5 text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 disabled:opacity-50"
                    >
                      <svg
                        className={`w-4 h-4 mr-1.5 ${syncing ? 'animate-spin' : ''}`}
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                        />
                      </svg>
                      {syncing ? 'Syncing...' : 'Refresh CH'}
                    </button>
                  </div>

                  {/* CH Nested Tabs */}
                  <div className="mt-4 flex space-x-1 bg-gray-100 dark:bg-slate-700 rounded-lg p-1">
                    {(['info', 'officers', 'filings', 'psc'] as const).map((tab) => (
                      <button
                        key={tab}
                        onClick={() => setChTab(tab)}
                        className={`flex-1 py-2 px-4 text-sm font-medium rounded-md capitalize transition-colors ${
                          chTab === tab
                            ? 'bg-white dark:bg-slate-600 text-gray-900 dark:text-white shadow'
                            : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
                        }`}
                      >
                        {tab === 'psc' ? 'PSC' : tab}
                      </button>
                    ))}
                  </div>
                </div>

                {/* CH Tab Content */}
                <div className="p-6">
                  {chLoading ? (
                    <div className="flex justify-center py-8">
                      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                    </div>
                  ) : (
                    <>
                      {/* Info Tab */}
                      {chTab === 'info' && (
                        <div className="space-y-4">
                          <dl className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div>
                              <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Company Number</dt>
                              <dd className="mt-1 text-sm text-gray-900 dark:text-white">{client.company_number}</dd>
                            </div>
                            {client.company_type && (
                              <div>
                                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Company Type</dt>
                                <dd className="mt-1 text-sm text-gray-900 dark:text-white">{client.company_type}</dd>
                              </div>
                            )}
                            {client.incorporation_date && (
                              <div>
                                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Incorporated</dt>
                                <dd className="mt-1 text-sm text-gray-900 dark:text-white">{formatDate(client.incorporation_date)}</dd>
                              </div>
                            )}
                            {client.address && (
                              <div className="md:col-span-2">
                                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Registered Address</dt>
                                <dd className="mt-1 text-sm text-gray-900 dark:text-white">{client.address}</dd>
                              </div>
                            )}
                          </dl>
                        </div>
                      )}

                      {/* Officers Tab */}
                      {chTab === 'officers' && (
                        <div className="space-y-6">
                          {/* Directors Section */}
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 flex items-center">
                              <span className="mr-2">👤</span>
                              DIRECTORS
                            </h4>
                            {activeDirectors.length === 0 ? (
                              <p className="text-sm text-gray-500 dark:text-gray-400">No directors found</p>
                            ) : (
                              <div className="space-y-3">
                                {activeDirectors.map((director) => (
                                  <div
                                    key={director.id}
                                    className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4"
                                  >
                                    <div className="font-medium text-gray-900 dark:text-white">
                                      {director.name}
                                    </div>
                                    <div className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                                      Director • Appointed: {formatDate(director.appointed_date)}
                                    </div>
                                    {director.nationality && (
                                      <div className="text-sm text-gray-500 dark:text-gray-400">
                                        Nationality: {director.nationality}
                                      </div>
                                    )}
                                    {formatDOB(director.dob_month, director.dob_year) && (
                                      <div className="text-sm text-gray-500 dark:text-gray-400">
                                        DOB: {formatDOB(director.dob_month, director.dob_year)}
                                      </div>
                                    )}
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>

                          {/* Secretary Section */}
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 flex items-center">
                              <span className="mr-2">📝</span>
                              SECRETARY
                            </h4>
                            {secretary ? (
                              <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
                                <div className="font-medium text-gray-900 dark:text-white">
                                  {secretary.name}
                                </div>
                                <div className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                                  Appointed: {formatDate(secretary.appointed_date)}
                                </div>
                              </div>
                            ) : (
                              <p className="text-sm text-gray-500 dark:text-gray-400">None appointed</p>
                            )}
                          </div>

                          {/* Last Synced */}
                          {lastSynced && (
                            <p className="text-xs text-gray-400 dark:text-gray-500 mt-4">
                              Last synced: {lastSynced}
                            </p>
                          )}
                        </div>
                      )}

                      {/* Filings Tab */}
                      {chTab === 'filings' && (
                        <div>
                          <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
                            <span className="mr-2">📄</span>
                            FILING HISTORY
                          </h4>
                          {filings.length === 0 ? (
                            <p className="text-sm text-gray-500 dark:text-gray-400">No filings found</p>
                          ) : (
                            <>
                              <div className="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
                                <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                                  <thead className="bg-gray-50 dark:bg-slate-700">
                                    <tr>
                                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">
                                        Date
                                      </th>
                                      <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">
                                        Type
                                      </th>
                                      <th className="px-4 py-3 text-center text-xs font-medium text-gray-500 dark:text-gray-300 uppercase">
                                        Status
                                      </th>
                                    </tr>
                                  </thead>
                                  <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                                    {filings.map((filing) => (
                                      <tr key={filing.transaction_id} className="hover:bg-gray-50 dark:hover:bg-slate-700">
                                        <td className="px-4 py-3 text-sm text-gray-900 dark:text-white whitespace-nowrap">
                                          {formatDate(filing.date)}
                                        </td>
                                        <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                                          {filing.description || filing.type}
                                        </td>
                                        <td className="px-4 py-3 text-center">
                                          <span className="text-green-500">✅</span>
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>

                              {filingsTotal > filings.length && (
                                <p className="mt-3 text-sm text-gray-500 dark:text-gray-400 text-center">
                                  Showing {filings.length} of {filingsTotal} filings
                                </p>
                              )}

                              <div className="mt-4 p-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
                                <p className="text-sm text-green-700 dark:text-green-400 flex items-center">
                                  <span className="mr-2">⚠️</span>
                                  No overdue filings
                                </p>
                              </div>
                            </>
                          )}
                        </div>
                      )}

                      {/* PSC Tab */}
                      {chTab === 'psc' && (
                        <div>
                          <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
                            <span className="mr-2">👥</span>
                            PERSONS WITH SIGNIFICANT CONTROL
                          </h4>
                          {activePSC.length === 0 ? (
                            <p className="text-sm text-gray-500 dark:text-gray-400">No PSC records found</p>
                          ) : (
                            <div className="space-y-3">
                              {activePSC.map((psc) => (
                                <div
                                  key={psc.id}
                                  className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4"
                                >
                                  <div className="font-medium text-gray-900 dark:text-white">
                                    {psc.name}
                                  </div>
                                  {psc.ownership_percentage && (
                                    <div className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                                      Ownership: {psc.ownership_percentage} shares
                                    </div>
                                  )}
                                  {psc.ownership_percentage && (
                                    <div className="text-sm text-gray-500 dark:text-gray-400">
                                      Voting: {psc.ownership_percentage} rights
                                    </div>
                                  )}
                                  {psc.notified_date && (
                                    <div className="text-sm text-gray-500 dark:text-gray-400">
                                      Notified: {formatDate(psc.notified_date)}
                                    </div>
                                  )}
                                  {psc.nature_of_control && psc.nature_of_control.length > 0 && (
                                    <div className="text-sm text-gray-500 dark:text-gray-400">
                                      Nature: {psc.nature_of_control[0]?.replace(/-/g, ' ')}
                                    </div>
                                  )}
                                </div>
                              ))}
                            </div>
                          )}

                          {/* Last Synced */}
                          {lastSynced && (
                            <p className="text-xs text-gray-400 dark:text-gray-500 mt-4">
                              Last synced: {lastSynced}
                            </p>
                          )}
                        </div>
                      )}
                    </>
                  )}
                </div>
              </>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
