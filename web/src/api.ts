export type Label = {
  id: number
  project_id: number
  name: string
  color?: string
}

export type Project = {
  id: number
  name: string
  description: string
  context: string
  created_at: string
  updated_at: string
}

export type Issue = {
  id: number
  project_id: number
  title: string
  body: string
  status: string
  priority: string
  assignee_session_id?: string
  labels?: Label[]
  created_at: string
  updated_at: string
  completed_at?: string
}

export type Comment = {
  id: number
  issue_id: number
  author?: string
  session_id?: string
  body: string
  created_at: string
}

export type Bootstrap = {
  project: Project
  my_issues: Issue[] | null
  open_issues: Issue[] | null
  recently_done: Issue[] | null
  recent_comments: Comment[] | null
}

/** api calls the ledger REST API and unwraps JSON or a friendly error. */
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    let msg = res.statusText
    try {
      msg = (await res.json()).error ?? msg
    } catch {
      // keep status text when the body is not JSON
    }
    throw new Error(`${res.status} ${msg}`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const jsonInit = (method: string, body: unknown): RequestInit => ({
  method,
  body: JSON.stringify(body),
})

export const STATUSES = ['todo', 'in_progress', 'cancelled'] as const
export const PRIORITIES = ['low', 'medium', 'high', 'urgent'] as const

export const statusLabel: Record<string, string> = {
  todo: 'Todo',
  in_progress: 'In Progress',
  done: 'Done',
  cancelled: 'Cancelled',
}
