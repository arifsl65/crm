'use client';

import { useEffect, useState, useCallback } from 'react';
import {
  Client,
  getClients,
  getServices,
  Service,
  ClientNote,
  getClientNotes,
  createClientNote,
  updateClientNote,
  deleteClientNote,
  searchCompaniesHouse,
  getCompanyFromCH,
  CHCompanySearchResult,
  CHCompanyProfile,
} from '@/lib/api';
import { usePanelContext } from '../components/DashboardShell';
import { AddClientPanel } from './components/AddClientPanel';
import { ByStaffView } from './components/ByStaffView';

interface ClientWithRisk extends Client {
  riskLevel?: 'high' | 'medium' | 'low' | 'none';
  overdueServices?: number;
  missingDocs?: number;
  isQuiet?: boolean;
}

export default function ClientsPage() {
  const { activePanel, setActivePanel } = usePanelContext();
  const [clients, setClients] = useState<ClientWithRisk[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState('');
  const [selectedClient, setSelectedClient] = useState<ClientWithRisk | null>(null);

  const fetchClients = useCallback(async () => {
    try {
      setLoading(true);
      const [clientsData, servicesData] = await Promise.all([
        getClients({
          search: search || undefined,
          limit: 100,
        }),
        getServices({ limit: 200 }),
      ]);

      const services = servicesData.services || [];
      const now = new Date();
      const fourteenDaysAgo = new Date(now.getTime() - 14 * 24 * 60 * 60 * 1000);

      // Calculate risk levels for each client
      const clientsWithRisk: ClientWithRisk[] = (clientsData.clients || []).map((client: Client) => {
        const clientServices = services.filter((s: Service) => s.client_id === client.id);

        const overdueServices = clientServices.filter((s: Service) => {
          if (!s.deadline || s.status === 'completed' || s.status === 'cancelled') return false;
          return new Date(s.deadline) < now;
        }).length;

        const missingDocs = clientServices.reduce((acc: number, s: Service) => {
          return acc + Math.max(0, (s.docs_required || 0) - (s.docs_received || 0));
        }, 0);

        // Check if client is "quiet" (no contact in 14+ days)
        const lastContact = client.last_contact_at ? new Date(client.last_contact_at) : null;
        const isQuiet = !lastContact || lastContact < fourteenDaysAgo;

        let riskLevel: 'high' | 'medium' | 'low' | 'none' = 'none';
        if (overdueServices > 1 || missingDocs > 5) riskLevel = 'high';
        else if (overdueServices > 0 || missingDocs > 2) riskLevel = 'medium';
        else if (missingDocs > 0) riskLevel = 'low';

        return {
          ...client,
          riskLevel,
          overdueServices,
          missingDocs,
          isQuiet,
        };
      });

      // Apply filters
      let filtered = clientsWithRisk;
      if (filter === 'at-risk') {
        filtered = clientsWithRisk.filter(c => c.riskLevel === 'high' || c.riskLevel === 'medium');
      } else if (filter === 'quiet') {
        filtered = clientsWithRisk.filter(c => c.isQuiet);
      } else if (filter === 'pending') {
        filtered = clientsWithRisk.filter(c => c.status === 'pending');
      } else if (filter === 'disabled') {
        filtered = clientsWithRisk.filter(c => c.status === 'inactive' || c.status === 'archived');
      }

      setClients(filtered);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load clients');
    } finally {
      setLoading(false);
    }
  }, [search, filter]);

  useEffect(() => {
    fetchClients();
  }, [fetchClients]);

  // Set default panel to 'list' when component mounts
  useEffect(() => {
    if (activePanel === 'today' || activePanel === 'overview') {
      setActivePanel('list');
    }
  }, [activePanel, setActivePanel]);

  const getStatusIcon = (client: ClientWithRisk) => {
    if (client.riskLevel === 'high') return '🔴';
    if (client.isQuiet) return '😶';
    if (client.status === 'pending') return '🟡';
    return '🟢';
  };

  const handleClientSelect = (client: ClientWithRisk) => {
    setSelectedClient(client);
  };

  const handleClosePanel = () => {
    setSelectedClient(null);
  };

  const handleClientCreated = () => {
    fetchClients();
    setActivePanel('list');
  };

  // Render based on active panel
  const renderMainContent = () => {
    switch (activePanel) {
      case 'add':
        return (
          <div className="w-full max-w-2xl mx-auto bg-white dark:bg-slate-800 h-full">
            <AddClientPanel
              onClose={() => setActivePanel('list')}
              onClientCreated={handleClientCreated}
            />
          </div>
        );

      case 'ch-lookup':
        return (
          <div className="w-full bg-white dark:bg-slate-800 h-full">
            <CompaniesHouseLookup
              onImport={(company: CHCompanySearchResult) => {
                // TODO: Pre-fill add client form with CH data
                // company.company_name, company.company_number, company.address_snippet
                console.log('Import company:', company);
                setActivePanel('add');
              }}
            />
          </div>
        );

      case 'by-staff':
        return (
          <div className="flex-1 flex overflow-hidden">
            {/* By Staff View */}
            <div className={`${selectedClient ? 'w-1/2' : 'flex-1'} overflow-y-auto bg-white dark:bg-slate-800`}>
              <ByStaffView
                onClientSelect={handleClientSelect}
                selectedClientId={selectedClient?.id}
              />
            </div>

            {/* Side Panel */}
            {selectedClient && (
              <div className="w-1/2 border-l border-gray-200 dark:border-gray-700 bg-white dark:bg-slate-800 overflow-y-auto">
                <ClientDetailPanel
                  client={selectedClient}
                  onClose={handleClosePanel}
                />
              </div>
            )}
          </div>
        );

      case 'list':
      default:
        return (
          <div className="flex-1 flex overflow-hidden">
            {/* Client List */}
            <div className={`${selectedClient ? 'w-1/2' : 'flex-1'} overflow-y-auto border-r border-gray-200 dark:border-gray-700`}>
              {error && (
                <div className="m-4 p-3 bg-red-50 dark:bg-red-900/30 rounded-lg text-red-700 dark:text-red-300 text-sm">
                  {error}
                </div>
              )}

              {loading ? (
                <div className="flex items-center justify-center h-64">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                </div>
              ) : clients.length === 0 ? (
                <div className="text-center py-12">
                  <span className="text-4xl">👥</span>
                  <p className="mt-2 text-gray-600 dark:text-gray-400">No clients found</p>
                  <button
                    onClick={() => setActivePanel('add')}
                    className="mt-4 inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700"
                  >
                    + Add Client
                  </button>
                </div>
              ) : (
                <div className="divide-y divide-gray-200 dark:divide-gray-700">
                  {clients.map((client) => (
                    <div
                      key={client.id}
                      onClick={() => handleClientSelect(client)}
                      className={`p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700 ${
                        selectedClient?.id === client.id ? 'bg-blue-50 dark:bg-blue-900/20' : ''
                      }`}
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex items-start gap-3 flex-1 min-w-0">
                          <span className="text-lg mt-0.5">{getStatusIcon(client)}</span>
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                              {client.company_name}
                            </p>
                            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                              {client.contact_name} • {client.email}
                            </p>
                          </div>
                        </div>

                        {/* Quick Actions */}
                        <div className="flex items-center gap-1 ml-2">
                          <button
                            onClick={(e) => { e.stopPropagation(); /* TODO: Email action */ }}
                            className="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-slate-600 rounded"
                            title="Send Email"
                          >
                            📧
                          </button>
                          <button
                            onClick={(e) => { e.stopPropagation(); /* TODO: Request docs */ }}
                            className="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-slate-600 rounded"
                            title="Request Documents"
                          >
                            📄
                          </button>
                          {(client.riskLevel === 'high' || client.riskLevel === 'medium' || client.isQuiet) && (
                            <button
                              onClick={(e) => { e.stopPropagation(); /* TODO: Chase */ }}
                              className="p-1.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-slate-600 rounded"
                              title="Chase"
                            >
                              🔔
                            </button>
                          )}
                        </div>
                      </div>

                      {/* Risk/Status Indicators */}
                      {((client.overdueServices ?? 0) > 0 || (client.missingDocs ?? 0) > 0) && (
                        <div className="mt-2 ml-8 flex gap-2">
                          {(client.overdueServices ?? 0) > 0 && (
                            <span className="text-xs text-red-600 dark:text-red-400">
                              {client.overdueServices} overdue
                            </span>
                          )}
                          {(client.missingDocs ?? 0) > 0 && (
                            <span className="text-xs text-orange-600 dark:text-orange-400">
                              {client.missingDocs} docs missing
                            </span>
                          )}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Side Panel */}
            {selectedClient && (
              <div className="w-1/2 bg-white dark:bg-slate-800 overflow-y-auto">
                <ClientDetailPanel
                  client={selectedClient}
                  onClose={handleClosePanel}
                />
              </div>
            )}
          </div>
        );
    }
  };

  return (
    <div className="h-full flex flex-col bg-gray-50 dark:bg-slate-900">
      {/* Header - only show for list and by-staff views */}
      {(activePanel === 'list' || activePanel === 'by-staff') && (
        <div className="bg-white dark:bg-slate-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-xl font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <span>👥</span> {activePanel === 'by-staff' ? 'Clients by Staff' : 'All Clients'}
              </h1>
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                {clients.length} clients {filter && `(${filter.replace('-', ' ')})`}
              </p>
            </div>
          </div>

          {/* Filters - only for list view */}
          {activePanel === 'list' && (
            <div className="mt-4 flex flex-wrap gap-3">
              <input
                type="text"
                placeholder="Search clients..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="flex-1 min-w-64 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
              />
              <select
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
              >
                <option value="">All Clients</option>
                <option value="at-risk">At Risk</option>
                <option value="quiet">Quiet (14+ days)</option>
                <option value="pending">Pending Onboarding</option>
                <option value="disabled">Disabled</option>
              </select>
            </div>
          )}
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-hidden">
        {renderMainContent()}
      </div>
    </div>
  );
}

function ClientDetailPanel({ client, onClose }: { client: ClientWithRisk; onClose: () => void }) {
  // Tabs: Info, Notes (Officers, Filings, PSC need backend endpoints)
  const [activeTab, setActiveTab] = useState<'info' | 'notes'>('info');
  const [notes, setNotes] = useState<ClientNote[]>([]);
  const [notesLoading, setNotesLoading] = useState(false);
  const [newNote, setNewNote] = useState('');
  const [editingNote, setEditingNote] = useState<{ id: string; text: string } | null>(null);
  const [showAddNote, setShowAddNote] = useState(false);

  const getStatusIcon = (c: ClientWithRisk) => {
    if (c.riskLevel === 'high') return '🔴';
    if (c.isQuiet) return '😶';
    if (c.status === 'pending') return '🟡';
    return '🟢';
  };

  const fetchNotes = useCallback(async () => {
    setNotesLoading(true);
    try {
      const data = await getClientNotes(client.id);
      setNotes(data.notes || []);
    } catch (err) {
      console.error('Failed to fetch notes:', err);
    } finally {
      setNotesLoading(false);
    }
  }, [client.id]);

  useEffect(() => {
    if (activeTab === 'notes') {
      fetchNotes();
    }
  }, [activeTab, fetchNotes]);

  const handleAddNote = async () => {
    if (!newNote.trim()) return;
    try {
      await createClientNote(client.id, newNote.trim());
      setNewNote('');
      setShowAddNote(false);
      fetchNotes();
    } catch (err) {
      console.error('Failed to add note:', err);
    }
  };

  const handleUpdateNote = async () => {
    if (!editingNote || !editingNote.text.trim()) return;
    try {
      await updateClientNote(client.id, editingNote.id, editingNote.text.trim());
      setEditingNote(null);
      fetchNotes();
    } catch (err) {
      console.error('Failed to update note:', err);
    }
  };

  const handleDeleteNote = async (noteId: string) => {
    if (!confirm('Delete this note?')) return;
    try {
      await deleteClientNote(client.id, noteId);
      fetchNotes();
    } catch (err) {
      console.error('Failed to delete note:', err);
    }
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffDays === 0) return 'Today ' + date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
    if (diffDays === 1) return 'Yesterday ' + date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
    return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="text-xl">{getStatusIcon(client)}</span>
          <div>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">{client.company_name}</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400">{client.contact_name}</p>
          </div>
        </div>
        <button
          onClick={onClose}
          className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        >
          ✕
        </button>
      </div>

      {/* Tabs - Info, Notes (Officers/Filings/PSC need backend) */}
      <div className="border-b border-gray-200 dark:border-gray-700 px-4">
        <div className="flex gap-4">
          {(['info', 'notes'] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`py-3 text-sm font-medium border-b-2 ${
                activeTab === tab
                  ? 'border-blue-600 text-blue-600 dark:text-blue-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400'
              }`}
            >
              {tab === 'notes' ? '📝 Notes' : 'Info'}
            </button>
          ))}
        </div>
      </div>

      {/* Tab Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {activeTab === 'info' && (
          <div className="space-y-4">
            {/* Risk Alert */}
            {client.riskLevel && client.riskLevel !== 'none' && (
              <div className={`p-3 rounded-lg ${
                client.riskLevel === 'high' ? 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800' :
                client.riskLevel === 'medium' ? 'bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800' :
                'bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800'
              }`}>
                <p className="text-sm font-medium">
                  {client.riskLevel === 'high' ? '🔴' : client.riskLevel === 'medium' ? '🟠' : '🟡'} Risk Alert
                </p>
                <ul className="text-xs mt-1 space-y-1">
                  {(client.overdueServices ?? 0) > 0 && (
                    <li>{client.overdueServices} overdue services</li>
                  )}
                  {(client.missingDocs ?? 0) > 0 && (
                    <li>{client.missingDocs} missing documents</li>
                  )}
                </ul>
              </div>
            )}

            {/* Quiet Alert */}
            {client.isQuiet && (
              <div className="p-3 rounded-lg bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600">
                <p className="text-sm font-medium">😶 Quiet Client</p>
                <p className="text-xs mt-1 text-gray-600 dark:text-gray-400">
                  No contact in 14+ days. Consider reaching out.
                </p>
              </div>
            )}

            {/* Contact Info */}
            <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
              <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Contact Information</h3>
              <div className="space-y-2 text-sm">
                <p><span className="text-gray-500">Email:</span> {client.email}</p>
                <p><span className="text-gray-500">Phone:</span> {client.phone || '-'}</p>
                <p><span className="text-gray-500">Address:</span> {client.address || '-'}</p>
              </div>
            </div>

            {/* Company Info */}
            <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
              <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Company Details</h3>
              <div className="space-y-2 text-sm">
                <p><span className="text-gray-500">Company Number:</span> {client.company_number || '-'}</p>
                <p><span className="text-gray-500">VAT Number:</span> {client.vat_number || '-'}</p>
                <p><span className="text-gray-500">Year End:</span> {client.year_end || '-'}</p>
                <p><span className="text-gray-500">VAT Quarter:</span> {client.vat_quarter || '-'}</p>
              </div>
            </div>

            {/* Quick Actions */}
            <div className="grid grid-cols-2 gap-2">
              <button className="p-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
                📧 Send Email
              </button>
              <button className="p-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
                📄 Request Docs
              </button>
              <button className="p-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
                📋 Add Service
              </button>
              <button className="p-3 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-slate-700 rounded-lg hover:bg-gray-200 dark:hover:bg-slate-600">
                🔔 Chase
              </button>
            </div>
          </div>
        )}

        {activeTab === 'notes' && (
          <div className="space-y-4">
            {/* Add Note Header */}
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">📝 Client Notes</h3>
              <button
                onClick={() => setShowAddNote(true)}
                className="px-3 py-1.5 text-xs font-medium text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30 rounded hover:bg-blue-100 dark:hover:bg-blue-900/50"
              >
                + Add Note
              </button>
            </div>

            {/* Add Note Form */}
            {showAddNote && (
              <div className="p-3 bg-gray-50 dark:bg-slate-700 rounded-lg border border-gray-200 dark:border-gray-600">
                <textarea
                  value={newNote}
                  onChange={(e) => setNewNote(e.target.value)}
                  placeholder="Write a note..."
                  rows={3}
                  className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-white resize-none"
                />
                <div className="flex justify-end gap-2 mt-2">
                  <button
                    onClick={() => { setShowAddNote(false); setNewNote(''); }}
                    className="px-3 py-1.5 text-xs font-medium text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleAddNote}
                    disabled={!newNote.trim()}
                    className="px-3 py-1.5 text-xs font-medium text-white bg-blue-600 rounded hover:bg-blue-700 disabled:opacity-50"
                  >
                    Save Note
                  </button>
                </div>
              </div>
            )}

            {/* Notes List */}
            {notesLoading ? (
              <div className="text-center py-8">
                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600 mx-auto"></div>
              </div>
            ) : notes.length === 0 ? (
              <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                <p>No notes yet</p>
                <p className="text-xs mt-1">Add a note to track important information</p>
              </div>
            ) : (
              <div className="space-y-3">
                {notes.map((note) => (
                  <div
                    key={note.id}
                    className="p-3 bg-gray-50 dark:bg-slate-700 rounded-lg border border-gray-200 dark:border-gray-600"
                  >
                    {editingNote?.id === note.id ? (
                      <div>
                        <textarea
                          value={editingNote.text}
                          onChange={(e) => setEditingNote({ ...editingNote, text: e.target.value })}
                          rows={3}
                          className="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-800 text-gray-900 dark:text-white resize-none"
                        />
                        <div className="flex justify-end gap-2 mt-2">
                          <button
                            onClick={() => setEditingNote(null)}
                            className="px-3 py-1.5 text-xs font-medium text-gray-600 dark:text-gray-400"
                          >
                            Cancel
                          </button>
                          <button
                            onClick={handleUpdateNote}
                            className="px-3 py-1.5 text-xs font-medium text-white bg-blue-600 rounded hover:bg-blue-700"
                          >
                            Save
                          </button>
                        </div>
                      </div>
                    ) : (
                      <>
                        <div className="flex items-start justify-between mb-2">
                          <div className="text-xs text-gray-500 dark:text-gray-400">
                            <span className="font-medium text-gray-700 dark:text-gray-300">{note.staff_name || 'Unknown'}</span>
                            <span className="mx-1">•</span>
                            <span>{formatDate(note.created_at)}</span>
                          </div>
                          <div className="flex gap-1">
                            <button
                              onClick={() => setEditingNote({ id: note.id, text: note.note })}
                              className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
                              title="Edit"
                            >
                              ✏️
                            </button>
                            <button
                              onClick={() => handleDeleteNote(note.id)}
                              className="p-1 text-gray-400 hover:text-red-500"
                              title="Delete"
                            >
                              🗑️
                            </button>
                          </div>
                        </div>
                        <p className="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap">{note.note}</p>
                      </>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function CompaniesHouseLookup({ onImport }: { onImport: (company: CHCompanySearchResult) => void }) {
  const [searchQuery, setSearchQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<CHCompanySearchResult[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [totalResults, setTotalResults] = useState(0);
  const [selectedCompany, setSelectedCompany] = useState<CHCompanyProfile | null>(null);
  const [loadingDetails, setLoadingDetails] = useState(false);
  const [detailsError, setDetailsError] = useState<string | null>(null);

  const handleViewDetails = async (companyNumber: string) => {
    console.log('handleViewDetails called with:', companyNumber);
    setLoadingDetails(true);
    setDetailsError(null);
    try {
      console.log('Fetching company details...');
      const details = await getCompanyFromCH(companyNumber);
      console.log('Got company details:', details);
      setSelectedCompany(details);
    } catch (err) {
      console.error('Error fetching company details:', err);
      setDetailsError(err instanceof Error ? err.message : 'Failed to load company details');
    } finally {
      setLoadingDetails(false);
    }
  };

  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    setSearching(true);
    setError(null);
    try {
      const data = await searchCompaniesHouse(searchQuery.trim());
      setResults(data.items || []);
      setTotalResults(data.total_results || 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      setResults([]);
    } finally {
      setSearching(false);
    }
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>🔍</span> Companies House Lookup
        </h2>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
          Search for a company to import client details
        </p>
      </div>

      {/* Search */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700">
        <div className="flex gap-2">
          <input
            type="text"
            placeholder="Company name or number..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            className="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white"
          />
          <button
            onClick={handleSearch}
            disabled={searching}
            className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50"
          >
            {searching ? 'Searching...' : 'Search'}
          </button>
        </div>
      </div>

      {/* Results */}
      <div className="flex-1 overflow-y-auto p-4">
        {error && (
          <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 rounded-lg text-red-700 dark:text-red-300 text-sm">
            {error}
          </div>
        )}

        {results.length === 0 && !error ? (
          <div className="text-center py-8">
            <span className="text-4xl">🏢</span>
            <p className="mt-2 text-gray-600 dark:text-gray-400">
              {searching ? 'Searching Companies House...' : 'Search for a company to see results'}
            </p>
          </div>
        ) : results.length > 0 && (
          <>
            <p className="text-xs text-gray-500 dark:text-gray-400 mb-3">
              Found {totalResults} result{totalResults !== 1 ? 's' : ''} • Showing {results.length}
            </p>
            <div className="space-y-3">
              {results.map((company) => (
                <div
                  key={company.company_number}
                  className="p-4 bg-gray-50 dark:bg-slate-700 rounded-lg border border-gray-200 dark:border-gray-600"
                >
                  <div className="flex items-start justify-between">
                    <div>
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {company.company_name}
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        {company.company_number} • {company.company_type}{company.date_of_creation && ` • Est. ${company.date_of_creation}`}
                      </p>
                      {company.address_snippet && (
                        <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                          {company.address_snippet}
                        </p>
                      )}
                    </div>
                    <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${
                      company.company_status === 'active'
                        ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
                        : company.company_status === 'dissolved'
                        ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                    }`}>
                      {company.company_status}
                    </span>
                  </div>
                  <div className="mt-3 flex gap-2">
                    <button
                      onClick={() => onImport(company)}
                      className="px-3 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-900/30 rounded hover:bg-blue-200 dark:hover:bg-blue-900/50"
                    >
                      Import as Client
                    </button>
                    <button
                      onClick={() => handleViewDetails(company.company_number)}
                      disabled={loadingDetails}
                      className="px-3 py-1 text-xs font-medium text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-slate-600 rounded hover:bg-gray-200 dark:hover:bg-slate-500 disabled:opacity-50"
                    >
                      {loadingDetails ? 'Loading...' : 'View Details'}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </div>

      {/* Company Details Modal */}
      {(selectedCompany || loadingDetails || detailsError) && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[80vh] overflow-hidden flex flex-col">
            {/* Modal Header */}
            <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <span>🏢</span> Company Details
              </h3>
              <button
                onClick={() => {
                  setSelectedCompany(null);
                  setDetailsError(null);
                }}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 text-xl"
              >
                ✕
              </button>
            </div>

            {/* Modal Body */}
            <div className="flex-1 overflow-y-auto p-4">
              {loadingDetails && (
                <div className="flex items-center justify-center py-12">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                </div>
              )}

              {detailsError && (
                <div className="p-4 bg-red-50 dark:bg-red-900/30 rounded-lg text-red-700 dark:text-red-300">
                  {detailsError}
                </div>
              )}

              {selectedCompany && !loadingDetails && (
                <div className="space-y-6">
                  {/* Company Header */}
                  <div className="flex items-start justify-between">
                    <div>
                      <h4 className="text-xl font-bold text-gray-900 dark:text-white">
                        {selectedCompany.company_name}
                      </h4>
                      <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                        {selectedCompany.company_number} • {selectedCompany.company_type}
                      </p>
                    </div>
                    <span className={`px-3 py-1 text-sm font-medium rounded-full ${
                      selectedCompany.company_status === 'active'
                        ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
                        : selectedCompany.company_status === 'dissolved'
                        ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300'
                        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                    }`}>
                      {selectedCompany.company_status}
                    </span>
                  </div>

                  {/* Basic Info */}
                  <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
                    <h5 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Basic Information</h5>
                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Incorporated:</span>
                        <p className="font-medium text-gray-900 dark:text-white">{selectedCompany.date_of_creation || '-'}</p>
                      </div>
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Company Type:</span>
                        <p className="font-medium text-gray-900 dark:text-white">{selectedCompany.company_type || '-'}</p>
                      </div>
                    </div>
                  </div>

                  {/* Registered Address */}
                  {selectedCompany.registered_office_address && (
                    <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
                      <h5 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Registered Office Address</h5>
                      <p className="text-sm text-gray-900 dark:text-white">
                        {[
                          selectedCompany.registered_office_address.address_line_1,
                          selectedCompany.registered_office_address.address_line_2,
                          selectedCompany.registered_office_address.locality,
                          selectedCompany.registered_office_address.region,
                          selectedCompany.registered_office_address.postal_code,
                          selectedCompany.registered_office_address.country,
                        ].filter(Boolean).join(', ')}
                      </p>
                    </div>
                  )}

                  {/* SIC Codes */}
                  {selectedCompany.sic_codes && selectedCompany.sic_codes.length > 0 && (
                    <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
                      <h5 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">SIC Codes (Nature of Business)</h5>
                      <div className="flex flex-wrap gap-2">
                        {selectedCompany.sic_codes.map((code) => (
                          <span key={code} className="px-2 py-1 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 text-xs rounded">
                            {code}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Accounts Information */}
                  {selectedCompany.accounts && (
                    <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
                      <h5 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Accounts</h5>
                      <div className="grid grid-cols-2 gap-4 text-sm">
                        {selectedCompany.accounts.accounting_reference_date && (
                          <div>
                            <span className="text-gray-500 dark:text-gray-400">Year End:</span>
                            <p className="font-medium text-gray-900 dark:text-white">
                              {selectedCompany.accounts.accounting_reference_date.day}/{selectedCompany.accounts.accounting_reference_date.month}
                            </p>
                          </div>
                        )}
                        {selectedCompany.accounts.last_accounts?.made_up_to && (
                          <div>
                            <span className="text-gray-500 dark:text-gray-400">Last Accounts:</span>
                            <p className="font-medium text-gray-900 dark:text-white">{selectedCompany.accounts.last_accounts.made_up_to}</p>
                          </div>
                        )}
                        {selectedCompany.accounts.next_due && (
                          <div>
                            <span className="text-gray-500 dark:text-gray-400">Next Due:</span>
                            <p className="font-medium text-gray-900 dark:text-white">{selectedCompany.accounts.next_due}</p>
                          </div>
                        )}
                        {selectedCompany.accounts.last_accounts?.type && (
                          <div>
                            <span className="text-gray-500 dark:text-gray-400">Account Type:</span>
                            <p className="font-medium text-gray-900 dark:text-white">{selectedCompany.accounts.last_accounts.type}</p>
                          </div>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Confirmation Statement */}
                  {selectedCompany.confirmation_statement && (
                    <div className="bg-gray-50 dark:bg-slate-700 rounded-lg p-4">
                      <h5 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Confirmation Statement</h5>
                      <div className="grid grid-cols-2 gap-4 text-sm">
                        {selectedCompany.confirmation_statement.last_made_up_to && (
                          <div>
                            <span className="text-gray-500 dark:text-gray-400">Last Filed:</span>
                            <p className="font-medium text-gray-900 dark:text-white">{selectedCompany.confirmation_statement.last_made_up_to}</p>
                          </div>
                        )}
                        {selectedCompany.confirmation_statement.next_due && (
                          <div>
                            <span className="text-gray-500 dark:text-gray-400">Next Due:</span>
                            <p className="font-medium text-gray-900 dark:text-white">{selectedCompany.confirmation_statement.next_due}</p>
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Modal Footer */}
            {selectedCompany && (
              <div className="p-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-2">
                <button
                  onClick={() => {
                    setSelectedCompany(null);
                    setDetailsError(null);
                  }}
                  className="px-4 py-2 text-sm font-medium text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200"
                >
                  Close
                </button>
                <button
                  onClick={() => {
                    onImport({
                      company_number: selectedCompany.company_number,
                      company_name: selectedCompany.company_name,
                      company_status: selectedCompany.company_status,
                      company_type: selectedCompany.company_type,
                      date_of_creation: selectedCompany.date_of_creation,
                      registered_office_address: selectedCompany.registered_office_address,
                      address_snippet: selectedCompany.registered_office_address
                        ? [
                            selectedCompany.registered_office_address.address_line_1,
                            selectedCompany.registered_office_address.locality,
                            selectedCompany.registered_office_address.postal_code,
                          ].filter(Boolean).join(', ')
                        : undefined,
                    });
                    setSelectedCompany(null);
                  }}
                  className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700"
                >
                  Import as Client
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
