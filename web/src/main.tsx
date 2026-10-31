import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { Admin } from './admin/Admin';
import { Dashboard } from './dashboard/Dashboard';
import './styles.css';

createRoot(document.getElementById('root')!).render(<StrictMode>{window.location.pathname === '/admin' || window.location.pathname === '/admin/' ? <Admin /> : <Dashboard />}</StrictMode>);
