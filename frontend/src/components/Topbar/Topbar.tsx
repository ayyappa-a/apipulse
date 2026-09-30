import React, { useState, useRef, useEffect } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Search, Bell, User, ChevronDown, Menu, X, Command } from 'lucide-react';
import { useSidebar } from '../../context/AppContext';
import { useIncidents } from '../../hooks/useIncidents';
import { useDependencies } from '../../hooks/useDependencies';
import { getStatusMeta } from '../../utils/status';
import type { Incident, Dependency } from '../../types/domain';
import './Topbar.css';

const BREADCRUMBS: Record<string, string> = {
  '/':             'Overview',
  '/dependencies': 'Dependencies',
  '/monitoring':   'Live Monitoring',
  '/reliability':  'Reliability',
  '/analytics':    'Analytics',
  '/incidents':    'Incidents',
  '/probes':       'Synthetic Probes',
  '/integrations': 'Integrations',
  '/settings':     'Settings',
};

export default function Topbar() {
  const { toggleSidebar } = useSidebar();
  const location = useLocation();
  const navigate = useNavigate();
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [notifOpen, setNotifOpen] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);

  const { incidents } = useIncidents();
  const { dependencies } = useDependencies();
  const activeIncidents = incidents.filter((i: Incident) => i.status !== 'resolved');

  const pageTitle = Object.entries(BREADCRUMBS).find(
    ([path]) => path === '/'
      ? location.pathname === '/'
      : location.pathname.startsWith(path)
  )?.[1] ?? 'APIPulse';

  // Keyboard shortcut: Ctrl+K / Cmd+K
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        setSearchOpen(true);
        setTimeout(() => searchRef.current?.focus(), 100);
      }
      if (e.key === 'Escape') {
        setSearchOpen(false);
        setSearchQuery('');
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  // Search results
  const searchResults = searchQuery.length > 1
    ? dependencies.filter((d: Dependency) =>
        d.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        d.vendor.displayName.toLowerCase().includes(searchQuery.toLowerCase())
      ).slice(0, 5)
    : [];

  const overallDegraded = dependencies.some(
    (d: Dependency) => d.status !== 'healthy' && d.status !== 'unknown'
  );

  return (
    <>
      <header className="topbar" role="banner">
        {/* Left: Menu + Breadcrumb */}
        <div className="topbar__left">
          <button
            className="topbar__icon-btn"
            onClick={toggleSidebar}
            aria-label="Toggle sidebar"
            id="sidebar-toggle"
          >
            <Menu size={18} />
          </button>
          <div className="topbar__breadcrumb">
            <span className="topbar__breadcrumb-root">APIPulse</span>
            <span className="topbar__breadcrumb-sep">/</span>
            <span className="topbar__breadcrumb-current">{pageTitle}</span>
          </div>
        </div>

        {/* Center: System Health Pill */}
        <div className="topbar__center">
          <div className={`topbar__health-pill ${overallDegraded ? 'topbar__health-pill--warn' : 'topbar__health-pill--ok'}`}>
            <span className={`health-dot ${overallDegraded ? 'health-dot--degraded' : 'health-dot--healthy'}`} />
            <span>{overallDegraded ? 'Degraded' : 'All Systems Healthy'}</span>
          </div>
        </div>

        {/* Right: Search, Notifications, User */}
        <div className="topbar__right">
          {/* Search trigger */}
          <button
            className="topbar__search-trigger"
            onClick={() => { setSearchOpen(true); setTimeout(() => searchRef.current?.focus(), 100); }}
            aria-label="Search (Ctrl+K)"
            id="global-search-trigger"
          >
            <Search size={15} />
            <span className="topbar__search-label">Search...</span>
            <kbd className="topbar__search-kbd">
              <Command size={10} />K
            </kbd>
          </button>

          {/* Notifications */}
          <div className="topbar__notif-wrap">
            <button
              className={`topbar__icon-btn ${notifOpen ? 'topbar__icon-btn--active' : ''}`}
              onClick={() => setNotifOpen(!notifOpen)}
              aria-label={`Notifications — ${activeIncidents.length} active incidents`}
              id="notifications-btn"
            >
              <Bell size={18} />
              {activeIncidents.length > 0 && (
                <span className="topbar__notif-badge">{activeIncidents.length}</span>
              )}
            </button>

            {notifOpen && (
              <div className="topbar__notif-dropdown">
                <div className="topbar__notif-header">
                  <span>Active Incidents</span>
                  <button className="topbar__icon-btn" onClick={() => setNotifOpen(false)}>
                    <X size={14} />
                  </button>
                </div>
                {activeIncidents.map((inc: Incident) => {
                  const meta = getStatusMeta(inc.healthStatus);
                  return (
                    <button
                      key={inc.id}
                      className="topbar__notif-item"
                      onClick={() => { navigate('/incidents'); setNotifOpen(false); }}
                    >
                      <span className={`health-dot ${meta.dotClass}`} />
                      <div className="topbar__notif-item-body">
                        <p className="topbar__notif-item-title">{inc.title}</p>
                        <p className="topbar__notif-item-meta">{inc.vendorName} · {meta.label}</p>
                      </div>
                    </button>
                  );
                })}
                {activeIncidents.length === 0 && (
                  <p className="topbar__notif-empty">No active incidents</p>
                )}
              </div>
            )}
          </div>

          {/* User Menu */}
          <button className="topbar__user-btn" aria-label="User menu" id="user-menu-btn">
            <div className="topbar__avatar" aria-hidden>SRE</div>
            <span className="topbar__username">SRE Team</span>
            <ChevronDown size={14} />
          </button>
        </div>
      </header>

      {/* Global Search Modal */}
      {searchOpen && (
        <>
          <div className="backdrop" onClick={() => { setSearchOpen(false); setSearchQuery(''); }} />
          <div className="topbar__search-modal" role="dialog" aria-label="Global search">
            <div className="topbar__search-modal-inner">
              <Search size={16} className="topbar__search-modal-icon" />
              <input
                ref={searchRef}
                className="topbar__search-modal-input"
                placeholder="Search dependencies, incidents, probes..."
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                aria-label="Search"
                autoComplete="off"
              />
              <button
                className="topbar__search-modal-close"
                onClick={() => { setSearchOpen(false); setSearchQuery(''); }}
                aria-label="Close search"
              >
                ESC
              </button>
            </div>
            {searchResults.length > 0 && (
              <div className="topbar__search-results">
                <p className="topbar__search-results-label">Dependencies</p>
                {searchResults.map((dep: Dependency) => {
                  const meta = getStatusMeta(dep.status);
                  return (
                    <button
                      key={dep.id}
                      className="topbar__search-result-item"
                      onClick={() => {
                        navigate(`/dependencies/${dep.id}`);
                        setSearchOpen(false);
                        setSearchQuery('');
                      }}
                    >
                      <span className={`health-dot ${meta.dotClass}`} />
                      <span className="topbar__search-result-name">{dep.name}</span>
                      <span className="topbar__search-result-vendor">{dep.vendor.displayName}</span>
                      <span className={`badge ${meta.cssClass}`}>{meta.label}</span>
                    </button>
                  );
                })}
              </div>
            )}
            {searchQuery.length > 1 && searchResults.length === 0 && (
              <div className="topbar__search-empty">
                <Search size={24} style={{ color: 'var(--color-text-tertiary)' }} />
                <p>No results for "{searchQuery}"</p>
              </div>
            )}
          </div>
        </>
      )}
    </>
  );
}
