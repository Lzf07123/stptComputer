const TOKEN_KEY = 'quiz_token'
const PENDING_KEY = 'quiz_pending_answers'
const FP_KEY = 'device_fp'

export function getToken(): string | null {
  return sessionStorage.getItem(TOKEN_KEY)
}

export function setToken(t: string) {
  sessionStorage.setItem(TOKEN_KEY, t)
}

export function clearToken() {
  sessionStorage.removeItem(TOKEN_KEY)
}

export function getDeviceFP(): string {
  let fp = localStorage.getItem(FP_KEY)
  if (!fp) {
    fp = crypto.randomUUID()
    localStorage.setItem(FP_KEY, fp)
  }
  return fp
}

export async function api<T = any>(path: string, opts: RequestInit = {}): Promise<T> {
  const headers = new Headers(opts.headers)
  headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const res = await fetch(path, { ...opts, headers })
  let body: any = {}
  try {
    body = await res.json()
  } catch {
    /* 空响应 */
  }
  if (!res.ok) {
    const err = new Error((body && body.error) || `HTTP ${res.status}`) as Error & {
      status?: number
      code?: string
    }
    err.status = res.status
    if (body && body.code) err.code = body.code
    throw err
  }
  return body as T
}

// ---------- 幂等提交缓冲：断网不丢分 ----------

export interface PendingAnswer {
  request_id: string
  question_id: number
  selected: number | null
}

export function loadPending(): PendingAnswer[] {
  try {
    return JSON.parse(localStorage.getItem(PENDING_KEY) || '[]')
  } catch {
    return []
  }
}

function savePending(q: PendingAnswer[]) {
  localStorage.setItem(PENDING_KEY, JSON.stringify(q))
}

export function pushPending(a: PendingAnswer) {
  const q = loadPending()
  q.push(a)
  savePending(q)
}

export async function flushPending(): Promise<number> {
  const q = loadPending()
  const rest: PendingAnswer[] = []
  let ok = 0
  for (const a of q) {
    try {
      await api('/api/answer', { method: 'POST', body: JSON.stringify(a) })
      ok++
    } catch {
      rest.push(a)
    }
  }
  savePending(rest)
  return ok
}
