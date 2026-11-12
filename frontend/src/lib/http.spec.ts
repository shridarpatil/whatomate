import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createHttpClient, isHttpError, HttpError } from './http'

const base = 'http://test/api'

function jsonRes(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

describe('http client', () => {
  let fetchMock: ReturnType<typeof vi.fn>
  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('get serializes params', async () => {
    fetchMock.mockResolvedValue(jsonRes({ ok: true }))
    const c = createHttpClient({ baseURL: base, timeout: 1000 })
    const res = await c.get('/x', { params: { page: 2, search: 'a b', empty: undefined, n: null } })
    expect(fetchMock.mock.calls[0][0]).toBe(`${base}/x?page=2&search=a+b`)
    expect(res.data).toEqual({ ok: true })
    expect(res.status).toBe(200)
  })

  it('json body and content-type', async () => {
    fetchMock.mockResolvedValue(jsonRes({}))
    const c = createHttpClient({ baseURL: base, timeout: 1000 })
    await c.post('/x', { a: 1 })
    const init = fetchMock.mock.calls[0][1]
    expect(init.body).toBe('{"a":1}')
    expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
    expect(init.credentials).toBe('include')
  })

  it('FormData passes through', async () => {
    fetchMock.mockResolvedValue(jsonRes({}))
    const c = createHttpClient({ baseURL: base, timeout: 1000 })
    const fd = new FormData()
    fd.append('a', 'b')
    await c.post('/x', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    const init = fetchMock.mock.calls[0][1]
    expect(init.body).toBe(fd)
    expect(new Headers(init.headers).has('Content-Type')).toBe(false)
  })

  it('responseType blob/arraybuffer', async () => {
    const c = createHttpClient({ baseURL: base, timeout: 1000 })
    fetchMock.mockResolvedValueOnce(new Response('abc', { headers: { 'content-type': 'text/csv' } }))
    const b = await c.get('/x', { responseType: 'blob' })
    expect(b.data).toBeInstanceOf(Blob)
    expect(b.headers.get('content-type')).toBe('text/csv')
    fetchMock.mockResolvedValueOnce(new Response('abc'))
    const a = await c.get('/x', { responseType: 'arraybuffer' })
    expect(a.data).toBeInstanceOf(ArrayBuffer)
  })

  it('non-2xx rejects HttpError', async () => {
    fetchMock.mockResolvedValue(jsonRes({ message: 'bad' }, 422))
    const c = createHttpClient({ baseURL: base, timeout: 1000 })
    const e = await c.get('/x').catch((err) => err)
    expect(e).toBeInstanceOf(HttpError)
    expect(isHttpError(e)).toBe(true)
    expect(e.response.status).toBe(422)
    expect(e.response.data.message).toBe('bad')
  })

  it('timeout rejects without response', async () => {
    vi.useFakeTimers()
    fetchMock.mockImplementation((_u: string, init: RequestInit) =>
      new Promise((_r, rej) => init.signal!.addEventListener('abort', () => rej(new DOMException('aborted', 'AbortError')))))
    const c = createHttpClient({ baseURL: base, timeout: 1000 })
    const p = c.get('/x', { timeout: 10 }).catch((err) => err)
    await vi.advanceTimersByTimeAsync(20)
    const e = await p
    expect(isHttpError(e)).toBe(true)
    expect(e.response).toBeUndefined()
  })

  it('onRequest can add headers', async () => {
    fetchMock.mockResolvedValue(jsonRes({}))
    const c = createHttpClient({
      baseURL: base,
      timeout: 1000,
      onRequest: (cfg) => { cfg.headers = { ...cfg.headers, 'X-Test': '1' } },
    })
    await c.get('/x')
    expect(new Headers(fetchMock.mock.calls[0][1].headers).get('X-Test')).toBe('1')
  })
})
