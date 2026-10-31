import { useState, type FormEvent } from 'react';

export function Login({ login, busy }: { login: (username: string, password: string) => Promise<unknown>; busy: boolean }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  async function submit(event: FormEvent) { event.preventDefault(); await login(username, password); setPassword(''); }
  return <form onSubmit={submit}><h2>Entrar</h2>
    <label>Usuário<input autoComplete="username" required value={username} onChange={event => setUsername(event.target.value)} /></label>
    <label>Senha<input type="password" autoComplete="current-password" required value={password} onChange={event => setPassword(event.target.value)} /></label>
    <button disabled={busy}>Entrar</button></form>;
}
