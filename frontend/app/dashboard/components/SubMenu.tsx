'use client';

import { usePathname } from 'next/navigation';

interface SubMenuItem {
  id: string;
  label: string;
  icon: string;
  panel: string; // Which panel to show
}

interface ModuleSubMenu {
  [key: string]: SubMenuItem[];
}

const subMenuItems: ModuleSubMenu = {
  dashboard: [
    { id: 'today', label: 'Today', icon: '📅', panel: 'today' },
    { id: 'overview', label: 'Overview', icon: '📊', panel: 'overview' },
    { id: 'deadlines', label: 'Deadlines', icon: '📆', panel: 'deadlines' },
    { id: 'alerts', label: 'Alerts', icon: '🔔', panel: 'alerts' },
    { id: 'actions', label: 'Actions', icon: '⚡', panel: 'actions' },
    { id: 'activity', label: 'Activity', icon: '📜', panel: 'activity' },
  ],
  clients: [
    { id: 'all', label: 'All Clients', icon: '📋', panel: 'list' },
    { id: 'add', label: 'Add Client', icon: '➕', panel: 'add' },
    { id: 'ch-lookup', label: 'CH Lookup', icon: '🔍', panel: 'ch-lookup' },
    { id: 'by-staff', label: 'By Staff', icon: '👤', panel: 'by-staff' },
  ],
  documents: [
    { id: 'all', label: 'All Documents', icon: '📄', panel: 'list' },
    { id: 'pending', label: 'Pending Review', icon: '⏳', panel: 'pending' },
    { id: 'upload', label: 'Upload', icon: '📤', panel: 'upload' },
    { id: 'firm', label: 'Firm Documents', icon: '🏢', panel: 'firm' },
  ],
  services: [
    { id: 'all', label: 'All Services', icon: '📋', panel: 'list' },
    { id: 'kanban', label: 'Kanban View', icon: '📊', panel: 'kanban' },
    { id: 'add', label: 'Add Service', icon: '➕', panel: 'add' },
    { id: 'templates', label: 'Templates', icon: '📑', panel: 'templates' },
  ],
  hmrc: [
    { id: 'overview', label: 'Overview', icon: '🏛️', panel: 'overview' },
    { id: 'vat', label: 'VAT Returns', icon: '📊', panel: 'vat' },
    { id: 'ct600', label: 'CT600', icon: '🏢', panel: 'ct600' },
    { id: 'sa', label: 'Self Assessment', icon: '👤', panel: 'sa' },
    { id: 'deadlines', label: 'Deadlines', icon: '📅', panel: 'deadlines' },
  ],
  email: [
    { id: 'inbox', label: 'Inbox', icon: '📥', panel: 'inbox' },
    { id: 'sent', label: 'Sent', icon: '📤', panel: 'sent' },
    { id: 'triage', label: 'Triage', icon: '📋', panel: 'triage' },
    { id: 'templates', label: 'Templates', icon: '📑', panel: 'templates' },
    { id: 'settings', label: 'Settings', icon: '⚙️', panel: 'settings' },
  ],
  staff: [
    { id: 'all', label: 'All Staff', icon: '👥', panel: 'list' },
    { id: 'invite', label: 'Invite', icon: '✉️', panel: 'invite' },
    { id: 'workload', label: 'Workload', icon: '📊', panel: 'workload' },
  ],
  settings: [
    { id: 'company', label: 'Company', icon: '🏢', panel: 'company' },
    { id: 'branding', label: 'Branding', icon: '🎨', panel: 'branding' },
    { id: 'security', label: 'Security', icon: '🔒', panel: 'security' },
    { id: 'integrations', label: 'Integrations', icon: '🔗', panel: 'integrations' },
  ],
  ai: [
    { id: 'chat', label: 'Chat', icon: '💬', panel: 'chat' },
    { id: 'history', label: 'History', icon: '📜', panel: 'history' },
    { id: 'actions', label: 'Quick Actions', icon: '⚡', panel: 'actions' },
  ],
};

interface SubMenuProps {
  activePanel: string;
  onPanelSelect: (panel: string) => void;
  currentModule?: string;
}

export function SubMenu({ activePanel, onPanelSelect, currentModule }: SubMenuProps) {
  const pathname = usePathname();

  // Determine which module we're in based on pathname or prop
  const getActiveModule = (): string => {
    if (currentModule) return currentModule;
    if (pathname === '/dashboard' || pathname === '/dashboard/') return 'dashboard';
    const segments = pathname.split('/').filter(s => s); // Remove empty segments
    if (segments.length >= 2) {
      return segments[1]; // e.g., /dashboard/clients -> clients
    }
    return 'dashboard';
  };

  const activeModule = getActiveModule();
  const items = subMenuItems[activeModule] || [];

  // Only show submenu for modules that have sub-items
  if (items.length === 0) return null;

  // Get display title for the module
  const getModuleTitle = () => {
    switch (activeModule) {
      case 'ai': return 'AI Chat';
      case 'hmrc': return 'HMRC';
      default: return activeModule.charAt(0).toUpperCase() + activeModule.slice(1);
    }
  };

  return (
    <aside className="w-48 bg-white dark:bg-slate-800/50 border-r border-gray-200 dark:border-gray-700 flex flex-col shrink-0">
      {/* Module Title */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700">
        <h2 className="font-semibold text-gray-900 dark:text-white">
          {getModuleTitle()}
        </h2>
      </div>

      {/* Sub-menu items */}
      <nav className="flex-1 py-2 overflow-y-auto">
        {items.map((item) => (
          <button
            key={item.id}
            onClick={() => onPanelSelect(item.panel)}
            className={`
              w-full flex items-center gap-3 px-4 py-2.5 text-sm font-medium transition-all text-left
              ${activePanel === item.panel
                ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 border-r-2 border-blue-600'
                : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-slate-700 hover:text-gray-900 dark:hover:text-white'
              }
            `}
          >
            <span className="text-base">{item.icon}</span>
            <span>{item.label}</span>
          </button>
        ))}
      </nav>
    </aside>
  );
}

export default SubMenu;
