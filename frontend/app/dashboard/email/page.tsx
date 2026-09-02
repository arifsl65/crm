'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { useAuth } from '@/components/auth-guard';
import {
  Email,
  EmailStats,
  getEmails,
  getEmailStats,
  markEmailRead,
} from '@/lib/api';
import { SkeletonTable, useToast } from '@/components';

type Tab = 'inbox' | 'sent' | 'compose' | 'templates';
type Filter = 'all' | 'unread' | 'action' | 'attachment' | 'urgent';

export default function EmailPage() {
  const { user } = useAuth();
  const toast = useToast();
  const [emails, setEmails] = useState<Email[]>([]);
  const [stats, setStats] = useState<EmailStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [activeTab, setActiveTab] = useState<Tab>('inbox');
  const [filter, setFilter] = useState<Filter>('all');
  const [selectedEmail, setSelectedEmail] = useState<Email | null>(null);
  const [showCompose, setShowCompose] = useState(false);

  const fetchEmails = useCallback(async () => {
    try {
      setLoading(true);
      const direction = activeTab === 'sent' ? 'outbound' : 'inbound';
      const data = await getEmails({
        direction: activeTab === 'inbox' || activeTab === 'sent' ? direction : undefined,
        search: search || undefined,
        limit: 50,
      });
      setEmails(data.emails || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load emails');
    } finally {
      setLoading(false);
    }
  }, [search, activeTab]);

  const fetchStats = useCallback(async () => {
    try {
      const data = await getEmailStats();
      setStats(data.stats);
    } catch (err) {
      console.error('Failed to fetch email stats:', err);
    }
  }, []);

  useEffect(() => {
    if (activeTab === 'inbox' || activeTab === 'sent') {
      fetchEmails();
    }
  }, [fetchEmails, activeTab]);

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  const handleEmailClick = async (email: Email) => {
    setSelectedEmail(email);
    if (!email.is_read) {
      try {
        await markEmailRead(email.id);
        setEmails((prev) =>
          prev.map((e) => (e.id === email.id ? { ...e, is_read: true } : e))
        );
        if (stats) {
          setStats({ ...stats, unread: Math.max(0, stats.unread - 1) });
        }
      } catch (err) {
        console.error('Failed to mark email as read:', err);
      }
    }
  };

  const getSentimentIcon = (sentiment?: string) => {
    switch (sentiment?.toLowerCase()) {
      case 'positive':
        return { icon: '😊', color: 'text-green-500' };
      case 'negative':
      case 'frustrated':
        return { icon: '😤', color: 'text-red-500' };
      default:
        return { icon: '😐', color: 'text-gray-500' };
    }
  };

  const getStatusTag = (email: Email) => {
    const tags = [];

    // AI-detected tags based on content
    if (email.ai_summary?.toLowerCase().includes('action')) {
      tags.push({ label: 'Action', icon: '📎', color: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' });
    }
    if (email.ai_summary?.toLowerCase().includes('urgent') || email.subject.toLowerCase().includes('urgent')) {
      tags.push({ label: 'Urgent', icon: '🚨', color: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' });
    }
    if (email.ai_summary?.toLowerCase().includes('query') || email.ai_summary?.toLowerCase().includes('question')) {
      tags.push({ label: 'Query', icon: '❓', color: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200' });
    }
    if (!email.client_id) {
      tags.push({ label: 'Unknown', icon: '🆕', color: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200' });
    }

    return tags;
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const days = Math.floor(diff / (1000 * 60 * 60 * 24));

    if (days === 0) {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } else if (days === 1) {
      return 'Yesterday';
    } else if (days < 7) {
      return date.toLocaleDateString([], { weekday: 'short' });
    } else {
      return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
    }
  };

  const filteredEmails = emails.filter((email) => {
    if (filter === 'all') return true;
    if (filter === 'unread') return !email.is_read;
    if (filter === 'action') return email.ai_summary?.toLowerCase().includes('action');
    if (filter === 'urgent') return email.subject.toLowerCase().includes('urgent');
    return true;
  });

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-slate-900">
      {/* Header */}
      <header className="bg-white dark:bg-slate-800 shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex justify-between items-center">
            <div className="flex items-center space-x-4">
              <Link
                href="/dashboard"
                className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
              </Link>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Email</h1>
              {stats && stats.unread > 0 && (
                <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                  {stats.unread} unread
                </span>
              )}
            </div>
            <button
              onClick={() => setShowCompose(true)}
              className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
            >
              <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Compose
            </button>
          </div>

          {/* Tabs */}
          <div className="mt-4 border-b border-gray-200 dark:border-gray-700">
            <nav className="-mb-px flex space-x-8">
              <button
                onClick={() => setActiveTab('inbox')}
                className={`py-2 px-1 border-b-2 font-medium text-sm ${
                  activeTab === 'inbox'
                    ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400'
                }`}
              >
                <span className="flex items-center">
                  <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                  </svg>
                  Inbox
                  {stats && stats.unread > 0 && (
                    <span className="ml-2 bg-blue-100 text-blue-600 dark:bg-blue-900 dark:text-blue-300 text-xs px-2 py-0.5 rounded-full">
                      {stats.unread}
                    </span>
                  )}
                </span>
              </button>
              <button
                onClick={() => setActiveTab('sent')}
                className={`py-2 px-1 border-b-2 font-medium text-sm ${
                  activeTab === 'sent'
                    ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400'
                }`}
              >
                <span className="flex items-center">
                  <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                  </svg>
                  Sent
                  {stats && (
                    <span className="ml-2 text-gray-400 text-xs">
                      {stats.sent}
                    </span>
                  )}
                </span>
              </button>
              <Link
                href="/dashboard/email/templates"
                className="py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 font-medium text-sm"
              >
                <span className="flex items-center">
                  <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                  Templates
                </span>
              </Link>
              <Link
                href="/dashboard/email/settings"
                className="py-2 px-1 border-b-2 border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300 dark:text-gray-400 font-medium text-sm"
              >
                <span className="flex items-center">
                  <svg className="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                  Settings
                </span>
              </Link>
            </nav>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <div className="flex gap-6">
          {/* Email List */}
          <div className={`${selectedEmail ? 'w-2/5' : 'w-full'} transition-all duration-300`}>
            {/* Search and Filters */}
            <div className="mb-4 flex flex-col sm:flex-row gap-3">
              <div className="flex-1 relative">
                <svg className="w-5 h-5 absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                <input
                  type="text"
                  placeholder="Search emails..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div className="flex gap-2">
                {(['all', 'unread', 'action', 'urgent'] as Filter[]).map((f) => (
                  <button
                    key={f}
                    onClick={() => setFilter(f)}
                    className={`px-3 py-2 text-sm rounded-md transition-colors ${
                      filter === f
                        ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                        : 'bg-white dark:bg-slate-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-600'
                    }`}
                  >
                    {f.charAt(0).toUpperCase() + f.slice(1)}
                  </button>
                ))}
              </div>
            </div>

            {/* Error */}
            {error && (
              <div className="mb-4 p-4 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-md">
                <p className="text-red-700 dark:text-red-300">{error}</p>
              </div>
            )}

            {/* Email List */}
            <div className="bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
              {loading ? (
                <div className="p-6">
                  <SkeletonTable rows={8} columns={3} />
                </div>
              ) : filteredEmails.length === 0 ? (
                <div className="p-12 text-center">
                  <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                  </svg>
                  <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white">No emails</h3>
                  <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                    {activeTab === 'inbox' ? 'Your inbox is empty.' : 'No sent emails yet.'}
                  </p>
                </div>
              ) : (
                <div className="divide-y divide-gray-200 dark:divide-gray-700">
                  {filteredEmails.map((email) => {
                    const sentiment = getSentimentIcon(email.sentiment);
                    const tags = getStatusTag(email);

                    return (
                      <div
                        key={email.id}
                        onClick={() => handleEmailClick(email)}
                        className={`p-4 cursor-pointer transition-colors ${
                          selectedEmail?.id === email.id
                            ? 'bg-blue-50 dark:bg-blue-900/20'
                            : 'hover:bg-gray-50 dark:hover:bg-slate-700'
                        } ${!email.is_read ? 'bg-blue-50/50 dark:bg-blue-900/10' : ''}`}
                      >
                        <div className="flex items-start justify-between">
                          <div className="flex items-start space-x-3 flex-1 min-w-0">
                            {/* Read/Unread indicator */}
                            <div className={`w-2 h-2 mt-2 rounded-full flex-shrink-0 ${
                              email.is_read ? 'bg-transparent' : 'bg-blue-500'
                            }`} />

                            <div className="flex-1 min-w-0">
                              {/* Sender and Date */}
                              <div className="flex items-center justify-between mb-1">
                                <span className={`text-sm truncate ${
                                  email.is_read
                                    ? 'text-gray-700 dark:text-gray-300'
                                    : 'font-semibold text-gray-900 dark:text-white'
                                }`}>
                                  {email.direction === 'inbound' ? email.from_email : email.to_email}
                                </span>
                                <span className="text-xs text-gray-500 dark:text-gray-400 flex-shrink-0 ml-2">
                                  {formatDate(email.created_at)}
                                </span>
                              </div>

                              {/* Subject */}
                              <p className={`text-sm truncate ${
                                email.is_read
                                  ? 'text-gray-600 dark:text-gray-400'
                                  : 'font-medium text-gray-900 dark:text-white'
                              }`}>
                                {email.subject}
                              </p>

                              {/* AI Summary */}
                              {email.ai_summary && (
                                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 truncate">
                                  🤖 {email.ai_summary}
                                </p>
                              )}

                              {/* Tags */}
                              <div className="flex items-center gap-2 mt-2 flex-wrap">
                                {tags.map((tag, idx) => (
                                  <span
                                    key={idx}
                                    className={`inline-flex items-center px-2 py-0.5 rounded text-xs ${tag.color}`}
                                  >
                                    {tag.icon} {tag.label}
                                  </span>
                                ))}
                                {email.sentiment && (
                                  <span className={`text-sm ${sentiment.color}`}>
                                    {sentiment.icon}
                                  </span>
                                )}
                                {email.client_name && (
                                  <span className="text-xs text-gray-500 dark:text-gray-400">
                                    • {email.client_name}
                                  </span>
                                )}
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

          {/* Email Detail Panel */}
          {selectedEmail && (
            <div className="w-3/5 bg-white dark:bg-slate-800 rounded-lg shadow overflow-hidden">
              <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-start">
                <div>
                  <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
                    {selectedEmail.subject}
                  </h2>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                    {selectedEmail.direction === 'inbound' ? 'From' : 'To'}: {selectedEmail.direction === 'inbound' ? selectedEmail.from_email : selectedEmail.to_email}
                  </p>
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                    {new Date(selectedEmail.created_at).toLocaleString()}
                  </p>
                </div>
                <button
                  onClick={() => setSelectedEmail(null)}
                  className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              {/* AI Summary */}
              {selectedEmail.ai_summary && (
                <div className="px-4 py-3 bg-blue-50 dark:bg-blue-900/20 border-b border-gray-200 dark:border-gray-700">
                  <div className="flex items-start">
                    <span className="text-xl mr-2">🤖</span>
                    <div>
                      <p className="text-sm font-medium text-blue-800 dark:text-blue-200">AI Summary</p>
                      <p className="text-sm text-blue-700 dark:text-blue-300">{selectedEmail.ai_summary}</p>
                    </div>
                  </div>
                </div>
              )}

              {/* Email Body */}
              <div className="p-4 overflow-auto max-h-[calc(100vh-400px)]">
                <div
                  className="prose dark:prose-invert max-w-none"
                  dangerouslySetInnerHTML={{ __html: selectedEmail.body_html }}
                />
              </div>

              {/* Action Bar */}
              <div className="p-4 border-t border-gray-200 dark:border-gray-700 flex gap-3">
                <button
                  onClick={() => {
                    setShowCompose(true);
                  }}
                  className="inline-flex items-center px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
                >
                  <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
                  </svg>
                  Reply
                </button>
                <button className="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 text-sm font-medium rounded-md hover:bg-gray-50 dark:hover:bg-slate-700 transition-colors">
                  <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z" />
                  </svg>
                  Forward
                </button>
              </div>
            </div>
          )}
        </div>
      </main>

      {/* Compose Modal */}
      {showCompose && (
        <ComposeModal
          onClose={() => setShowCompose(false)}
          onSent={() => {
            setShowCompose(false);
            toast.success('Email sent successfully');
            fetchEmails();
            fetchStats();
          }}
          replyTo={selectedEmail}
        />
      )}
    </div>
  );
}

// Compose Modal Component
function ComposeModal({
  onClose,
  onSent,
  replyTo,
}: {
  onClose: () => void;
  onSent: () => void;
  replyTo?: Email | null;
}) {
  const [to, setTo] = useState(replyTo?.from_email || '');
  const [subject, setSubject] = useState(replyTo ? `Re: ${replyTo.subject}` : '');
  const [body, setBody] = useState('');
  const [sending, setSending] = useState(false);

  const handleSend = async () => {
    if (!to || !subject || !body) {
      alert('Please fill in all fields');
      return;
    }

    setSending(true);
    try {
      const { sendEmail } = await import('@/lib/api');
      await sendEmail({
        to_email: to,
        subject,
        body_html: `<p>${body.replace(/\n/g, '</p><p>')}</p>`,
        body_text: body,
        type: 'manual',
      });
      onSent();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to send email');
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {replyTo ? 'Reply' : 'New Email'}
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
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                To
              </label>
              <input
                type="email"
                value={to}
                onChange={(e) => setTo(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                placeholder="recipient@example.com"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Subject
              </label>
              <input
                type="text"
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                placeholder="Email subject"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Message
              </label>
              <textarea
                value={body}
                onChange={(e) => setBody(e.target.value)}
                rows={12}
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-slate-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 resize-none"
                placeholder="Write your message..."
              />
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
            onClick={handleSend}
            disabled={sending}
            className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {sending ? 'Sending...' : 'Send'}
          </button>
        </div>
      </div>
    </div>
  );
}
