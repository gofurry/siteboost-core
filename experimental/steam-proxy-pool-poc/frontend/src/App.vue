<script setup lang="ts">
import { computed, ref } from 'vue'
import ProbePanel from './ProbePanel.vue'
import ProbeRow from './ProbeRow.vue'

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

type DiagnosisReport = {
  direct: ProbeResult
  system?: ProbeResult
  localCandidates: ProbeResult[]
  manual?: ProbeResult
  recommended?: ProbeResult
  summary: string
}

const report = ref<DiagnosisReport | null>(null)
const manualProxy = ref('')
const manualResult = ref<ProbeResult | null>(null)
const busy = ref(false)
const scanBusy = ref(false)
const error = ref('')

const recommendedLabel = computed(() => {
  const candidate = report.value?.recommended?.candidate
  if (!candidate) return '暂无可用出口'
  if (candidate.source === 'direct') return '直连'
  if (candidate.source === 'system_proxy') return '系统代理'
  if (candidate.source === 'common_port') return candidate.address
  if (candidate.source === 'manual') return candidate.proxyUrl
  return candidate.name
})

async function callBackend<T>(name: string, ...args: unknown[]): Promise<T> {
  const app = window.go?.main?.App
  const fn = (app as Record<string, (...args: unknown[]) => Promise<T>> | undefined)?.[name]
  if (!fn) throw new Error('Wails backend is not ready')
  return fn(...args)
}

async function runDiagnosis() {
  error.value = ''
  busy.value = true
  manualResult.value = null
  try {
    report.value = await callBackend<DiagnosisReport>('RunDiagnosis', manualProxy.value)
  } catch (err) {
    setError(err)
  } finally {
    busy.value = false
  }
}

async function scanLocal() {
  error.value = ''
  scanBusy.value = true
  try {
    const results = await callBackend<ProbeResult[]>('ScanLocalProxies')
    if (report.value) {
      report.value.localCandidates = results
    } else {
      report.value = {
        direct: emptyProbe('Direct', 'direct'),
        localCandidates: results,
        summary: '已完成本机端口扫描。'
      }
    }
  } catch (err) {
    setError(err)
  } finally {
    scanBusy.value = false
  }
}

async function testManual() {
  error.value = ''
  manualResult.value = null
  try {
    manualResult.value = await callBackend<ProbeResult>('TestManualProxy', manualProxy.value)
  } catch (err) {
    setError(err)
  }
}

function useRecommended() {
  const candidate = report.value?.recommended?.candidate
  if (!candidate) return
  manualProxy.value = candidate.proxyUrl || candidate.address
}

function emptyProbe(name: string, source: string): ProbeResult {
  return {
    candidate: { name, address: '', proxyUrl: '', protocol: 'unknown', source },
    ok: false,
    checks: [],
    durationMs: 0
  }
}

function setError(err: unknown) {
  error.value = err instanceof Error ? err.message : String(err)
}

function sourceLabel(source: string) {
  const labels: Record<string, string> = {
    direct: '直连',
    system_proxy: '系统',
    common_port: '本机',
    manual: '手动'
  }
  return labels[source] ?? source
}

function protocolLabel(protocol: string) {
  if (protocol === 'mixed') return 'HTTP/Mixed'
  if (protocol === 'socks5') return 'SOCKS5'
  if (protocol === 'unknown') return '未知'
  return protocol.toUpperCase()
}
</script>

<template>
  <main class="shell">
    <section class="topbar">
      <div>
        <p class="eyebrow">SteamScope Network POC</p>
        <h1>Steam 网络连通性诊断</h1>
        <p class="subtle">检测直连、系统代理、本机代理端口和手动代理，推荐一个可用请求出口。</p>
      </div>
      <div class="recommendation" :class="{ ok: report?.recommended }">
        <span>推荐出口</span>
        <strong>{{ recommendedLabel }}</strong>
      </div>
    </section>

    <section class="toolbar">
      <button :disabled="busy" @click="runDiagnosis">{{ busy ? '诊断中...' : '开始诊断' }}</button>
      <button :disabled="scanBusy" class="secondary" @click="scanLocal">{{ scanBusy ? '扫描中...' : '重新扫描本机端口' }}</button>
      <button class="secondary" :disabled="!report?.recommended" @click="useRecommended">使用推荐出口</button>
      <label>
        手动代理
        <input v-model="manualProxy" placeholder="127.0.0.1:7897 / socks5://127.0.0.1:1080" />
      </label>
      <button class="secondary" @click="testManual">测试手动代理</button>
    </section>

    <section class="scope-note">
      <div>
        <h2>诊断口径</h2>
        <p>这里检测的是 SteamScope 当前进程的请求出口，不代表整台电脑、Steam 客户端或浏览器的绝对连通性。</p>
      </div>
      <ul>
        <li>如果 Steam 客户端或浏览器可用，但本页直连失败，通常说明加速器没有接管 SteamScope 进程。</li>
        <li>可尝试把 SteamScope 加入加速器、开启 TUN/全局模式，或选择一个可用的本机 HTTP/SOCKS 代理端口。</li>
      </ul>
    </section>

    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="report" class="summary">{{ report.summary }}</p>

    <section class="grid">
      <ProbePanel v-if="report" title="直连检测" :result="report.direct" />
      <ProbePanel v-if="report?.system" title="系统代理" :result="report.system" />
      <ProbePanel v-if="manualResult" title="手动代理测试" :result="manualResult" />
      <ProbePanel v-else-if="report?.manual" title="手动代理测试" :result="report.manual" />
    </section>

    <section class="local">
      <div class="section-title">
        <h2>本机代理端口</h2>
        <span>{{ report?.localCandidates.length ?? 0 }} 个候选</span>
      </div>
      <div class="rows">
        <ProbeRow v-for="item in report?.localCandidates ?? []" :key="item.candidate.name + item.candidate.proxyUrl" :result="item" />
      </div>
    </section>

    <section class="risk">
      本工具不提供代理节点，只检测你机器上已有的网络出口。HTTP 公共代理不适合承载敏感登录态；如后续用于真实账号能力，请优先使用可信本机代理。
    </section>
  </main>
</template>
