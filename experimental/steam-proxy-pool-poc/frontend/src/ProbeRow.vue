<script setup lang="ts">
type ProxyCandidate = {
  name: string
  address: string
  proxyUrl: string
  protocol: string
  source: string
}

type ConnectivityCheck = {
  name: string
  target: string
  ok: boolean
  durationMs: number
  error?: string
  httpStatus?: number
  note?: string
}

type ProbeResult = {
  candidate: ProxyCandidate
  ok: boolean
  checks: ConnectivityCheck[]
  durationMs: number
  error?: string
  suggestion?: string
}

defineProps<{
  result: ProbeResult
}>()

function protocolLabel(protocol: string) {
  if (protocol === 'mixed') return 'HTTP/Mixed'
  if (protocol === 'socks5') return 'SOCKS5'
  if (protocol === 'unknown') return '未知'
  return protocol.toUpperCase()
}
</script>

<template>
  <div class="probe-row" :class="{ ok: result.ok }">
    <div class="row-main">
      <strong>{{ result.candidate.address || result.candidate.name }}</strong>
      <span>{{ protocolLabel(result.candidate.protocol) }} · {{ result.durationMs }}ms</span>
    </div>
    <div class="row-status">{{ result.ok ? '可用' : '不可用' }}</div>
    <div class="row-detail">{{ result.error || result.suggestion || 'Steam 检测通过' }}</div>
  </div>
</template>
