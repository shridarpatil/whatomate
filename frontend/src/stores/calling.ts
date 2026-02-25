import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { callLogsService, ivrFlowsService, callTransfersService, outgoingCallsService, type CallLog, type IVRFlow, type CallTransfer } from '@/services/api'

export const useCallingStore = defineStore('calling', () => {
  // Call Logs state
  const callLogs = ref<CallLog[]>([])
  const callLogsTotal = ref(0)
  const callLogsLoading = ref(false)
  const callLogsPage = ref(1)
  const callLogsLimit = ref(20)

  // IVR Flows state
  const ivrFlows = ref<IVRFlow[]>([])
  const ivrFlowsTotal = ref(0)
  const ivrFlowsLoading = ref(false)

  // Current items
  const currentCallLog = ref<CallLog | null>(null)
  const currentIVRFlow = ref<IVRFlow | null>(null)

  // Call Transfer state
  const waitingTransfers = ref<CallTransfer[]>([])
  const activeTransfer = ref<CallTransfer | null>(null)
  const localStream = ref<MediaStream | null>(null)
  const peerConnection = ref<RTCPeerConnection | null>(null)
  const isOnCall = ref(false)
  const callDuration = ref(0)
  const isMuted = ref(false)
  let durationTimer: number | null = null

  // Outgoing call state
  const outgoingCallLogId = ref<string | null>(null)
  const outgoingCallStatus = ref<'initiating' | 'ringing' | 'answered' | 'ended' | null>(null)
  const outgoingContactName = ref<string>('')
  const outgoingContactPhone = ref<string>('')

  // Computed
  const activeFlows = computed(() => ivrFlows.value.filter(f => f.is_active && f.is_call_start))
  const isOutgoingCall = computed(() => outgoingCallLogId.value !== null)

  // Call Logs actions
  async function fetchCallLogs(params?: {
    status?: string
    account?: string
    contact_id?: string
    direction?: string
    ivr_flow_id?: string
    phone?: string
    from?: string
    to?: string
    page?: number
    limit?: number
  }) {
    callLogsLoading.value = true
    try {
      const page = params?.page ?? callLogsPage.value
      const limit = params?.limit ?? callLogsLimit.value
      const response = await callLogsService.list({ ...params, page, limit })
      const data = response.data as any
      callLogs.value = data.data?.call_logs ?? data.call_logs ?? []
      callLogsTotal.value = data.data?.total ?? data.total ?? 0
      callLogsPage.value = page
    } catch {
      // Silently handle
    } finally {
      callLogsLoading.value = false
    }
  }

  async function fetchCallLog(id: string) {
    try {
      const response = await callLogsService.get(id)
      const data = response.data as any
      currentCallLog.value = data.data ?? data
      return currentCallLog.value
    } catch {
      return null
    }
  }

  // IVR Flows actions
  async function fetchIVRFlows(params?: { search?: string; page?: number; limit?: number }) {
    ivrFlowsLoading.value = true
    try {
      const response = await ivrFlowsService.list(params)
      const data = response.data as any
      ivrFlows.value = data.data?.ivr_flows ?? data.ivr_flows ?? []
      ivrFlowsTotal.value = data.data?.total ?? data.total ?? 0
    } catch {
      // Silently handle
    } finally {
      ivrFlowsLoading.value = false
    }
  }

  async function fetchIVRFlow(id: string) {
    try {
      const response = await ivrFlowsService.get(id)
      const data = response.data as any
      currentIVRFlow.value = data.data ?? data
      return currentIVRFlow.value
    } catch {
      return null
    }
  }

  async function createIVRFlow(flowData: Parameters<typeof ivrFlowsService.create>[0]) {
    const response = await ivrFlowsService.create(flowData)
    const data = response.data as any
    const flow = data.data ?? data
    ivrFlows.value.unshift(flow)
    return flow
  }

  async function updateIVRFlow(id: string, flowData: Parameters<typeof ivrFlowsService.update>[1]) {
    const response = await ivrFlowsService.update(id, flowData)
    const data = response.data as any
    const updated = data.data ?? data
    const idx = ivrFlows.value.findIndex(f => f.id === id)
    if (idx !== -1) {
      ivrFlows.value[idx] = updated
    }
    return updated
  }

  async function deleteIVRFlow(id: string) {
    await ivrFlowsService.delete(id)
    ivrFlows.value = ivrFlows.value.filter(f => f.id !== id)
  }

  // ICE server config (fetched from backend)
  let cachedICEServers: RTCIceServer[] | null = null

  async function getICEServers(): Promise<RTCIceServer[]> {
    if (cachedICEServers) return cachedICEServers
    try {
      const response = await outgoingCallsService.getICEServers()
      const data = response.data as any
      const servers = data.data?.ice_servers ?? data.ice_servers ?? []
      cachedICEServers = servers.map((s: any) => ({
        urls: s.urls,
        ...(s.username && { username: s.username, credential: s.credential }),
      }))
    } catch {
      cachedICEServers = []
    }
    return cachedICEServers!
  }

  // Call Transfer actions
  async function fetchWaitingTransfers() {
    try {
      const response = await callTransfersService.list({ status: 'waiting' })
      const data = response.data as any
      waitingTransfers.value = data.data?.call_transfers ?? data.call_transfers ?? []
    } catch {
      // Silently handle
    }
  }

  async function acceptTransfer(id: string) {
    // Get microphone access
    let stream: MediaStream
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    } catch {
      throw new Error('Microphone access is required to accept calls')
    }

    localStream.value = stream

    // Create RTCPeerConnection with configured ICE servers
    const iceServers = await getICEServers()
    const pc = new RTCPeerConnection({ iceServers })
    peerConnection.value = pc

    // Add local audio track
    stream.getAudioTracks().forEach(track => {
      pc.addTrack(track, stream)
    })

    // Handle remote audio (caller's voice)
    pc.ontrack = (event) => {
      const audio = new Audio()
      audio.srcObject = event.streams[0]
      audio.play().catch(() => { /* ignore autoplay */ })
    }

    // Clean up when WebRTC connection drops
    pc.onconnectionstatechange = () => {
      if (pc.connectionState === 'disconnected' || pc.connectionState === 'failed' || pc.connectionState === 'closed') {
        cleanup()
      }
    }

    // Create SDP offer
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)

    // Wait for ICE gathering (with 3s timeout to avoid long TURN delays)
    await new Promise<void>((resolve) => {
      if (pc.iceGatheringState === 'complete') {
        resolve()
        return
      }
      const timeout = setTimeout(resolve, 3000)
      pc.onicegatheringstatechange = () => {
        if (pc.iceGatheringState === 'complete') {
          clearTimeout(timeout)
          resolve()
        }
      }
    })

    const sdpOffer = pc.localDescription?.sdp
    if (!sdpOffer) {
      cleanup()
      throw new Error('Failed to generate SDP offer')
    }

    // Send offer to server, get answer
    const response = await callTransfersService.connect(id, sdpOffer)
    const data = response.data as any
    const sdpAnswer = data.data?.sdp_answer ?? data.sdp_answer

    // Set remote description
    await pc.setRemoteDescription(new RTCSessionDescription({
      type: 'answer',
      sdp: sdpAnswer
    }))

    // Transfer is now connected
    const transfer = waitingTransfers.value.find(t => t.id === id)
    if (transfer) {
      activeTransfer.value = { ...transfer, status: 'connected' }
    }
    waitingTransfers.value = waitingTransfers.value.filter(t => t.id !== id)
    isOnCall.value = true
    callDuration.value = 0

    // Start duration timer
    durationTimer = window.setInterval(() => {
      callDuration.value++
    }, 1000)
  }

  // Outgoing call actions
  async function makeOutgoingCall(contactId: string, contactName: string, whatsappAccount: string) {
    // Get microphone access
    let stream: MediaStream
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    } catch {
      throw new Error('Microphone access is required to make calls')
    }

    localStream.value = stream

    // Create RTCPeerConnection with configured ICE servers
    const iceServers = await getICEServers()
    const pc = new RTCPeerConnection({ iceServers })
    peerConnection.value = pc

    // Add local audio track
    stream.getAudioTracks().forEach(track => {
      pc.addTrack(track, stream)
    })

    // Handle remote audio (consumer's voice)
    pc.ontrack = (event) => {
      const audio = new Audio()
      audio.srcObject = event.streams[0]
      audio.play().catch(() => { /* ignore autoplay */ })
    }

    // Clean up when WebRTC connection drops
    pc.onconnectionstatechange = () => {
      if (pc.connectionState === 'disconnected' || pc.connectionState === 'failed' || pc.connectionState === 'closed') {
        cleanup()
      }
    }

    // Create SDP offer
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)

    // Wait for ICE gathering (with 3s timeout to avoid long TURN delays)
    await new Promise<void>((resolve) => {
      if (pc.iceGatheringState === 'complete') {
        resolve()
        return
      }
      const timeout = setTimeout(resolve, 3000)
      pc.onicegatheringstatechange = () => {
        if (pc.iceGatheringState === 'complete') {
          clearTimeout(timeout)
          resolve()
        }
      }
    })

    const sdpOffer = pc.localDescription?.sdp
    if (!sdpOffer) {
      cleanup()
      throw new Error('Failed to generate SDP offer')
    }

    // Send to server
    const response = await outgoingCallsService.initiate({
      contact_id: contactId,
      whatsapp_account: whatsappAccount,
      sdp_offer: sdpOffer,
    })
    const data = response.data as any
    const callLogId = data.data?.call_log_id ?? data.call_log_id
    const sdpAnswer = data.data?.sdp_answer ?? data.sdp_answer

    // Set remote description
    await pc.setRemoteDescription(new RTCSessionDescription({
      type: 'answer',
      sdp: sdpAnswer,
    }))

    // Update state
    outgoingCallLogId.value = callLogId
    outgoingCallStatus.value = 'initiating'
    outgoingContactName.value = contactName
    outgoingContactPhone.value = ''
    isOnCall.value = true
    callDuration.value = 0

    // Start duration timer
    durationTimer = window.setInterval(() => {
      callDuration.value++
    }, 1000)
  }

  async function endCall() {
    if (outgoingCallLogId.value) {
      // Outgoing call hangup
      try {
        await outgoingCallsService.hangup(outgoingCallLogId.value)
      } catch {
        // Best effort
      }
    } else if (activeTransfer.value) {
      // Transfer call hangup
      try {
        await callTransfersService.hangup(activeTransfer.value.id)
      } catch {
        // Best effort
      }
    }
    cleanup()
  }

  function toggleMute() {
    if (localStream.value) {
      const audioTrack = localStream.value.getAudioTracks()[0]
      if (audioTrack) {
        audioTrack.enabled = !audioTrack.enabled
        isMuted.value = !audioTrack.enabled
      }
    }
  }

  function cleanup() {
    if (durationTimer) {
      clearInterval(durationTimer)
      durationTimer = null
    }
    if (peerConnection.value) {
      peerConnection.value.close()
      peerConnection.value = null
    }
    if (localStream.value) {
      localStream.value.getTracks().forEach(t => t.stop())
      localStream.value = null
    }
    isOnCall.value = false
    activeTransfer.value = null
    outgoingCallLogId.value = null
    outgoingCallStatus.value = null
    outgoingContactName.value = ''
    outgoingContactPhone.value = ''
    callDuration.value = 0
    isMuted.value = false
  }

  // WebSocket handler for call events
  function handleCallEvent(type: string, payload: any) {
    switch (type) {
      case 'call_transfer_waiting':
        // Deduplicate: only add if this transfer ID isn't already in the list
        if (!waitingTransfers.value.some(t => t.id === payload.id)) {
          waitingTransfers.value.push(payload as CallTransfer)
        }
        break
      case 'call_transfer_connected':
        // Another agent accepted this transfer — remove from our waiting list
        waitingTransfers.value = waitingTransfers.value.filter(t => t.id !== payload.id)
        break
      case 'call_transfer_completed':
      case 'call_transfer_abandoned':
      case 'call_transfer_no_answer':
        waitingTransfers.value = waitingTransfers.value.filter(t => t.id !== payload.id)
        if (activeTransfer.value?.id === payload.id) {
          cleanup()
        }
        break
      case 'call_ended':
        // If the agent is on a call that just ended, clean up
        if (isOnCall.value) {
          cleanup()
        }
        fetchCallLogs()
        break
      // Outgoing call events
      case 'outgoing_call_ringing':
        outgoingCallStatus.value = 'ringing'
        break
      case 'outgoing_call_answered':
        outgoingCallStatus.value = 'answered'
        break
      case 'outgoing_call_rejected':
        cleanup()
        break
      case 'outgoing_call_ended':
        cleanup()
        break
      default:
        // For regular call events, refresh call logs
        fetchCallLogs()
        break
    }
  }

  return {
    // Call logs
    callLogs,
    callLogsTotal,
    callLogsLoading,
    callLogsPage,
    callLogsLimit,
    currentCallLog,
    fetchCallLogs,
    fetchCallLog,
    // IVR flows
    ivrFlows,
    ivrFlowsTotal,
    ivrFlowsLoading,
    currentIVRFlow,
    activeFlows,
    fetchIVRFlows,
    fetchIVRFlow,
    createIVRFlow,
    updateIVRFlow,
    deleteIVRFlow,
    // Call transfers
    waitingTransfers,
    activeTransfer,
    isOnCall,
    callDuration,
    isMuted,
    fetchWaitingTransfers,
    acceptTransfer,
    endCall,
    toggleMute,
    cleanup,
    // Outgoing calls
    outgoingCallLogId,
    outgoingCallStatus,
    outgoingContactName,
    outgoingContactPhone,
    isOutgoingCall,
    makeOutgoingCall,
    // WS handler
    handleCallEvent
  }
})
