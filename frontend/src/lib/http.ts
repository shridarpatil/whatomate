/**
 * Minimal fetch-based HTTP client. Supports the small slice of the legacy client API that the
 * app relies on: `res.data`, `res.status`, and `err.response?.{status,data}`.
 */

export interface HttpConfig {
  params?: Record<string, unknown>
  headers?: Record<string, string>
  responseType?: 'json' | 'blob' | 'arraybuffer' | 'text'
  timeout?: number
  signal?: AbortSignal
}

export interface RequestConfig extends HttpConfig {
  method: string
  url: string
  data?: unknown
  _retry?: boolean
}

export interface HttpResponse<T = any> {
  data: T
  status: number
  headers: Headers
}

export class HttpError<T = any> extends Error {
  response?: { status: number; data: T }
  config: RequestConfig
  isHttpError = true as const

  constructor(message: string, config: RequestConfig, response?: { status: number; data: T }) {
    super(message)
    this.name = 'HttpError'
    this.config = config
    this.response = response
  }
}

export function isHttpError(e: unknown): e is HttpError {
  return e instanceof HttpError || (typeof e === 'object' && e !== null && (e as any).isHttpError === true)
}

export interface HttpClient {
  request<T = any>(c: RequestConfig): Promise<HttpResponse<T>>
  get<T = any>(url: string, c?: HttpConfig): Promise<HttpResponse<T>>
  delete<T = any>(url: string, c?: HttpConfig): Promise<HttpResponse<T>>
  post<T = any>(url: string, data?: unknown, c?: HttpConfig): Promise<HttpResponse<T>>
  put<T = any>(url: string, data?: unknown, c?: HttpConfig): Promise<HttpResponse<T>>
  patch<T = any>(url: string, data?: unknown, c?: HttpConfig): Promise<HttpResponse<T>>
  baseURL: string
}

export interface HttpClientOptions {
  baseURL: string
  timeout: number
  onRequest?: (c: RequestConfig) => void
  onError?: (
    e: HttpError,
    retry: (c: RequestConfig) => Promise<HttpResponse>
  ) => Promise<HttpResponse>
}

function buildQuery(params?: Record<string, unknown>): string {
  if (!params) return ''
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null) continue
    sp.append(k, String(v))
  }
  const qs = sp.toString()
  return qs ? `?${qs}` : ''
}

async function parseBody(res: Response, responseType?: HttpConfig['responseType']): Promise<any> {
  if (responseType === 'blob') return res.blob()
  if (responseType === 'arraybuffer') return res.arrayBuffer()
  const text = await res.text()
  if (text === '') return null
  if (responseType === 'text') return text
  if ((res.headers.get('content-type') || '').includes('json')) {
    try {
      return JSON.parse(text)
    } catch {
      return text
    }
  }
  return text
}

export function createHttpClient(opts: HttpClientOptions): HttpClient {
  async function send<T>(config: RequestConfig): Promise<HttpResponse<T>> {
    opts.onRequest?.(config)

    const headers = new Headers(config.headers)
    let body: BodyInit | undefined
    const d = config.data
    if (d !== undefined && d !== null) {
      if (d instanceof FormData) {
        // Let the browser set the multipart boundary.
        headers.delete('Content-Type')
        body = d
      } else if (typeof d === 'string' || d instanceof Blob || d instanceof URLSearchParams || d instanceof ArrayBuffer) {
        body = d as BodyInit
      } else {
        body = JSON.stringify(d)
        if (!headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
      }
    }

    const controller = new AbortController()
    const timeoutMs = config.timeout ?? opts.timeout
    let timedOut = false
    const timer = setTimeout(() => {
      timedOut = true
      controller.abort()
    }, timeoutMs)
    const onCallerAbort = () => controller.abort()
    if (config.signal) {
      if (config.signal.aborted) controller.abort()
      else config.signal.addEventListener('abort', onCallerAbort, { once: true })
    }

    try {
      const url = `${opts.baseURL}${config.url}${buildQuery(config.params)}`
      let res: Response
      try {
        res = await fetch(url, {
          method: config.method.toUpperCase(),
          headers,
          body,
          credentials: 'include',
          signal: controller.signal,
        })
      } catch (err) {
        const msg = timedOut ? `timeout of ${timeoutMs}ms exceeded` : (err as Error)?.message || 'Network Error'
        throw new HttpError(msg, config)
      }

      let data: any
      try {
        data = await parseBody(res, config.responseType)
      } catch (err) {
        if (res.ok) {
          throw new HttpError(timedOut ? `timeout of ${timeoutMs}ms exceeded` : (err as Error)?.message || 'Network Error', config)
        }
        data = null
      }
      if (!res.ok) {
        throw new HttpError(`Request failed with status code ${res.status}`, config, { status: res.status, data })
      }
      return { data, status: res.status, headers: res.headers }
    } finally {
      clearTimeout(timer)
      config.signal?.removeEventListener('abort', onCallerAbort)
    }
  }

  async function request<T = any>(config: RequestConfig): Promise<HttpResponse<T>> {
    try {
      return await send<T>(config)
    } catch (e) {
      if (opts.onError && isHttpError(e)) {
        return opts.onError(e, (c) => request(c)) as Promise<HttpResponse<T>>
      }
      throw e
    }
  }

  return {
    baseURL: opts.baseURL,
    request,
    get: (url, c) => request({ ...c, method: 'get', url }),
    delete: (url, c) => request({ ...c, method: 'delete', url }),
    post: (url, data, c) => request({ ...c, method: 'post', url, data }),
    put: (url, data, c) => request({ ...c, method: 'put', url, data }),
    patch: (url, data, c) => request({ ...c, method: 'patch', url, data }),
  }
}
