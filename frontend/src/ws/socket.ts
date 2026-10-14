export interface WsMessage {
  type: string
  data: any
}

type OnMessage = (msg: WsMessage) => void
type OnStatus = (connected: boolean) => void

export class QuizSocket {
  private ws: WebSocket | null = null
  private closed = false
  private reconnectTimer: number | undefined
  private pingTimer: number | undefined

  constructor(
    private url: string,
    private onMessage: OnMessage,
    private onStatus?: OnStatus,
  ) {}

  connect() {
    if (this.closed) return
    try {
      this.ws = new WebSocket(this.url)
    } catch {
      this.scheduleReconnect()
      return
    }
    this.ws.onopen = () => {
      this.onStatus?.(true)
      this.startHeartbeat()
    }
    this.ws.onmessage = (e) => {
      try {
        this.onMessage(JSON.parse(e.data) as WsMessage)
      } catch {
        /* 忽略坏消息 */
      }
    }
    this.ws.onclose = () => {
      this.onStatus?.(false)
      this.stopHeartbeat()
      this.scheduleReconnect()
    }
    this.ws.onerror = () => {
      this.ws?.close()
    }
  }

  private startHeartbeat() {
    this.stopHeartbeat()
    this.pingTimer = window.setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'ping' }))
      }
    }, 30000)
  }

  private stopHeartbeat() {
    if (this.pingTimer) {
      clearInterval(this.pingTimer)
      this.pingTimer = undefined
    }
  }

  private scheduleReconnect() {
    if (this.closed || this.reconnectTimer) return
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = undefined
      this.connect()
    }, 2000)
  }

  close() {
    this.closed = true
    this.stopHeartbeat()
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = undefined
    }
    this.ws?.close()
  }
}

export function wsURL(): string {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/ws`
}
