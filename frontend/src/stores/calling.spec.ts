import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const connectMock = vi.fn()

vi.mock('@/services/api', () => ({
  callLogsService: {},
  ivrFlowsService: {},
  callTransfersService: { connect: (...args: unknown[]) => connectMock(...args) },
  outgoingCallsService: { getICEServers: vi.fn(async () => ({ data: { ice_servers: [] } })) },
}))
vi.mock('vue-sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
vi.mock('@/i18n', () => ({ i18n: { global: { t: (k: string) => k } } }))

class MockPeerConnection {
  localDescription = { sdp: 'v=0\r\no=- offer' }
  iceGatheringState = 'complete'
  connectionState = 'new'
  ontrack: unknown = null
  onconnectionstatechange: unknown = null
  onicegatheringstatechange: unknown = null
  addTrack = vi.fn()
  createOffer = vi.fn(async () => ({ type: 'offer', sdp: 'v=0\r\no=- offer' }))
  setLocalDescription = vi.fn(async () => undefined)
  setRemoteDescription = vi.fn(async () => undefined)
  close = vi.fn()
}

const waitingTransfer = {
  id: 'transfer-1',
  call_log_id: 'call-log-1',
  caller_phone: '919999999999',
  status: 'waiting',
} as never

beforeEach(() => {
  setActivePinia(createPinia())
  connectMock.mockReset()
  vi.stubGlobal('navigator', {
    mediaDevices: {
      getUserMedia: vi.fn(async () => ({ getAudioTracks: () => [], getTracks: () => [] })),
    },
  })
  vi.stubGlobal('RTCPeerConnection', MockPeerConnection)
  vi.stubGlobal('RTCSessionDescription', class { constructor(public init: unknown) {} })
  vi.stubGlobal('window', { setInterval: vi.fn(() => 1) })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

// The panel is shown while `isConnecting || isOnCall || waitingTransfers.length`.
// acceptTransfer drops the transfer from waitingTransfers on its first lines but
// only sets isOnCall at the very end, so without isConnecting the agent's panel
// disappears for the whole WebRTC setup — several seconds of nothing.
describe('acceptTransfer connecting state', () => {
  it('keeps the panel visible for the whole WebRTC setup', async () => {
    const { useCallingStore } = await import('./calling')
    const store = useCallingStore()
    store.waitingTransfers = [waitingTransfer]

    let resolveConnect: (v: unknown) => void = () => {}
    connectMock.mockReturnValue(new Promise(resolve => { resolveConnect = resolve }))

    const accepting = store.acceptTransfer('transfer-1')
    await vi.waitFor(() => expect(connectMock).toHaveBeenCalled())

    // Mid-flight: the transfer is gone from the waiting list and the call is
    // not yet connected, so isConnecting is the only thing holding the panel up.
    expect(store.waitingTransfers).toHaveLength(0)
    expect(store.isOnCall).toBe(false)
    expect(store.isConnecting).toBe(true)
    expect(store.activeTransfer?.status).toBe('connecting')

    resolveConnect({ data: { sdp_answer: 'v=0\r\no=- answer' } })
    await accepting

    expect(store.isConnecting).toBe(false)
    expect(store.isOnCall).toBe(true)
  })

  it('clears the connecting state when accept fails', async () => {
    const { useCallingStore } = await import('./calling')
    const store = useCallingStore()
    store.waitingTransfers = [waitingTransfer]

    connectMock.mockRejectedValue(new Error('connect failed'))

    await expect(store.acceptTransfer('transfer-1')).rejects.toThrow('connect failed')
    expect(store.isConnecting).toBe(false)
  })
})
