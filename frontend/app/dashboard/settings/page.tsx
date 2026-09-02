'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import { CompanySettings, getSettings, updateSettings } from '@/lib/api';
import { useToast } from '@/components';

export default function SettingsPage() {
  const { user } = useAuth();
  const toast = useToast();
  const [settings, setSettings] = useState<CompanySettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Form state
  const [firmName, setFirmName] = useState('');
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [address, setAddress] = useState('');
  const [logoUrl, setLogoUrl] = useState('');
  const [reminderDay3, setReminderDay3] = useState(true);
  const [reminderDay7, setReminderDay7] = useState(true);
  const [reminderDay14, setReminderDay14] = useState(true);

  const isAdmin = user?.role === 'super_admin' || user?.role === 'tenant_admin';

  useEffect(() => {
    fetchSettings();
  }, []);

  const fetchSettings = async () => {
    try {
      setLoading(true);
      const data = await getSettings();
      setSettings(data.settings);
      // Populate form
      setFirmName(data.settings.firm_name || '');
      setEmail(data.settings.email || '');
      setPhone(data.settings.phone || '');
      setAddress(data.settings.address || '');
      setLogoUrl(data.settings.logo_url || '');
      setReminderDay3(data.settings.reminder_rules?.day3 ?? true);
      setReminderDay7(data.settings.reminder_rules?.day7 ?? true);
      setReminderDay14(data.settings.reminder_rules?.day14 ?? true);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load settings');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!isAdmin) return;

    try {
      setSaving(true);
      await updateSettings({
        firm_name: firmName,
        email,
        phone: phone || undefined,
        address: address || undefined,
        logo_url: logoUrl || undefined,
        reminder_rules: {
          day3: reminderDay3,
          day7: reminderDay7,
          day14: reminderDay14,
        },
      });
      toast.success('Settings saved successfully');
      fetchSettings();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
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
              href="/dashboard"
              className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
              </svg>
            </Link>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Settings</h1>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Navigation Tabs */}
        <div className="mb-8 border-b border-gray-200 dark:border-gray-700">
          <nav className="-mb-px flex space-x-8">
            <span className="border-b-2 border-blue-500 py-4 px-1 text-sm font-medium text-blue-600 dark:text-blue-400">
              General
            </span>
            <Link
              href="/dashboard/settings/security"
              className="border-b-2 border-transparent py-4 px-1 text-sm font-medium text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 dark:hover:text-gray-300"
            >
              Security
            </Link>
          </nav>
        </div>

        {/* Error */}
        {error && (
          <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
            <p className="text-red-700 dark:text-red-300">{error}</p>
          </div>
        )}

        <form onSubmit={handleSave} className="space-y-8">
          {/* Company Information */}
          <div className="bg-white dark:bg-slate-800 shadow rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-6">Company Information</h2>
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <div>
                <label htmlFor="firmName" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Firm Name
                </label>
                <input
                  id="firmName"
                  type="text"
                  value={firmName}
                  onChange={(e) => setFirmName(e.target.value)}
                  disabled={!isAdmin}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50 focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
              <div>
                <label htmlFor="email" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Contact Email
                </label>
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={!isAdmin}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50 focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
              <div>
                <label htmlFor="phone" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Phone Number
                </label>
                <input
                  id="phone"
                  type="tel"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  disabled={!isAdmin}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50 focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
              <div>
                <label htmlFor="logoUrl" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Logo URL
                </label>
                <input
                  id="logoUrl"
                  type="url"
                  value={logoUrl}
                  onChange={(e) => setLogoUrl(e.target.value)}
                  disabled={!isAdmin}
                  placeholder="https://..."
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50 focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
              <div className="sm:col-span-2">
                <label htmlFor="address" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                  Address
                </label>
                <textarea
                  id="address"
                  value={address}
                  onChange={(e) => setAddress(e.target.value)}
                  disabled={!isAdmin}
                  rows={3}
                  className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-slate-700 text-gray-900 dark:text-white disabled:opacity-50 focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
            </div>
          </div>

          {/* Reminder Settings */}
          <div className="bg-white dark:bg-slate-800 shadow rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-6">Document Reminder Rules</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              Configure automatic reminders for pending document requests.
            </p>
            <div className="space-y-4">
              <label className="flex items-center">
                <input
                  type="checkbox"
                  checked={reminderDay3}
                  onChange={(e) => setReminderDay3(e.target.checked)}
                  disabled={!isAdmin}
                  className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded disabled:opacity-50"
                />
                <span className="ml-3 text-sm text-gray-700 dark:text-gray-300">
                  Send reminder after 3 days
                </span>
              </label>
              <label className="flex items-center">
                <input
                  type="checkbox"
                  checked={reminderDay7}
                  onChange={(e) => setReminderDay7(e.target.checked)}
                  disabled={!isAdmin}
                  className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded disabled:opacity-50"
                />
                <span className="ml-3 text-sm text-gray-700 dark:text-gray-300">
                  Send reminder after 7 days
                </span>
              </label>
              <label className="flex items-center">
                <input
                  type="checkbox"
                  checked={reminderDay14}
                  onChange={(e) => setReminderDay14(e.target.checked)}
                  disabled={!isAdmin}
                  className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded disabled:opacity-50"
                />
                <span className="ml-3 text-sm text-gray-700 dark:text-gray-300">
                  Send reminder after 14 days
                </span>
              </label>
            </div>
          </div>

          {/* Integration Status */}
          <div className="bg-white dark:bg-slate-800 shadow rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900 dark:text-white mb-6">Integrations</h2>
            <div className="space-y-4">
              <div className="flex items-center justify-between p-4 bg-gray-50 dark:bg-slate-700 rounded-lg">
                <div className="flex items-center space-x-3">
                  <div className="w-10 h-10 bg-purple-100 dark:bg-purple-900/30 rounded-lg flex items-center justify-center">
                    <svg className="w-6 h-6 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
                    </svg>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-gray-900 dark:text-white">Stripe</p>
                    <p className="text-xs text-gray-500 dark:text-gray-400">Payment processing</p>
                  </div>
                </div>
                <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                  settings?.stripe_connected
                    ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
                    : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
                }`}>
                  {settings?.stripe_connected ? 'Connected' : 'Not Connected'}
                </span>
              </div>
            </div>
          </div>

          {/* Save Button */}
          {isAdmin && (
            <div className="flex justify-end">
              <button
                type="submit"
                disabled={saving}
                className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50"
              >
                {saving ? (
                  <>
                    <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Saving...
                  </>
                ) : (
                  'Save Changes'
                )}
              </button>
            </div>
          )}
        </form>

        {/* Last Updated */}
        {settings?.updated_at && (
          <p className="mt-4 text-sm text-gray-500 dark:text-gray-400 text-center">
            Last updated: {new Date(settings.updated_at).toLocaleString()}
          </p>
        )}
      </main>
    </div>
  );
}
