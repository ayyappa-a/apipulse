import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import Sidebar from '../components/Sidebar/Sidebar';
import Topbar from '../components/Topbar/Topbar';
import Overview from '../pages/Overview';
import Dependencies from '../pages/Dependencies';
import Monitoring from '../pages/Monitoring';
import Reliability from '../pages/Reliability';
import Analytics from '../pages/Analytics';
import Incidents from '../pages/Incidents';
import Probes from '../pages/Probes';
import Integrations from '../pages/Integrations';
import Settings from '../pages/Settings';
import { useToast } from '../context/AppContext';
import { X, CheckCircle, AlertTriangle, AlertCircle, Info } from 'lucide-react';
import './App.css';

export default function App() {
  const { toasts, removeToast } = useToast();

  const getToastIcon = (type: string) => {
    switch (type) {
      case 'success':
        return <CheckCircle size={16} className="text-success" />;
      case 'warning':
        return <AlertTriangle size={16} className="text-warning" />;
      case 'error':
        return <AlertCircle size={16} className="text-danger" />;
      default:
        return <Info size={16} className="text-primary" />;
    }
  };

  return (
    <div className="app-layout">
      {/* Sidebar Navigation */}
      <Sidebar />

      {/* Main Container */}
      <div className="app-main">
        {/* Top Header Bar */}
        <Topbar />

        {/* Scrollable Viewport Content */}
        <main className="app-content" id="main-content" role="main">
          <Routes>
            <Route path="/" element={<Overview />} />
            <Route path="/dependencies" element={<Dependencies />} />
            <Route path="/dependencies/:id" element={<Dependencies />} />
            <Route path="/monitoring" element={<Monitoring />} />
            <Route path="/reliability" element={<Reliability />} />
            <Route path="/analytics" element={<Analytics />} />
            <Route path="/incidents" element={<Incidents />} />
            <Route path="/probes" element={<Probes />} />
            <Route path="/integrations" element={<Integrations />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
      </div>

      {/* Toast Notification Stack */}
      {toasts.length > 0 && (
        <div className="toast-container" aria-live="polite">
          {toasts.map(toast => (
            <div key={toast.id} className={`toast-item toast-item--${toast.type}`}>
              {getToastIcon(toast.type)}
              <span className="toast-message">{toast.message}</span>
              <button
                className="toast-close-btn"
                onClick={() => removeToast(toast.id)}
                aria-label="Dismiss notification"
              >
                <X size={14} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
