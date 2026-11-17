import { useState } from 'react'
import { api, jsonInit, PRIORITIES, statusLabel, type Issue, type Project } from '../api'
import { useApi, useField } from '../hooks'
import { nav } from '../nav'
import { fmtDate } from './ProjectList'

export default function ProjectDetail({ id }: { id: number }) {
  const [project, error, reload] = useApi(() => api<Project>(`/api/projects/${id}`), [id])
  const [status, setStatus] = useState('')
  const [priority, setPriority] = useState('')
  const qs = new URLSearchParams()
  if (status) qs.set('status', status)
  if (priority) qs.set('priority', priority)
  const [issues, issuesError, reloadIssues] = useApi(
    () => api<Issue[]>(`/api/issues?project_id=${id}&${qs}`),
    [id, status, priority],
  )

  const [editing, setEditing] = useState(false)
  const [md, setMd] = useState('')
  const [title, onTitle, setTitle] = useField('')
  const [body, onBody, setBody] = useField('')
  const [labels, onLabels] = useField('')
  const [session, onSession] = useField('')
  const [newPriority, setNewPriority] = useState('medium')

  const startEdit = () => {
    setMd(project?.context ?? '')
    setEditing(true)
  }

  const saveContext = async () => {
    try {
      await api(`/api/projects/${id}/context`, jsonInit('PUT', { context: md }))
      setEditing(false)
      reload()
    } catch (err) {
      alert(err instanceof Error ? err.message : String(err))
    }
  }

  const createIssue = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) return
    const payload: Record<string, unknown> = {
      project_id: id,
      title,
      body,
      priority: newPriority,
    }
    if (labels.trim()) payload.labels = labels.split(',').map((s) => s.trim()).filter(Boolean)
    if (session.trim()) payload.assignee_session_id = session.trim()
    try {
      await api('/api/issues', jsonInit('POST', payload))
      setTitle('')
      setBody('')
      reloadIssues()
    } catch (err) {
      alert(err instanceof Error ? err.message : String(err))
    }
  }

  if (error) {
    return (
      <main>
        <p className="error">{error}</p>
        <a href="/" onClick={(e) => (e.preventDefault(), nav('/'))}>← Projects</a>
      </main>
    )
  }
  if (!project) return <main>Loading…</main>

  return (
    <main>
      <a href="/" onClick={(e) => (e.preventDefault(), nav('/'))}>← Projects</a>
      <h1>
        #{project.id} {project.name}
      </h1>
      {project.description && <p className="muted">{project.description}</p>}

      <section className="panel">
        <h2>Project Context</h2>
        {editing ? (
          <div>
            <textarea
              rows={12}
              value={md}
              onChange={(e) => setMd(e.target.value)}
              placeholder="# 目的&#10;# 技術スタック&#10;# 決定事項 …"
            />
            <div className="actions">
              <button onClick={saveContext}>保存</button>
              <button className="ghost" onClick={() => setEditing(false)}>キャンセル</button>
            </div>
          </div>
        ) : (
          <div>
            {project.context ? (
              <pre className="context">{project.context}</pre>
            ) : (
              <p className="muted small">プロジェクトの目的・技術スタック・決定事項を書いておくと、新しいセッションがすぐ参照できます。</p>
            )}
            <button className="ghost" onClick={startEdit}>編集</button>
          </div>
        )}
      </section>

      <section>
        <h2>Issues</h2>
        <div className="filters">
          <label>
            status
            <select value={status} onChange={(e) => setStatus(e.target.value)}>
              <option value="">すべて</option>
              <option value="todo">Todo</option>
              <option value="in_progress">In Progress</option>
              <option value="done">Done</option>
              <option value="cancelled">Cancelled</option>
            </select>
          </label>
          <label>
            priority
            <select value={priority} onChange={(e) => setPriority(e.target.value)}>
              <option value="">すべて</option>
              {PRIORITIES.map((p) => (
                <option key={p} value={p}>{p}</option>
              ))}
            </select>
          </label>
        </div>

        {issuesError && <p className="error">{issuesError}</p>}
        <table className="list">
          <tbody>
            {(issues ?? []).map((i) => (
              <tr key={i.id} onClick={() => nav(`/p/${id}/i/${i.id}`)}>
                <td className="muted">#{i.id}</td>
                <td><span className={`badge badge-${i.status}`}>{statusLabel[i.status] ?? i.status}</span></td>
                <td className="muted">{i.priority}</td>
                <td>
                  <strong>{i.title}</strong>
                  {(i.labels?.length ?? 0) > 0 && (
                    <div className="muted small">{i.labels!.map((l) => l.name).join(', ')}</div>
                  )}
                </td>
                <td className="muted small">{i.assignee_session_id ?? ''}</td>
                <td className="muted small right">{fmtDate(i.updated_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {issues && issues.length === 0 && <p className="muted">該当する issue はありません</p>}

        <details className="panel">
          <summary>新しい Issue</summary>
          <form onSubmit={createIssue}>
            <input placeholder="タイトル(必須)" value={title} onChange={onTitle} required />
            <textarea rows={5} placeholder="説明・背景・完了条件(Markdown)" value={body} onChange={onBody} />
            <div className="inline">
              <select value={newPriority} onChange={(e) => setNewPriority(e.target.value)}>
                {PRIORITIES.map((p) => (
                  <option key={p} value={p}>{p}</option>
                ))}
              </select>
              <input placeholder="ラベル(カンマ区切り)" value={labels} onChange={onLabels} />
              <input placeholder="担当セッションID(任意)" value={session} onChange={onSession} />
            </div>
            <button type="submit">作成</button>
          </form>
        </details>
      </section>
    </main>
  )
}
