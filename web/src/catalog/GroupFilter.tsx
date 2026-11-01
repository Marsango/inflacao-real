import { useGroups } from './useGroups';

type Props = { group: string; select: (group: string) => void };

export function GroupFilter({ group, select }: Props) {
  const { status, groups, retry } = useGroups();
  const selected = groups.find((option) => option.id === group);
  return <fieldset className="group-filter"><legend>Categorias</legend>
    <div className="presets" role="group" aria-label="Categorias de produtos">
      <button type="button" aria-pressed={group === ''} onClick={() => select('')}>Todos os produtos</button>
      {groups.map((option) => <button key={option.id} type="button" aria-pressed={group === option.id}
        onClick={() => select(option.id)}>{option.label}</button>)}
    </div>
    {status === 'loading' && <p role="status">Carregando categorias…</p>}
    {status === 'error' && <p role="alert">Não foi possível carregar as categorias. <button type="button" onClick={retry}>Tentar carregar categorias novamente</button></p>}
    {selected && <p>{selected.description}</p>}
    {status === 'ready' && group && !selected && <p role="alert">Categoria desconhecida. Selecione outra categoria ou todos os produtos.</p>}
  </fieldset>;
}
