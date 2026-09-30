import React from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import {
  Activity, LayoutDashboard, Globe, ShieldCheck, BarChart3,
  AlertTriangle, Radio, Plug, Settings, ChevronLeft, ChevronRight,
  Zap
} from 'lucide-react';
import { useSidebar } from '../../context/AppContext';
import { useIncidents } from '../../hooks/useIncidents';
import './Sidebar.css';

interface NavItem {
  path: string;
  label: string;
  icon: React.ReactNode;
  badge?: number;
}

const NAV_ITEMS: NavItem[] = [
  { path: '/',              label: 'Overview',        icon: <LayoutDashboard size={18} /> },
  { path: '/dependencies',  label: 'Dependencies',    icon: <Activity size={18} /> },
  { path: '/monitoring',    label: 'Live Monitoring', icon: <Radio size={18} /> },
  { path: '/reliability',   label: 'Reliability',     icon: <ShieldCheck size={18} /> },
  { path: '/analytics',     label: 'Analytics',       icon: <BarChart3 size={18} /> },
  { path: '/incidents',     label: 'Incidents',       icon: <AlertTriangle size={18} /> },
  { path: '/probes',        label: 'Synthetic Probes',icon: <Globe size={18} /> },
  { path: '/integrations',  label: 'Integrations',    icon: <Plug size={18} /> },
  { path: '/settings',      label: 'Settings',        icon: <Settings size={18} /> },
];

export default function Sidebar() {
  const { collapsed, toggleSidebar } = useSidebar();
  const location = useLocation();
  const { activeCount: activeIncidents } = useIncidents();

  const items = NAV_ITEMS.map(item => {
    if (item.path === '/incidents') return { ...item, badge: activeIncidents };
    return item;
  });

  return (
    <aside className={`sidebar ${collapsed ? 'sidebar--collapsed' : ''}`} aria-label="Primary navigation">
      {/* Logo */}
      <div className="sidebar__logo">
        <div className="sidebar__logo-mark">
          <Zap size={20} strokeWidth={2.5} />
        </div>
        {!collapsed && (
          <span className="sidebar__logo-text">
            API<span className="sidebar__logo-pulse">Pulse</span>
          </span>
        )}
      </div>

      <div className="sidebar__divider" />

      {/* Navigation */}
      <nav className="sidebar__nav" aria-label="Main navigation">
        {!collapsed && (
          <p className="sidebar__section-label">Monitoring</p>
        )}
        {items.slice(0, 5).map(item => (
          <SidebarNavLink key={item.path} item={item} collapsed={collapsed} currentPath={location.pathname} />
        ))}

        {!collapsed && (
          <p className="sidebar__section-label" style={{ marginTop: 'var(--space-4)' }}>Management</p>
        )}
        {items.slice(5).map(item => (
          <SidebarNavLink key={item.path} item={item} collapsed={collapsed} currentPath={location.pathname} />
        ))}
      </nav>

      {/* Live Runtime Indicator */}
      {!collapsed && (
        <div className="sidebar__demo-banner" style={{ background: 'rgba(34, 197, 94, 0.12)', borderColor: 'rgba(34, 197, 94, 0.25)', color: '#4ade80' }}>
          <div className="sidebar__demo-dot" style={{ background: '#4ade80', boxShadow: '0 0 6px #4ade80' }} />
          <span>LIVE RUNTIME</span>
        </div>
      )}

      {/* Collapse Toggle */}
      <button
        className="sidebar__collapse-btn"
        onClick={toggleSidebar}
        aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        title={collapsed ? 'Expand' : 'Collapse'}
      >
        {collapsed ? <ChevronRight size={16} /> : <ChevronLeft size={16} />}
      </button>
    </aside>
  );
}

function SidebarNavLink({
  item,
  collapsed,
  currentPath,
}: {
  item: NavItem;
  collapsed: boolean;
  currentPath: string;
}) {
  const isActive = item.path === '/'
    ? currentPath === '/'
    : currentPath.startsWith(item.path);

  return (
    <NavLink
      to={item.path}
      className={`sidebar__nav-item ${isActive ? 'sidebar__nav-item--active' : ''}`}
      title={collapsed ? item.label : undefined}
      aria-current={isActive ? 'page' : undefined}
    >
      <span className="sidebar__nav-icon">{item.icon}</span>
      {!collapsed && (
        <span className="sidebar__nav-label">{item.label}</span>
      )}
      {!collapsed && item.badge != null && item.badge > 0 && (
        <span className="sidebar__nav-badge" aria-label={`${item.badge} active`}>
          {item.badge}
        </span>
      )}
      {collapsed && item.badge != null && item.badge > 0 && (
        <span className="sidebar__nav-badge-dot" aria-label={`${item.badge} active`} />
      )}
    </NavLink>
  );
}
