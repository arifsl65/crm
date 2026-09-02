'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import {
  EmailAccount,
  getEmailAccounts,
  createIMAPAccount,
  deleteEmailAccount,
  syncEmailAccount,
  testEmailAccountConnection,
  disconnectEmailAccount,
  reconnectEmailAccount,
} from '@/lib/api';
import { SkeletonTable, useToast } from '@/components';

export default function EmailSettingsPage() {
  const { user } = useAuth();
  const toast = useToast();
  const [accounts, setAccounts] = useState<EmailAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showAddModal, setShowAddModal] = useState(false);
  const [syncingId, setSyncingId] = useState<string | null>(null);
  const [testingId, setTestingId] = useState<string | null>(null);

  const canManage = user?.role === 'super_admin' || user?.role === 'tenant_admin';

  const fetchAccounts = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getEmailAccounts({ limit: 50 });
      setAccounts(data.email_accounts || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load email accounts');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAccounts();
  }, [fetchAccounts]);

  const handleSync = async (id: string) => {
    setSyncingId(id);
    try {
      await syncEmailAccount(id);
      toast.success('Sync initiated');
      fetchAccounts();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to sync');
    } finally {
      setSyncingId(null);
    }
  };

  const handleTest = async (id: string) => {
    setTestingId(id);
    try {
      const result = await testEmailAccountConnection(id);
      if (result.success) {
        toast.success('Connection successful');
      } else {
        toast.error(result.message || 'Connection failed');
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Connection test failed');
    } finally {
      setTestingId(null);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to remove this email account?')) return;

    try {
      await deleteEmailAccount(id);
      setAccounts((prev) => prev.filter((a) => a.id !== id));
      toast.success('Email account removed');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to remove account');
    }
  };

  const handleToggleConnection = async (account: EmailAccount) => {
    try {
      if (account.status === 'disconnected') {
        await reconnectEmailAccount(account.id);
        toast.success('Account reconnected');
      } else {
        await disconnectEmailAccount(account.id);
        toast.success('Account disconnected');
      }
      fetchAccounts();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update account');
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
      case 'error':
        return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200';
      case 'disconnected':
        return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200';
      default:
        return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200';
    }
  };

  const getProviderIcon = (provider: string) => {
    switch (provider) {
      case 'google':
        return '📧';
      case 'microsoft':
        return '📨';
      case 'zoho':
        return '✉️';
      default:
        return '📬';
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
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Email Settings</h1>
            </div>
            {canManage && (
              <button
                onClick={() => setShowAddModal(true)}
                className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
              >
                <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                </svg>
                Add Account
              </button>
            )}
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Error */}
        {error && (
          <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
            <p className="text-red-700 dark:text-red-300">{error}</p>
          </div>
        )}

        {/* Email Accounts Section */}
        <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden mb-8">
          <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">Connected Email Accounts</h2>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Manage email accounts connected to your organization
            </p>
          </div>

          {loading ? (
            <div className="p-6">
              <SkeletonTable rows={3} columns={5} />
            </div>
          ) : accounts.length === 0 ? (
            <div className="p-12 text-center">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
              <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white">No email accounts</h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Connect an email account to start sending and receiving emails.
              </p>
              {canManage && (
                <button
                  onClick={() => setShowAddModal(true)}
                  className="mt-4 inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
                >
                  Add Email Account
                </button>
              )}
            </div>
          ) : (
            <div className="divide-y divide-gray-200 dark:divide-gray-700">
              {accounts.map((account) => (
                <div key={account.id} className="p-6">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-4">
                      <span className="text-2xl">{getProviderIcon(account.provider)}</span>
                      <div>
                        <div className="flex items-center gap-2">
                          <h3 className="text-sm font-medium text-gray-900 dark:text-white">
                            {account.email}
                          </h3>
                          <span className={`px-2 py-0.5 text-xs font-medium rounded-full ${getStatusBadge(account.status)}`}>
                            {account.status}
                          </span>
                          <span className="px-2 py-0.5 text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200 rounded-full">
                            {account.type}
                          </span>
                        </div>
                        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                          {account.provider.charAt(0).toUpperCase() + account.provider.slice(1)} • {account.auth_method.toUpperCase()}
                          {account.last_sync_at && (
                            <> • Last synced: {new Date(account.last_sync_at).toLocaleString()}</>
                          )}
                        </p>
                        {account.error_message && (
                          <p className="text-sm text-red-600 dark:text-red-400 mt-1">
                            Error: {account.error_message}
                          </p>
                        )}
                      </div>
                    </div>
                    {canManage && (
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => handleTest(account.id)}
                          disabled={testingId === account.id}
                          className="px-3 py-1.5 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700 rounded transition-colors disabled:opacity-50"
                        >
                          {testingId === account.id ? 'Testing...' : 'Test'}
                        </button>
                        <button
                          onClick={() => handleSync(account.id)}
                          disabled={syncingId === account.id || account.status === 'disconnected'}
                          className="px-3 py-1.5 text-sm text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded transition-colors disabled:opacity-50"
                        >
                          {syncingId === account.id ? 'Syncing...' : 'Sync'}
                        </button>
                        <button
                          onClick={() => handleToggleConnection(account)}
                          className={`px-3 py-1.5 text-sm rounded transition-colors ${
                            account.status === 'disconnected'
                              ? 'text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20'
                              : 'text-orange-600 hover:bg-orange-50 dark:hover:bg-orange-900/20'
                          }`}
                        >
                          {account.status === 'disconnected' ? 'Reconnect' : 'Disconnect'}
                        </button>
                        <button
                          onClick={() => handleDelete(account.id)}
                          className="px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors"
                        >
                          Remove
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* OAuth Integration Section */}
        <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">Connect Email Provider</h2>
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              Connect your email provider for seamless integration
            </p>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <button
                disabled
                className="flex items-center justify-center gap-3 p-4 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg hover:border-gray-400 dark:hover:border-gray-500 transition-colors opacity-50 cursor-not-allowed"
              >
                <span className="text-2xl">📧</span>
                <div className="text-left">
                  <p className="text-sm font-medium text-gray-900 dark:text-white">Gmail / Google Workspace</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Coming soon</p>
                </div>
              </button>
              <button
                disabled
                className="flex items-center justify-center gap-3 p-4 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg hover:border-gray-400 dark:hover:border-gray-500 transition-colors opacity-50 cursor-not-allowed"
              >
                <span className="text-2xl">📨</span>
                <div className="text-left">
                  <p className="text-sm font-medium text-gray-900 dark:text-white">Microsoft 365 / Outlook</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Coming soon</p>
                </div>
              </button>
              <button
                onClick={() => setShowAddModal(true)}
                className="flex items-center justify-center gap-3 p-4 border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg hover:border-blue-400 dark:hover:border-blue-500 transition-colors"
              >
                <span className="text-2xl">📬</span>
                <div className="text-left">
                  <p className="text-sm font-medium text-gray-900 dark:text-white">IMAP / SMTP</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400">Connect via IMAP</p>
                </div>
              </button>
            </div>
          </div>
        </div>
      </main>

      {/* Add Account Modal */}
      {showAddModal && (
        <AddAccountModal
          onClose={() => setShowAddModal(false)}
          onAdded={() => {
            setShowAddModal(false);
            fetchAccounts();
            toast.success('Email account added');
          }}
        />
      )}
    </div>
  );
}

// Add Account Modal Component
function AddAccountModal({
  onClose,
  onAdded,
}: {
  onClose: () => void;
  onAdded: () => void;
}) {
  const [email, setEmail] = useState('');
  const [imapHost, setImapHost] = useState('');
  const [imapPort, setImapPort] = useState('993');
  const [password, setPassword] = useState('');
  const [accountType, setAccountType] = useState<'shared' | 'personal'>('shared');
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!email || !imapHost || !password) {
      alert('Please fill in all required fields');
      return;
    }

    setSaving(true);
    try {
      await createIMAPAccount({
        email,
        imap_host: imapHost,
        imap_port: parseInt(imapPort, 10),
        imap_password: password,
        type: accountType,
      });
      onAdded();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to add email account');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-md">
        {/* Header */}
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            Add IMAP Account
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
        <div className="p-4">
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Email Address *
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                placeholder="your@email.com"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                IMAP Server *
              </label>
              <input
                type="text"
                value={imapHost}
                onChange={(e) => setImapHost(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                placeholder="imap.gmail.com"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                IMAP Port
              </label>
              <input
                type="number"
                value={imapPort}
                onChange={(e) => setImapPort(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                placeholder="993"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Password *
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                placeholder="App password or email password"
              />
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                For Gmail, use an App Password instead of your regular password
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Account Type
              </label>
              <select
                value={accountType}
                onChange={(e) => setAccountType(e.target.value as 'shared' | 'personal')}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
              >
                <option value="shared">Shared (visible to all team members)</option>
                <option value="personal">Personal (only visible to you)</option>
              </select>
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
            {saving ? 'Adding...' : 'Add Account'}
          </button>
        </div>
      </div>
    </div>
  );
}
