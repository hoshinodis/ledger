import { useState } from 'react'
import { api, jsonInit, STATUSES, PRIORITIES, statusLabel, type Comment, type Issue } from '../api'
import { useApi, useField } from '../hooks'
import { nav } from '../nav'
import { fmtDate } from './ProjectList'

export default function IssueDetail({ id }: { id: number }) {
  const [issue, error, reload] = useApi(() => api<Issue>(`/api/issues/${id}`), [id])
  const [comments, commentsError, reloadComments] = useApi(
    () => api<Comment[]>(`/api/issues/${id}/comments`),
    [id],
  )

  const [completeOpen, setCompleteOpen] = useState(false)
  const [result, onResult, setResult] = useField('')
  const [commentBody, onCommentBody, setCommentBody] = useField('')
  const [author, onAuthor] = useField('')
  const [sessionId, onSessionId] = useField('')

  const patch = async (fields: Record<string, unknown>) => {
    try {
      await api(`/api/issues/${id}`, jsonInit('PATCH', fields))
      reload()
    } catch (err) {
      alert(err instanceof Error ? err.message : String(err))
    }
  }

  const complete = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!result.trim()) return
    try {
      await api(`/api/issues/${id}/complete`, jsonInit('POST', {
        body: result,
        author: author.trim(),
        session_id: sessionId.trim(),
      }))
      setResult('')
      setCompleteOpen(false)
      reload()
      reloadComments()
    } catch (err) {
      alert(err instanceof Error ? err.message : String(err))
    }
  }

  const addComment = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!commentBody.trim()) return
    try {
      await api(`/api/issues/${id}/comments`, jsonInit('POST', {
        body: commentBody,
        author: author.trim(),
        session_id: sessionId.trim(),
      }))
      setCommentBody('')
      reloadComments()
    } catch (err) {
      alert(err instanceof Error ? err.message : String(err))
    }
  }

  if (error) {
    return (
      <main>
        <p className="error">{error}</p>
        {issue?.project_id && (
          <a href={`/p/${issue.project_id}`} onClick={(e) => (e.preventDefault(), nav(`/p/${issue.project_id}`))}>← Project</a>
        )}
      </main>
    )
  }
  if (!issue) return <main>Loading…</main>

  const done = issue.status === 'done'

  return (
    <main>
      <a href={`/p/${issue.project_id}`} onClick={(e) => (e.preventDefault(), nav(`/p/${issue.project_id}`))}>← Project #{issue.project_id}</a>
      <h1>
        #{issue.id} {issue.title}
      </h1>
      <div className="meta">
        <span className={`badge badge-${issue.status}`}>{statusLabel[issue.status] ?? issue.status}</span>
        <span className={`prio prio-${issue.priority}`}>{issue.priority}</span>
        {(issue.labels ?? []).map((l) => (
          <span key={l.id} className="label">{l.name}</span>
        ))}
        {issue.assignee_session_id && <span className="muted">session: {issue.assignee_session_id}</span>}
        {done && issue.completed_at && <span className="muted small">completed {fmtDate(issue.completed_at)}</span>}
      </div>

      <div className="actions">
        {!done && (
          <select value="" onChange={(e) => e.target.value && patch({ status: e.target.value })}>
            <option value="">ステータス変更…</option>
            {STATUSES.map((s) => (
              <option key={s} value={s}>{statusLabel[s]}</option>
            ))}
          </select>
        )}
        <select value="" onChange={(e) => e.target.value && patch({ priority: e.target.value })}>
          <option value="">優先度変更…</option>
          {PRIORITIES.map((p) => (
            <option key={p} value={p}>{p}</option>
          ))}
        </select>
        <button onClick={() => patch({ assignee_session_id: '' })}>担当解除</button>
      </div>

      <section className="panel">
        <h2>本文</h2>
        {issue.body ? <pre className="context">{issue.body}</pre> : <p className="muted small">(本文なし)</p>}
      </section>

      {!done && (
        <details className="panel" open={completeOpen} onToggle={(e) => setCompleteOpen((e.target as HTMLDetailsElement).open)}>
          <summary>完了にする(結果コメント必須)</summary>
          <form onSubmit={complete}>
            <textarea
              rows={4}
              placeholder="何をしたか・判断理由・残課題(Markdown)"
              value={result}
              onChange={onResult}
              required
            />
            <div className="inline">
              <input placeholder="実行者名" value={author} onChange={onAuthor} />
              <input placeholder="セッションID(任意)" value={sessionId} onChange={onSessionId} />
            </div>
            <button type="submit">Done にする</button>
          </form>
        </details>
      )}

      <section>
        <h2>Comments</h2>
        {commentsError && <p className="error">{commentsError}</p>}
        {(comments ?? []).map((c) => (
          <div key={c.id} className="comment">
            <div className="muted small">
              #{c.id} {fmtDate(c.created_at)}
              {c.author && ` ${c.author}`}
              {c.session_id && ` (${c.session_id})`}
            </div>
            <pre>{c.body}</pre>
          </div>
        ))}
        {comments && comments.length === 0 && <p className="muted small">コメントはまだありません</p>}

        <form onSubmit={addComment}>
          <textarea
            rows={3}
            placeholder="調査結果・判断・変更内容・残課題を書き込む"
            value={commentBody}
            onChange={onCommentBody}
            required
          />
          <div className="inline">
            <input placeholder="投稿者名" value={author} onChange={onAuthor} />
            <input placeholder="セッションID(任意)" value={sessionId} onChange={onSessionId} />
          </div>
          <button type="submit">コメント追加</button>
        </form>
      </section>
    </main>
  )
}
