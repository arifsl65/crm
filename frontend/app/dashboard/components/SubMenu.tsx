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
  // UI_MODULES.md: Dashboard (6 items)
  dashboard: [
    { id: 'today', label: 'Today', icon: '📅', panel: 'today' },
    { id: 'overview', label: 'Overview', icon: '📊', panel: 'overview' },
    { id: 'deadlines', label: 'Deadlines', icon: '📆', panel: 'deadlines' },
    { id: 'alerts', label: 'Alerts', icon: '🔔', panel: 'alerts' },
    { id: 'actions', label: 'Actions', icon: '⚡', panel: 'actions' },
    { id: 'activity', label: 'Activity', icon: '📜', panel: 'activity' },
  ],
  // UI_MODULES.md: Clients (4 items)
  clients: [
    { id: 'all', label: 'All', icon: '📋', panel: 'list' },
    { id: 'add', label: 'Add', icon: '➕', panel: 'add' },
    { id: 'ch-lookup', label: 'CH Lookup', icon: '🔍', panel: 'ch-lookup' },
    { id: 'by-staff', label: 'By Staff', icon: '👤', panel: 'by-staff' },
  ],
  // UI_MODULES.md: Documents (5 items)
  documents: [
    { id: 'all', label: 'All', icon: '📋', panel: 'list' },
    { id: 'upload', label: 'Upload', icon: '⬆️', panel: 'upload' },
    { id: 'request', label: 'Request', icon: '📤', panel: 'request' },
    { id: 'e-sign', label: 'E-Sign', icon: '✍️', panel: 'e-sign' },
    { id: 'firm', label: 'Firm', icon: '🏢', panel: 'firm' },
  ],
  // UI_MODULES.md: Services (4 items)
  services: [
    { id: 'all', label: 'All', icon: '📋', panel: 'list' },
    { id: 'add', label: 'Add', icon: '➕', panel: 'add' },
    { id: 'hmrc', label: 'HMRC', icon: '🏛️', panel: 'hmrc' },
    { id: 'deadlines', label: 'Deadlines', icon: '📅', panel: 'deadlines' },
  ],
  // UI_MODULES.md: Email (5 items)
  email: [
    { id: 'inbox', label: 'Inbox', icon: '📥', panel: 'inbox' },
    { id: 'compose', label: 'Compose', icon: '✏️', panel: 'compose' },
    { id: 'chase', label: 'Chase', icon: '🔔', panel: 'chase' },
    { id: 'templates', label: 'Templates', icon: '📋', panel: 'templates' },
    { id: 'settings', label: 'Settings', icon: '⚙️', panel: 'settings' },
  ],
  // UI_MODULES.md: Staff (3 items)
  staff: [
    { id: 'all', label: 'All', icon: '📋', panel: 'list' },
    { id: 'add', label: 'Add', icon: '➕', panel: 'add' },
    { id: 'workload', label: 'Workload', icon: '📊', panel: 'workload' },
  ],
  // UI_MODULES.md: Settings (9 items)
  settings: [
    { id: 'company', label: 'Company', icon: '🏢', panel: 'company' },
    { id: 'integrations', label: 'Integrations', icon: '🔗', panel: 'integrations' },
    { id: 'doc-types', label: 'Doc Types', icon: '📄', panel: 'doc-types' },
    { id: 'service-types', label: 'Service Types', icon: '📋', panel: 'service-types' },
    { id: 'appearance', label: 'Appearance', icon: '🎨', panel: 'appearance' },
    { id: 'subscription', label: 'Subscription', icon: '💳', panel: 'subscription' },
    { id: 'branding', label: 'Branding', icon: '🏷️', panel: 'branding' },
    { id: 'security', label: 'Security', icon: '🔐', panel: 'security' },
    { id: 'audit-log', label: 'Audit Log', icon: '📊', panel: 'audit-log' },
  ],
  // UI_MODULES.md: AI Chat (2 items)
  ai: [
    { id: 'chat', label: 'Chat', icon: '💬', panel: 'chat' },
    { id: 'history', label: 'History', icon: '📜', panel: 'history' },
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
