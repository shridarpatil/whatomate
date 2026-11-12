// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

function jsonRes(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

describe('api client', () => {
  let fetchMock: ReturnType<typeof vi.fn>
  let api: typeof import('./api')['api']

  beforeEach(async () => {
    vi.resetModules()
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    localStorage.clear()
    localStorage.setItem('selected_organization_id', 'org-1')
    document.cookie = 'whm_csrf=tok'
    ;({ api } = await import('./api'))
  })
  afterEach(() => vi.unstubAllGlobals())

  const hdr = (i: number) => new Headers(fetchMock.mock.calls[i][1].headers)

  it('adds org header always and CSRF only on mutating', async () => {
    fetchMock.mockImplementation(async () => jsonRes({}))
    await api.get('/a')
    expect(hdr(0).get('X-Organization-ID')).toBe('org-1')
    expect(hdr(0).has('X-CSRF-Token')).toBe(false)
    await api.post('/a', {})
    await api.put('/a', {})
    await api.patch('/a', {})
    await api.delete('/a')
    for (let i = 1; i < 5; i++) {
      expect(hdr(i).get('X-Organization-ID')).toBe('org-1')
      expect(hdr(i).get('X-CSRF-Token')).toBe('tok')
    }
  })

  it('401 refreshes then retries once', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonRes({}, 401))
      .mockResolvedValueOnce(jsonRes({}, 200))
      .mockResolvedValueOnce(jsonRes({ v: 1 }, 200))
    const res = await api.get('/a')
    expect(res.data).toEqual({ v: 1 })
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[1][0]).toContain('/auth/refresh')
  })

  it('concurrent 401s share one refresh', async () => {
    const seen = new Set<string>()
    fetchMock.mockImplementation(async (url: string) => {
      if (url.includes('/auth/refresh')) return jsonRes({})
      if (!seen.has(url)) { seen.add(url); return jsonRes({}, 401) }
      return jsonRes({ ok: 1 })
    })
    const rs = await Promise.all([1, 2, 3, 4, 5].map((i) => api.get(`/r${i}`)))
    expect(rs.every((r) => r.data.ok === 1)).toBe(true)
    const refreshes = fetchMock.mock.calls.filter((c) => String(c[0]).includes('/auth/refresh'))
    expect(refreshes).toHaveLength(1)
  })

  it('auth endpoints never refresh', async () => {
    fetchMock.mockResolvedValue(jsonRes({ message: 'no' }, 401))
    await expect(api.post('/auth/login', {})).rejects.toMatchObject({ response: { status: 401 } })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('failed refresh logs out', async () => {
    localStorage.setItem('user', 'u')
    localStorage.setItem('auth_token', 't')
    localStorage.setItem('refresh_token', 'r')
    fetchMock.mockResolvedValue(jsonRes({}, 401))
    await api.get('/a').catch(() => {})
    expect(localStorage.getItem('user')).toBeNull()
    expect(localStorage.getItem('auth_token')).toBeNull()
    expect(localStorage.getItem('refresh_token')).toBeNull()
    expect(window.location.href.endsWith('/login')).toBe(true)
  })
})
