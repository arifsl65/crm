'use client';

import { useState, useEffect, createContext, useContext } from 'react';
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

// Panel context for sharing state with module pages
interface PanelContextType {
  activePanel: string;
  setActivePanel: (panel: string) => void;
}

const PanelContext = createContext<PanelContextType | null>(null);

export function usePanelContext() {
  const context = useContext(PanelContext);
  if (!context) {
    throw new Error('usePanelContext must be used within DashboardShell');
  }
  return context;
}

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
  const isMainDashboard = pathname === '/dashboard' || pathname === '/dashboard/';
  const isAIPage = pathname === '/dashboard/ai' || pathname === '/dashboard/ai/';

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
    <PanelContext.Provider value={{ activePanel, setActivePanel }}>
      <div className="min-h-screen bg-gray-100 dark:bg-slate-900 flex flex-col">
        {/* Header */}
        <Header user={user || undefined} onLogout={logout} />

        {/* Main Content Area */}
        <div className="flex-1 flex overflow-hidden">
          {/* Sidebar - Always visible */}
          <Sidebar userRole={user?.role} />

          {/* SubMenu - Shows for all modules (not just main dashboard) */}
          <SubMenu
            activePanel={activePanel}
            onPanelSelect={handlePanelSelect}
            currentModule={currentModule}
          />

          {/* Content Area */}
          <main className="flex-1 flex overflow-hidden">
            {/* Main Content - Panel or Module Content */}
            <div className="flex-1 overflow-y-auto">
              {isMainDashboard ? (
                <div className="h-full p-4">
                  <div className="bg-white dark:bg-slate-800 rounded-lg shadow-sm h-full overflow-hidden">
                    {renderDashboardPanel()}
                  </div>
                </div>
              ) : (
                <div className="h-full">
                  {children}
                </div>
              )}
            </div>

            {/* AI Chat - Right side */}
            {!isAIPage && (
              <div
                className={`
                  hidden lg:block border-l border-gray-200 dark:border-gray-700 overflow-hidden transition-all
                  ${isMainDashboard ? 'w-96' : shouldCollapseChat ? 'w-80' : 'w-96'}
                `}
              >
                <div className="h-full p-4">
                  <AIChat
                    userName={user?.name || user?.email?.split('@')[0]}
                    minimized={!isMainDashboard && shouldCollapseChat}
                    onAction={handleAIAction}
                  />
                </div>
              </div>
            )}
          </main>
        </div>
      </div>
    </PanelContext.Provider>
  );
}

export default DashboardShell;
export { PanelContext };
