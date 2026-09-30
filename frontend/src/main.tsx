import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './app/App';
import { ToastProvider, SidebarProvider } from './context/AppContext';
import './styles/global.css';
import './components/ui/components.css';

const rootElement = document.getElementById('root');

if (!rootElement) {
  throw new Error('Failed to find the root element in index.html');
}

ReactDOM.createRoot(rootElement).render(
  <React.StrictMode>
    <BrowserRouter>
      <ToastProvider>
        <SidebarProvider>
          <App />
        </SidebarProvider>
      </ToastProvider>
    </BrowserRouter>
  </React.StrictMode>
);
