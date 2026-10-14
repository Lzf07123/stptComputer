// 招新答题系统压测（k6）
// 用法：
//   docker compose -p stpt-loadtest up -d --build   （独立环境，WEB_PORT=8081）
//   curl -X POST .../api/admin/quiz-state -d '{"state":"start"}'
//   docker run --rm -i --add-host host.docker.internal:host-gateway grafana/k6 run - < loadtest/k6-loadtest.js
// 三个场景：答题全流程 150 VU、排行榜轮询 200 VU、WebSocket 50 VU
import http from 'k6/http'
import { check, sleep } from 'k6'
import ws from 'k6/ws'

const BASE = __ENV.BASE_URL || 'http://host.docker.internal:8081'
const WS_BASE = BASE.replace('http', 'ws')

export const options = {
  scenarios: {
    quiz_flow: { executor: 'constant-vus', exec: 'quizFlow', vus: 150, duration: '90s' },
    rank_poll: { executor: 'constant-vus', exec: 'rankPoll', vus: 200, duration: '90s' },
    ws_conn:   { executor: 'constant-vus', exec: 'wsConn',   vus: 50,  duration: '60s' },
  },
  thresholds: {
    // 对应设计文档 §1.3：答题提交 P99<100ms、排行榜端到端 <500ms
    'http_req_duration{name:answer}': ['p(99)<100'],
    'http_req_duration{name:rank}':   ['p(99)<500'],
    'ws_connecting':                  ['p(99)<500'],
  },
}

function auth(tok) {
  return { 'Content-Type': 'application/json', Authorization: `Bearer ${tok}` }
}

export function quizFlow() {
  const uid = `${__VU}-${__ITER}`
  let r = http.post(`${BASE}/api/session`,
    JSON.stringify({ nickname: `压${uid}`.slice(0, 16), device_fp: `k6-fp-${uid}` }),
    { headers: { 'Content-Type': 'application/json' } })
  check(r, { 'session 200': (x) => x.status === 200 })
  if (r.status !== 200) { sleep(1); return }
  const tok = r.json().token

  r = http.post(`${BASE}/api/quiz/round`, JSON.stringify({ difficulty: '全部' }),
    { headers: auth(tok) })
  check(r, { 'round 200': (x) => x.status === 200 })
  if (r.status !== 200) { sleep(1); return }
  const qs = (r.json().questions || [])

  for (const q of qs) {
    const a = http.post(`${BASE}/api/answer`,
      JSON.stringify({ request_id: `k6-${uid}-${q.id}`, question_id: q.id, selected: 0 }),
      { headers: auth(tok), tags: { name: 'answer' } })
    check(a, { 'answer 200': (x) => x.status === 200 })
  }

  r = http.get(`${BASE}/api/me`, { headers: auth(tok) })
  check(r, { 'me 200': (x) => x.status === 200 })
  sleep(0.5)
}

export function rankPoll() {
  const r = http.get(`${BASE}/api/rank?top=99`, { tags: { name: 'rank' } })
  check(r, { 'rank 200': (x) => x.status === 200 })
  sleep(1)
}

export function wsConn() {
  const res = ws.connect(`${WS_BASE}/ws`, null, (socket) => {
    socket.on('open', () => {
      socket.setTimeout(() => socket.close(), 8000)
    })
  })
  check(res, { 'ws 101': (r) => r && r.status === 101 })
}
