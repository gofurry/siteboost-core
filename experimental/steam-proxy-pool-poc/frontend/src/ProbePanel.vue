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
  title: string
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
  <article class="panel" :class="{ ok: result.ok }">
    <header>
      <div>
        <h2>{{ title }}</h2>
        <p>{{ result.candidate.proxyUrl || result.candidate.address || result.candidate.name }}</p>
      </div>
      <strong>{{ result.ok ? '可用' : '失败' }}</strong>
    </header>
    <div class="meta">
      <span>{{ protocolLabel(result.candidate.protocol) }}</span>
      <span>{{ result.durationMs }}ms</span>
    </div>
    <p v-if="result.suggestion" class="suggestion">{{ result.suggestion }}</p>
    <ol>
      <li v-for="check in result.checks" :key="check.name + check.target" :class="{ ok: check.ok }">
        <span>{{ check.name }}</span>
        <small>{{ check.httpStatus || check.note || check.error || `${check.durationMs}ms` }}</small>
      </li>
    </ol>
  </article>
</template>
