'use client';

import { useState, useEffect } from 'react';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/components/auth-guard';
import { Sidebar } from './Sidebar';
import { SubMenu } from './SubMenu';
import { Header } from './Header';
import { AIChat } from './AIChat';
import {
  TodayPanel,
  OverviewPanel,
  DeadlinesPanel,
  AlertsPanel,
  ActionsPanel,
  ActivityPanel,
} from './panels';

interface DashboardShellProps {
  children: React.ReactNode;
}

export function DashboardShell({ children }: DashboardShellProps) {
  const { user, logout } = useAuth();
  const pathname = usePathname();
  const [activePanel, setActivePanel] = useState('today');
  const [showAIChat, setShowAIChat] = useState(true);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  // Determine if we're on the main dashboard or a module page
  const isMainDashboard = pathname === '/dashboard';
  const isAIPage = pathname === '/dashboard/ai';

  // Get current module from pathname
  const getCurrentModule = (): string => {
    if (pathname === '/dashboard') return 'dashboard';
    const segments = pathname.split('/');
    if (segments.length >= 3) {
      return segments[2];
    }
    return 'dashboard';
  };

  const currentModule = getCurrentModule();

  // Modules that should show the AI Chat in collapsed mode
  const modulesWithCollapsedChat = ['clients', 'documents', 'services', 'email', 'staff', 'hmrc'];
  const shouldCollapseChat = modulesWithCollapsedChat.includes(currentModule) && !isAIPage;

  // Handle panel selection from submenu
  const handlePanelSelect = (panel: string) => {
    setActivePanel(panel);
  };

  // Handle AI Chat actions
  const handleAIAction = (action: string, itemId?: string) => {
    console.log('AI Action:', action, itemId);
    // TODO: Implement action handling
  };

  // Render the appropriate panel based on activePanel state
  const renderDashboardPanel = () => {
    switch (activePanel) {
      case 'today':
        return <TodayPanel />;
      case 'overview':
        return <OverviewPanel />;
      case 'deadlines':
        return <DeadlinesPanel />;
      case 'alerts':
        return <AlertsPanel />;
      case 'actions':
        return <ActionsPanel />;
      case 'activity':
        return <ActivityPanel />;
      default:
        return <TodayPanel />;
    }
  };

  if (!mounted) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-slate-900">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-100 dark:bg-slate-900 flex flex-col">
      {/* Header */}
      <Header user={user || undefined} onLogout={logout} />

      {/* Main Content Area */}
      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar - Always visible */}
        <Sidebar userRole={user?.role} />

        {/* SubMenu - Shows for current module */}
        <SubMenu activePanel={activePanel} onPanelSelect={handlePanelSelect} />

        {/* Content Area */}
        <main className="flex-1 flex overflow-hidden">
          {/* Main Content */}
          <div className={`flex-1 overflow-y-auto ${shouldCollapseChat ? '' : 'lg:flex-[2]'}`}>
            {/* If on dashboard root and showing a panel, show panel content */}
            {isMainDashboard ? (
              <div className="h-full p-4">
                <div className="bg-white dark:bg-slate-800 rounded-lg shadow-sm h-full">
                  {renderDashboardPanel()}
                </div>
              </div>
            ) : (
              /* Module pages render their own content */
              <div className="h-full">
                {children}
              </div>
            )}
          </div>

          {/* AI Chat - Right side (collapsible on module pages) */}
          {!isAIPage && (
            <div
              className={`
                hidden lg:block border-l border-gray-200 dark:border-gray-700 overflow-hidden transition-all
                ${shouldCollapseChat ? 'w-80' : 'flex-1 max-w-md'}
              `}
            >
              <div className="h-full p-4">
                <AIChat
                  userName={user?.name || user?.email?.split('@')[0]}
                  minimized={shouldCollapseChat}
                  onAction={handleAIAction}
                />
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

export default DashboardShell;
