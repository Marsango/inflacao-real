import { Admin } from './admin/Admin';
import { About } from './about/About';
import { Dashboard } from './dashboard/Dashboard';

export function App() {
  const pathname = window.location.pathname.replace(/\/$/, '');
  if (pathname === '/admin') return <Admin />;
  if (pathname === '/sobre') return <About />;
  return <Dashboard />;
}
