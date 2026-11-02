import { useEffect, useState } from 'react';
import { deleteDiagnostics, fetchDiagnostics, type Diagnostic, type Scraper } from './api';
export function useDiagnostics(scraper: Scraper, csrf: string) {
  const [severity, setSeverity] = useState('');
  const [before, setBefore] = useState('');
  const [version, refresh] = useState(0);
  const [busy, setBusy] = useState(false);
  const [mutationError, setMutationError] = useState('');
  const { rows, error } = useEntries(scraper, severity, before, version);
  const reload = () => { setBefore(''); refresh(value => value + 1); };
  const remove = async (id?: string) => {
    if (!window.confirm(id ? 'Excluir este diagnóstico após a análise?' : 'Excluir todos os diagnósticos deste coletor?')) return;
    setBusy(true); setMutationError('');
    try { await deleteDiagnostics(scraper, csrf, id); reload(); }
    catch { setMutationError('Não foi possível excluir. Confira sua sessão e tente novamente.'); }
    finally { setBusy(false); }
  };
  return { rows, error: mutationError || error, severity, setSeverity, before, setBefore, busy, reload, remove };
}
function useEntries(scraper: Scraper, severity: string, before: string, version: number) {
  const [rows, setRows] = useState<Diagnostic[] | null>(null);
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    setRows(null); setError('');
    void fetchDiagnostics(scraper, severity, before).then(result => { if (active) setRows(result); })
      .catch(() => { if (active) setError('Não foi possível carregar os diagnósticos. Confira sua sessão e tente novamente.'); });
    return () => { active = false; };
  }, [scraper, severity, before, version]);
  return { rows, error };
}
