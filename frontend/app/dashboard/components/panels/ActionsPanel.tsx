'use client';

import { useState } from 'react';
import Link from 'next/link';

interface ActionItem {
  id: string;
  icon: string;
  label: string;
  description: string;
  variant: 'primary' | 'secondary' | 'warning';
  count?: number;
  action?: () => void;
  href?: string;
}

interface ActionsPanelProps {
  overdueCount?: number;
  quietCount?: number;
  pendingDocsCount?: number;
  onClose?: () => void;
}

export function ActionsPanel({
  overdueCount = 3,
  quietCount = 5,
  pendingDocsCount = 8,
  onClose,
}: ActionsPanelProps) {
  const [isProcessing, setIsProcessing] = useState<string | null>(null);

  const handleChaseAll = async () => {
    setIsProcessing('chase-all');
    // TODO: Implement chase all functionality
    setTimeout(() => {
      setIsProcessing(null);
      alert('Chase emails would be sent to all overdue clients');
    }, 1000);
  };

  const handleBulkReminders = async () => {
    setIsProcessing('bulk-reminders');
    // TODO: Implement bulk reminders
    setTimeout(() => {
      setIsProcessing(null);
      alert('Bulk reminders would be sent');
    }, 1000);
  };

  const actions: ActionItem[] = [
    {
      id: 'chase-all',
      icon: '📧',
      label: `Chase All Overdue`,
      description: `Send chase emails to ${overdueCount} overdue clients`,
      variant: 'warning',
      count: overdueCount,
      action: handleChaseAll,
    },
    {
      id: 'troublemakers',
      icon: '🔍',
      label: 'Show Troublemakers',
      description: 'View clients with repeated issues',
      variant: 'secondary',
      href: '/dashboard/clients?filter=at_risk',
    },
    {
      id: 'bulk-reminders',
      icon: '📨',
      label: 'Send Bulk Reminders',
      description: `Send document request reminders to ${quietCount} quiet clients`,
      variant: 'primary',
      count: quietCount,
      action: handleBulkReminders,
    },
    {
      id: 'pending-docs',
      icon: '📄',
      label: 'Review Pending Documents',
      description: `${pendingDocsCount} documents awaiting review`,
      variant: 'secondary',
      count: pendingDocsCount,
      href: '/dashboard/documents?status=pending',
    },
    {
      id: 'draft-reminder',
      icon: '✏️',
      label: 'Draft Reminder Email',
      description: 'Create a custom reminder template',
      variant: 'secondary',
      href: '/dashboard/email/templates',
    },
    {
      id: 'export-report',
      icon: '📊',
      label: 'Export Status Report',
      description: 'Download a summary of all pending items',
      variant: 'secondary',
    },
  ];

  return (
    <div className="h-full flex flex-col bg-white dark:bg-slate-800 rounded-lg">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
          <span>⚡</span> Quick Actions
        </h2>
        {onClose && (
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
            ✕
          </button>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {actions.map((action) => (
          <ActionButton
            key={action.id}
            action={action}
            isProcessing={isProcessing === action.id}
          />
        ))}
      </div>

      {/* AI Suggestion */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700">
        <div className="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <span className="text-xl">🤖</span>
            <div className="flex-1">
              <p className="text-sm font-medium text-blue-900 dark:text-blue-100">
                AI Suggestion
              </p>
              <p className="text-xs text-blue-700 dark:text-blue-300 mt-1">
                {overdueCount > 0
                  ? `You have ${overdueCount} overdue items. Would you like me to draft chase emails for all of them?`
                  : 'All caught up! No urgent actions needed.'}
              </p>
              {overdueCount > 0 && (
                <button
                  onClick={handleChaseAll}
                  disabled={isProcessing === 'chase-all'}
                  className="mt-2 px-3 py-1 text-xs font-medium text-white bg-blue-600 rounded hover:bg-blue-700 disabled:opacity-50"
                >
                  {isProcessing === 'chase-all' ? 'Processing...' : 'Do it'}
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function ActionButton({
  action,
  isProcessing,
}: {
  action: ActionItem;
  isProcessing: boolean;
}) {
  const getButtonStyle = () => {
    switch (action.variant) {
      case 'primary':
        return 'bg-blue-600 text-white hover:bg-blue-700 border-blue-600';
      case 'warning':
        return 'bg-orange-500 text-white hover:bg-orange-600 border-orange-500';
      default:
        return 'bg-white dark:bg-slate-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-slate-600 border-gray-200 dark:border-gray-600';
    }
  };

  const content = (
    <div className={`w-full p-4 rounded-lg border transition-all ${getButtonStyle()} ${isProcessing ? 'opacity-70' : ''}`}>
      <div className="flex items-center gap-3">
        <span className="text-2xl">{action.icon}</span>
        <div className="flex-1 text-left">
          <div className="flex items-center gap-2">
            <p className="font-medium">{action.label}</p>
            {action.count !== undefined && action.count > 0 && (
              <span className={`px-1.5 py-0.5 text-xs font-medium rounded ${
                action.variant === 'secondary'
                  ? 'bg-gray-200 dark:bg-slate-600 text-gray-700 dark:text-gray-300'
                  : 'bg-white/20 text-white'
              }`}>
                {action.count}
              </span>
            )}
          </div>
          <p className={`text-xs mt-0.5 ${
            action.variant === 'secondary'
              ? 'text-gray-500 dark:text-gray-400'
              : 'text-white/80'
          }`}>
            {action.description}
          </p>
        </div>
        {isProcessing && (
          <div className="animate-spin rounded-full h-5 w-5 border-2 border-white border-t-transparent"></div>
        )}
      </div>
    </div>
  );

  if (action.href) {
    return (
      <Link href={action.href} className="block">
        {content}
      </Link>
    );
  }

  return (
    <button
      onClick={action.action}
      disabled={isProcessing}
      className="w-full"
    >
      {content}
    </button>
  );
}

export default ActionsPanel;
