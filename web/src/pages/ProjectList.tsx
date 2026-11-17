import { api, jsonInit, type Project } from '../api'
import { useApi, useField } from '../hooks'
import { nav } from '../nav'

export default function ProjectList() {
  const [projects, error, reload] = useApi(() => api<Project[]>('/api/projects'))
  const [name, onName, setName] = useField('')
  const [desc, onDesc, setDesc] = useField('')

  const create = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    try {
      await api('/api/projects', jsonInit('POST', { name, description: desc }))
      setName('')
      setDesc('')
      reload()
    } catch (err) {
      alert(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <main>
      <h1>Projects</h1>
      {error && <p className="error">{error}</p>}
      <table className="list">
        <tbody>
          {(projects ?? []).map((p) => (
            <tr key={p.id} onClick={() => nav(`/p/${p.id}`)}>
              <td className="muted">#{p.id}</td>
              <td>
                <strong>{p.name}</strong>
                {p.description && <div className="muted small">{p.description}</div>}
              </td>
              <td className="muted small right">{fmtDate(p.updated_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {projects && projects.length === 0 && <p className="muted">プロジェクトはまだありません</p>}

      <details className="panel">
        <summary>新しいプロジェクト</summary>
        <form onSubmit={create}>
          <input placeholder="プロジェクト名" value={name} onChange={onName} required />
          <input placeholder="説明(任意)" value={desc} onChange={onDesc} />
          <button type="submit">作成</button>
        </form>
      </details>
    </main>
  )
}

export function fmtDate(s: string): string {
  if (s.length < 16) return s
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toLocaleString('ja-JP', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
