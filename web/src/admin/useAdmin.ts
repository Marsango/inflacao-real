import { useEffect, useState } from 'react';
import { AdminError, adminRequest, type Decision, type Session, type Snapshot } from './api';

export function useAdmin() {
  const [session, setSession] = useState<Session | null>(null);
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const reload = async () => { setSnapshot(await adminRequest<Snapshot>('candidates')); };
  useEffect(() => { void adminRequest<Session>('session').then(setSession).catch(() => {}); }, []);
  useEffect(() => { if (session) void reload().catch(report); }, [session]);
  function report(cause: unknown) {
    setError(cause instanceof Error ? cause.message : 'Falha ao carregar.'); if (cause instanceof AdminError && cause.status === 401) { setSession(null); setSnapshot(null); }
  }
  async function run(operation: () => Promise<void>) {
    setBusy(true); setError('');
    try { await operation(); return true; } catch (cause) { report(cause); return false; } finally { setBusy(false); }
  }
  return adminActions({ session, snapshot, error, busy, run, reload, setSession, setSnapshot });
}
function adminActions({ session, snapshot, error, busy, run, reload, setSession, setSnapshot }: AdminState) {
  return { session, snapshot, error, busy, refresh: () => run(reload),
    login: (username: string, password: string) => run(async () => { setSession(await adminRequest<Session>('session', 'POST', { username, password })); }),
    logout: () => run(async () => { await adminRequest('session', 'DELETE', undefined, session?.csrf); setSession(null); setSnapshot(null); }),
    decide: (decision: Decision) => run(async () => { await adminRequest('reviews', 'POST', { ...decision, revision: snapshot?.revision }, session?.csrf); await reload(); }),
  };
}

interface AdminState {
  session: Session | null; snapshot: Snapshot | null; error: string; busy: boolean;
  run: (operation: () => Promise<void>) => Promise<boolean>;
  reload: () => Promise<void>;
  setSession: (value: Session | null) => void;
  setSnapshot: (value: Snapshot | null) => void;
}
