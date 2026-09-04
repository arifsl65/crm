'use client';

import { usePathname } from 'next/navigation';
import Link from 'next/link';

interface SubMenuItem {
  id: string;
  label: string;
  icon: string;
  panel: string; // Which panel to show (for dashboard) or route path (for other modules)
  route?: string; // Optional explicit route path
}

interface ModuleSubMenu {
  [key: string]: SubMenuItem[];
}

// Modules that use routing (Link navigation) instead of panel switching
const routingModules = ['documents', 'clients', 'services', 'email', 'staff', 'settings', 'ai'];

const subMenuItems: ModuleSubMenu = {
  // UI_MODULES.md: Dashboard (6 items) - Uses panel switching
  dashboard: [
    { id: 'today', label: 'Today', icon: '📅', panel: 'today' },
    { id: 'overview', label: 'Overview', icon: '📊', panel: 'overview' },
    { id: 'deadlines', label: 'Deadlines', icon: '📆', panel: 'deadlines' },
    { id: 'alerts', label: 'Alerts', icon: '🔔', panel: 'alerts' },
    { id: 'actions', label: 'Actions', icon: '⚡', panel: 'actions' },
    { id: 'activity', label: 'Activity', icon: '📜', panel: 'activity' },
  ],
  // UI_MODULES.md: Clients (4 items) - Uses routing
  clients: [
    { id: 'all', label: 'All', icon: '📋', panel: 'list', route: '/dashboard/clients' },
    { id: 'add', label: 'Add', icon: '➕', panel: 'add', route: '/dashboard/clients/add' },
    { id: 'ch-lookup', label: 'CH Lookup', icon: '🔍', panel: 'ch-lookup', route: '/dashboard/clients/ch-lookup' },
    { id: 'by-staff', label: 'By Staff', icon: '👤', panel: 'by-staff', route: '/dashboard/clients/by-staff' },
  ],
  // UI_MODULES.md: Documents (5 items) - Uses routing
  documents: [
    { id: 'all', label: 'All', icon: '📋', panel: 'list', route: '/dashboard/documents' },
    { id: 'upload', label: 'Upload', icon: '⬆️', panel: 'upload', route: '/dashboard/documents/upload' },
    { id: 'request', label: 'Request', icon: '📤', panel: 'request', route: '/dashboard/documents/request' },
    { id: 'e-sign', label: 'E-Sign', icon: '✍️', panel: 'e-sign', route: '/dashboard/documents/e-sign' },
    { id: 'firm', label: 'Firm', icon: '🏢', panel: 'firm', route: '/dashboard/documents/firm' },
  ],
  // UI_MODULES.md: Services (4 items) - Uses routing
  services: [
    { id: 'all', label: 'All', icon: '📋', panel: 'list', route: '/dashboard/services' },
    { id: 'add', label: 'Add', icon: '➕', panel: 'add', route: '/dashboard/services/add' },
    { id: 'hmrc', label: 'HMRC', icon: '🏛️', panel: 'hmrc', route: '/dashboard/services/hmrc' },
    { id: 'deadlines', label: 'Deadlines', icon: '📅', panel: 'deadlines', route: '/dashboard/services/deadlines' },
  ],
  // UI_MODULES.md: Email (5 items) - Uses routing
  email: [
    { id: 'inbox', label: 'Inbox', icon: '📥', panel: 'inbox', route: '/dashboard/email' },
    { id: 'compose', label: 'Compose', icon: '✏️', panel: 'compose', route: '/dashboard/email/compose' },
    { id: 'chase', label: 'Chase', icon: '🔔', panel: 'chase', route: '/dashboard/email/chase' },
    { id: 'templates', label: 'Templates', icon: '📋', panel: 'templates', route: '/dashboard/email/templates' },
    { id: 'settings', label: 'Settings', icon: '⚙️', panel: 'settings', route: '/dashboard/email/settings' },
  ],
  // UI_MODULES.md: Staff (3 items) - Uses routing
  staff: [
    { id: 'all', label: 'All', icon: '📋', panel: 'list', route: '/dashboard/staff' },
    { id: 'add', label: 'Add', icon: '➕', panel: 'add', route: '/dashboard/staff/add' },
    { id: 'workload', label: 'Workload', icon: '📊', panel: 'workload', route: '/dashboard/staff/workload' },
  ],
  // UI_MODULES.md: Settings (9 items) - Uses routing
  settings: [
    { id: 'company', label: 'Company', icon: '🏢', panel: 'company', route: '/dashboard/settings' },
    { id: 'integrations', label: 'Integrations', icon: '🔗', panel: 'integrations', route: '/dashboard/settings/integrations' },
    { id: 'doc-types', label: 'Doc Types', icon: '📄', panel: 'doc-types', route: '/dashboard/settings/doc-types' },
    { id: 'service-types', label: 'Service Types', icon: '📋', panel: 'service-types', route: '/dashboard/settings/service-types' },
    { id: 'appearance', label: 'Appearance', icon: '🎨', panel: 'appearance', route: '/dashboard/settings/appearance' },
    { id: 'subscription', label: 'Subscription', icon: '💳', panel: 'subscription', route: '/dashboard/settings/subscription' },
    { id: 'branding', label: 'Branding', icon: '🏷️', panel: 'branding', route: '/dashboard/settings/branding' },
    { id: 'security', label: 'Security', icon: '🔐', panel: 'security', route: '/dashboard/settings/security' },
    { id: 'audit-log', label: 'Audit Log', icon: '📊', panel: 'audit-log', route: '/dashboard/settings/audit-log' },
  ],
  // UI_MODULES.md: AI Chat (2 items) - Uses routing
  ai: [
    { id: 'chat', label: 'Chat', icon: '💬', panel: 'chat', route: '/dashboard/ai' },
    { id: 'history', label: 'History', icon: '📜', panel: 'history', route: '/dashboard/ai/history' },
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

  // Check if this item is currently active based on route
  const isItemActive = (item: SubMenuItem) => {
    if (activeModule === 'dashboard') {
      return activePanel === item.panel;
    }
    // For routing modules, check if current pathname matches the route
    if (item.route) {
      // Exact match for base routes (e.g., /dashboard/documents)
      if (pathname === item.route) return true;
      // Check if we're on a sub-route but this is the "all" item (base route)
      if (item.id === 'all' || item.id === 'inbox' || item.id === 'company' || item.id === 'chat') {
        // Only match exactly, not sub-routes
        return pathname === item.route;
      }
      // For other items, match if pathname starts with their route
      return pathname.startsWith(item.route);
    }
    return false;
  };

  const useRouting = routingModules.includes(activeModule);

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
        {items.map((item) => {
          const isActive = isItemActive(item);
          const className = `
            w-full flex items-center gap-3 px-4 py-2.5 text-sm font-medium transition-all text-left
            ${isActive
              ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 border-r-2 border-blue-600'
              : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-slate-700 hover:text-gray-900 dark:hover:text-white'
            }
          `;

          // Use Link for routing modules, button for dashboard panels
          if (useRouting && item.route) {
            return (
              <Link
                key={item.id}
                href={item.route}
                className={className}
              >
                <span className="text-base">{item.icon}</span>
                <span>{item.label}</span>
              </Link>
            );
          }

          return (
            <button
              key={item.id}
              onClick={() => onPanelSelect(item.panel)}
              className={className}
            >
              <span className="text-base">{item.icon}</span>
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>
    </aside>
  );
}

export default SubMenu;
