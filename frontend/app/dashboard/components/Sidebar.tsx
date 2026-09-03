'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

interface MenuItem {
  id: string;
  label: string;
  icon: string;
  href: string;
  adminOnly?: boolean;
}

const menuItems: MenuItem[] = [
  { id: 'dashboard', label: 'Dashboard', icon: '🏠', href: '/dashboard' },
  { id: 'ai', label: 'AI Chat', icon: '🤖', href: '/dashboard/ai' },
  { id: 'clients', label: 'Clients', icon: '👥', href: '/dashboard/clients' },
  { id: 'documents', label: 'Documents', icon: '📄', href: '/dashboard/documents' },
  { id: 'services', label: 'Services', icon: '📋', href: '/dashboard/services' },
  { id: 'hmrc', label: 'HMRC', icon: '🏛️', href: '/dashboard/hmrc' },
  { id: 'email', label: 'Email', icon: '📧', href: '/dashboard/email' },
  { id: 'staff', label: 'Staff', icon: '👤', href: '/dashboard/staff', adminOnly: true },
  { id: 'settings', label: 'Settings', icon: '⚙️', href: '/dashboard/settings', adminOnly: true },
];

interface SidebarProps {
  userRole?: string;
  onModuleSelect?: (moduleId: string) => void;
}

export function Sidebar({ userRole, onModuleSelect }: SidebarProps) {
  const pathname = usePathname();

  const isAdmin = userRole === 'super_admin' || userRole === 'tenant_admin';

  const filteredItems = menuItems.filter(item => !item.adminOnly || isAdmin);

  // Group items
  const dailyUse = filteredItems.filter(item => ['dashboard', 'ai'].includes(item.id));
  const coreData = filteredItems.filter(item => ['clients', 'documents', 'services', 'hmrc', 'email'].includes(item.id));
  const admin = filteredItems.filter(item => ['staff', 'settings'].includes(item.id));

  const isActive = (href: string) => {
    if (href === '/dashboard') {
      return pathname === '/dashboard';
    }
    return pathname.startsWith(href);
  };

  const renderGroup = (items: MenuItem[], label?: string) => (
    <div className="mb-2">
      {label && (
        <div className="px-3 py-1 text-[10px] font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-wider">
          {label}
        </div>
      )}
      {items.map((item) => (
        <Link
          key={item.id}
          href={item.href}
          onClick={() => onModuleSelect?.(item.id)}
          className={`
            flex items-center gap-3 px-3 py-2.5 mx-2 rounded-lg text-sm font-medium transition-all
            ${isActive(item.href)
              ? 'bg-blue-600 text-white shadow-md'
              : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-slate-700'
            }
          `}
        >
          <span className="text-lg">{item.icon}</span>
          <span>{item.label}</span>
          {item.id === 'ai' && (
            <span className="ml-auto w-2 h-2 bg-green-500 rounded-full animate-pulse" title="AI Online" />
          )}
        </Link>
      ))}
    </div>
  );

  return (
    <aside className="w-56 bg-white dark:bg-slate-800 border-r border-gray-200 dark:border-gray-700 flex flex-col h-full">
      {/* Logo */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700">
        <Link href="/dashboard" className="flex items-center gap-2">
          <span className="text-2xl">📊</span>
          <span className="font-bold text-lg text-gray-900 dark:text-white">Accountant CRM</span>
        </Link>
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 overflow-y-auto">
        {renderGroup(dailyUse, 'Daily Use')}
        <div className="my-2 mx-4 border-t border-gray-200 dark:border-gray-700" />
        {renderGroup(coreData, 'Core Data')}
        {admin.length > 0 && (
          <>
            <div className="my-2 mx-4 border-t border-gray-200 dark:border-gray-700" />
            {renderGroup(admin, 'Admin')}
          </>
        )}
      </nav>

      {/* Bottom section */}
      <div className="p-4 border-t border-gray-200 dark:border-gray-700">
        <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span className="w-2 h-2 bg-green-500 rounded-full"></span>
          <span>All systems operational</span>
        </div>
      </div>
    </aside>
  );
}

export default Sidebar;
