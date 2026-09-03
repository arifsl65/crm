'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { getClients, getDocuments, getServices, getUsers } from '@/lib/api';

interface SearchResult {
  id: string;
  type: 'client' | 'document' | 'service' | 'staff';
  title: string;
  subtitle?: string;
  url: string;
}

interface RecentItem {
  id: string;
  type: 'client' | 'document' | 'service' | 'staff';
  title: string;
  url: string;
  accessedAt: Date;
}

interface GlobalSearchProps {
  placeholder?: string;
  onSearch?: (query: string) => Promise<SearchResult[]>;
  className?: string;
}

/**
 * GlobalSearch - Command palette style search component
 * Triggered by Ctrl/Cmd + K
 */
const RECENT_ITEMS_KEY = 'global-search-recent';
const MAX_RECENT_ITEMS = 5;

export function GlobalSearch({
  placeholder = 'Search everything...',
  onSearch,
  className = '',
}: GlobalSearchProps) {
  const router = useRouter();
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [recentItems, setRecentItems] = useState<RecentItem[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Load recent items from localStorage
  useEffect(() => {
    try {
      const stored = localStorage.getItem(RECENT_ITEMS_KEY);
      if (stored) {
        const items = JSON.parse(stored);
        setRecentItems(items.slice(0, MAX_RECENT_ITEMS));
      }
    } catch (e) {
      console.error('Failed to load recent items:', e);
    }
  }, []);

  // Save item to recent when clicked
  const addToRecent = (item: SearchResult) => {
    const newRecent: RecentItem = {
      id: item.id,
      type: item.type,
      title: item.title,
      url: item.url,
      accessedAt: new Date(),
    };

    setRecentItems((prev) => {
      // Remove if already exists, add to front
      const filtered = prev.filter((r) => r.id !== item.id || r.type !== item.type);
      const updated = [newRecent, ...filtered].slice(0, MAX_RECENT_ITEMS);

      // Save to localStorage
      try {
        localStorage.setItem(RECENT_ITEMS_KEY, JSON.stringify(updated));
      } catch (e) {
        console.error('Failed to save recent items:', e);
      }

      return updated;
    });
  };

  // Keyboard shortcut to open search
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setIsOpen(true);
      }
      if (e.key === 'Escape') {
        setIsOpen(false);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  // Focus input when opened
  useEffect(() => {
    if (isOpen && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isOpen]);

  // Search handler - calls real API endpoints
  const handleSearch = useCallback(
    async (searchQuery: string) => {
      if (!searchQuery.trim()) {
        setResults([]);
        return;
      }

      setIsLoading(true);
      try {
        if (onSearch) {
          const searchResults = await onSearch(searchQuery);
          setResults(searchResults);
        } else {
          // Search all endpoints in parallel
          const [clientsRes, documentsRes, servicesRes, usersRes] = await Promise.allSettled([
            getClients({ search: searchQuery, limit: 5 }),
            getDocuments({ search: searchQuery, limit: 5 }),
            getServices({ search: searchQuery, limit: 5 }),
            getUsers({ search: searchQuery, limit: 5 }),
          ]);

          const allResults: SearchResult[] = [];

          // Process clients
          if (clientsRes.status === 'fulfilled' && clientsRes.value?.clients) {
            clientsRes.value.clients.forEach((client: { id: string; company_name: string; contact_name?: string }) => {
              allResults.push({
                id: client.id,
                type: 'client',
                title: client.company_name,
                subtitle: client.contact_name || undefined,
                url: `/dashboard/clients/${client.id}`,
              });
            });
          }

          // Process documents
          if (documentsRes.status === 'fulfilled' && documentsRes.value?.documents) {
            documentsRes.value.documents.forEach((doc: { id: string; name: string; client_name?: string }) => {
              allResults.push({
                id: doc.id,
                type: 'document',
                title: doc.name,
                subtitle: doc.client_name || undefined,
                url: `/dashboard/documents/${doc.id}`,
              });
            });
          }

          // Process services
          if (servicesRes.status === 'fulfilled' && servicesRes.value?.services) {
            servicesRes.value.services.forEach((service: { id: string; name: string; client_name?: string }) => {
              allResults.push({
                id: service.id,
                type: 'service',
                title: service.name,
                subtitle: service.client_name || undefined,
                url: `/dashboard/services/${service.id}`,
              });
            });
          }

          // Process users (staff)
          if (usersRes.status === 'fulfilled' && usersRes.value?.users) {
            usersRes.value.users.forEach((user: { id: string; name?: string; email: string; role?: string }) => {
              allResults.push({
                id: user.id,
                type: 'staff',
                title: user.name || user.email,
                subtitle: user.role || undefined,
                url: `/dashboard/staff/${user.id}`,
              });
            });
          }

          setResults(allResults);
        }
      } catch (error) {
        console.error('Search error:', error);
        setResults([]);
      } finally {
        setIsLoading(false);
      }
    },
    [onSearch]
  );

  // Debounced search
  useEffect(() => {
    const timer = setTimeout(() => {
      handleSearch(query);
    }, 300);
    return () => clearTimeout(timer);
  }, [query, handleSearch]);

  // Keyboard navigation
  // Fix #31: Use router.push() for client-side SPA navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex((prev) => Math.min(prev + 1, results.length - 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex((prev) => Math.max(prev - 1, 0));
        break;
      case 'Enter':
        e.preventDefault();
        if (query && results[selectedIndex]) {
          addToRecent(results[selectedIndex]);
          router.push(results[selectedIndex].url);
          setIsOpen(false);
        } else if (!query && recentItems[selectedIndex]) {
          router.push(recentItems[selectedIndex].url);
          setIsOpen(false);
        }
        break;
    }
  };

  // Click outside to close
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
    }
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isOpen]);

  const typeIcons: Record<string, React.ReactNode> = {
    client: <span className="text-sm">👥</span>,
    document: <span className="text-sm">📄</span>,
    service: <span className="text-sm">📋</span>,
    staff: <span className="text-sm">👤</span>,
  };

  const typeLabels: Record<string, string> = {
    client: 'Client',
    document: 'Document',
    service: 'Service',
    staff: 'Staff',
  };

  if (!isOpen) {
    return (
      <button
        onClick={() => setIsOpen(true)}
        className={`flex items-center gap-2 px-3 py-2 text-sm text-gray-500 bg-gray-100 dark:bg-gray-800 dark:text-gray-400 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors ${className}`}
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <span>Search...</span>
        <kbd className="hidden sm:inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-gray-400 bg-gray-200 dark:bg-gray-700 rounded">
          <span className="text-xs">Ctrl</span>K
        </kbd>
      </button>
    );
  }

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="min-h-screen px-4 pt-4 pb-20 text-center sm:block sm:p-0">
        {/* Backdrop */}
        <div className="fixed inset-0 bg-gray-500 bg-opacity-75 dark:bg-gray-900 dark:bg-opacity-80 transition-opacity" />

        {/* Dialog */}
        <div
          ref={containerRef}
          className="relative inline-block w-full max-w-lg my-8 sm:my-16 text-left align-middle transition-all transform bg-white dark:bg-gray-800 rounded-xl shadow-2xl"
        >
          {/* Search Input */}
          <div className="flex items-center px-4 border-b border-gray-200 dark:border-gray-700">
            <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setSelectedIndex(0);
              }}
              onKeyDown={handleKeyDown}
              placeholder={placeholder}
              className="w-full px-4 py-4 text-gray-900 dark:text-white bg-transparent border-0 focus:ring-0 focus:outline-none"
            />
            {isLoading && (
              <svg className="w-5 h-5 text-gray-400 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
            )}
          </div>

          {/* Results */}
          <div className="max-h-80 overflow-y-auto">
            {/* Show recent items when no query */}
            {!query && recentItems.length > 0 && (
              <div className="py-2">
                <div className="px-4 py-2 text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                  Recent
                </div>
                <ul>
                  {recentItems.map((item, index) => (
                    <li key={`${item.type}-${item.id}`}>
                      <button
                        type="button"
                        className={`w-full flex items-center gap-3 px-4 py-3 text-left ${
                          index === selectedIndex
                            ? 'bg-blue-50 dark:bg-blue-900/30'
                            : 'hover:bg-gray-50 dark:hover:bg-gray-700'
                        }`}
                        onMouseEnter={() => setSelectedIndex(index)}
                        onClick={() => {
                          router.push(item.url);
                          setIsOpen(false);
                        }}
                      >
                        <div className="flex-shrink-0 w-8 h-8 flex items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-700">
                          <span className="text-gray-400">🕐</span>
                        </div>
                        <div className="flex-1 min-w-0">
                          <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                            {item.title}
                          </p>
                        </div>
                        <span className="text-xs text-gray-400 dark:text-gray-500">
                          {typeLabels[item.type]}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
                <div className="px-4 py-3 text-xs text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-700">
                  Type to search clients, documents, services, staff...
                </div>
              </div>
            )}

            {/* Show placeholder when no query and no recent */}
            {!query && recentItems.length === 0 && (
              <div className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                Type to search clients, documents, services, staff...
              </div>
            )}

            {/* Show search results grouped by type when query exists */}
            {query && results.length > 0 && (() => {
              // Group results by type
              const grouped = results.reduce((acc, result) => {
                if (!acc[result.type]) acc[result.type] = [];
                if (acc[result.type].length < 5) { // Max 5 per group
                  acc[result.type].push(result);
                }
                return acc;
              }, {} as Record<string, SearchResult[]>);

              // Flatten for keyboard navigation index
              const flatResults = Object.values(grouped).flat();
              let currentIndex = 0;

              return (
                <div className="py-2">
                  {(['client', 'document', 'service', 'staff'] as const).map((type) => {
                    const items = grouped[type];
                    if (!items || items.length === 0) return null;

                    return (
                      <div key={type}>
                        <div className="px-4 py-2 text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider border-t border-gray-100 dark:border-gray-700 first:border-t-0">
                          {typeLabels[type]}s
                        </div>
                        <ul>
                          {items.map((result) => {
                            const itemIndex = currentIndex++;
                            return (
                              <li key={`${result.type}-${result.id}`}>
                                <button
                                  type="button"
                                  className={`w-full flex items-center gap-3 px-4 py-3 text-left ${
                                    itemIndex === selectedIndex
                                      ? 'bg-blue-50 dark:bg-blue-900/30'
                                      : 'hover:bg-gray-50 dark:hover:bg-gray-700'
                                  }`}
                                  onMouseEnter={() => setSelectedIndex(itemIndex)}
                                  onClick={() => {
                                    addToRecent(result);
                                    router.push(result.url);
                                    setIsOpen(false);
                                  }}
                                >
                                  <div className="flex-shrink-0 w-8 h-8 flex items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-700">
                                    {typeIcons[result.type]}
                                  </div>
                                  <div className="flex-1 min-w-0">
                                    <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                                      {result.title}
                                    </p>
                                    {result.subtitle && (
                                      <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                                        {result.subtitle}
                                      </p>
                                    )}
                                  </div>
                                  <span className="text-xs text-blue-600 dark:text-blue-400">
                                    → Open
                                  </span>
                                </button>
                              </li>
                            );
                          })}
                        </ul>
                      </div>
                    );
                  })}
                </div>
              );
            })()}

            {/* Show no results message */}
            {query && results.length === 0 && !isLoading && (
              <div className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                No results found for &quot;{query}&quot;
              </div>
            )}
          </div>

          {/* Footer */}
          <div className="px-4 py-2 text-xs text-gray-400 border-t border-gray-200 dark:border-gray-700 flex items-center gap-4">
            <span className="flex items-center gap-1">
              <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">Enter</kbd>
              to select
            </span>
            <span className="flex items-center gap-1">
              <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">Esc</kbd>
              to close
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}

export default GlobalSearch;
