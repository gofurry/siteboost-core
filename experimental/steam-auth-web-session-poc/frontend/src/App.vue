<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import QRCode from 'qrcode'
import {
  CancelSteamLogin,
  ClaimFreebie,
  GetSteamAccount,
  GetSteamLoginStatus,
  GetNetworkConfig,
  ListFreebies,
  LogoutSteam,
  MarkFreebieStatus,
  OpenStorePage,
  RefreshFreebies,
  SaveNetworkConfig,
  StartSteamCredentialLogin,
  StartSteamQRLogin,
  SubmitSteamGuardCode,
  TestWebSession
} from '../wailsjs/go/main/App'

type GuardAction = {
  type: string
  detail?: string
}

type Account = {
  steamId: string
  account: string
  loggedIn: boolean
  lastLoginAt?: string
}

type LoginState = {
  loginId: string
  status: string
  steamId?: string
  account?: string
  pollIntervalSecs: number
  validActions?: GuardAction[]
  expiresAt?: string
  safeStatusMessage: string
}

type WebSessionTest = {
  ok: boolean
  steamId?: string
  account?: string
  cookieDomains: number
  communityOk: boolean
  storeOk: boolean
  lastCookieRefreshAt?: string
  message: string
}

type Freebie = {
  appID: string
  packageID?: number
  packageTitle?: string
  title: string
  storeURL: string
  capsuleURL: string
  released: string
  originalPrice: string
  finalPrice: string
  discount: string
  source: string
  status: string
  note: string
  firstSeenAt: string
  lastSeenAt: string
  updatedAt: string
}

type FreebieSnapshot = {
  items: Freebie[]
  total: number
  todoCount: number
  claimedCount: number
  skippedCount: number
  failedCount: number
  lastRefreshAt: string
  sourceURL: string
}

type FreebieClaimResult = {
  ok: boolean
  appID: string
  packageID?: number
  message: string
  cookieDomains: number
  snapshot?: FreebieSnapshot
}

type NetworkConfig = {
  proxyUrl: string
}

const activeTab = ref<'qr' | 'credentials'>('qr')
const account = ref<Account>({ steamId: '', account: '', loggedIn: false })
const login = ref<LoginState | null>(null)
const qrDataUrl = ref('')
const accountName = ref('')
const password = ref('')
const guardCode = ref('')
const selectedGuardType = ref('')
const message = ref('推荐使用 Steam App 扫码登录。账号密码仅用于本次登录，不会落盘。')
const error = ref('')
const busy = ref(false)
const freebieBusy = ref(false)
const claimingAppID = ref('')
const sessionTest = ref<WebSessionTest | null>(null)
const freebies = ref<FreebieSnapshot | null>(null)
const networkConfig = ref<NetworkConfig>({ proxyUrl: '' })
const proxyURL = ref('')
const networkBusy = ref(false)
let pollTimer: number | undefined

const guardActions = computed(() => login.value?.validActions ?? [])
const remoteConfirmation = computed(() => guardActions.value.find((item) => item.type === 'device_confirmation' || item.type === 'email_confirmation'))
const codeActions = computed(() => remoteConfirmation.value ? [] : guardActions.value.filter((item) => item.type === 'email_code' || item.type === 'device_code'))
const needsGuardCode = computed(() => codeActions.value.length > 0)
const statusText = computed(() => {
  const status = login.value?.status
  const labels: Record<string, string> = {
    waiting_qr_scan: '等待扫码',
    waiting_guard_code: '等待验证码',
    waiting_device_confirmation: '等待手机确认',
    waiting_email_confirmation: '等待邮箱确认',
    remote_interaction: '远端交互中',
    polling: '轮询中',
    authenticated: '已登录',
    failed: '失败',
    timeout: '超时',
    canceled: '已取消'
  }
  return status ? labels[status] ?? status : account.value.loggedIn ? '已登录' : '空闲'
})

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    todo: '待领取',
    claimed: '已入库',
    skipped: '已跳过',
    failed: '失败'
  }
  return labels[status] ?? status
}

async function refreshAccount() {
  account.value = await GetSteamAccount()
}

async function loadFreebies() {
  try {
    freebies.value = await ListFreebies()
  } catch {
    freebies.value = null
  }
}

async function loadNetworkConfig() {
  try {
    networkConfig.value = await GetNetworkConfig()
    proxyURL.value = networkConfig.value.proxyUrl
  } catch (err) {
    setError(err)
  }
}

async function saveNetworkConfig() {
  clearError()
  networkBusy.value = true
  try {
    networkConfig.value = await SaveNetworkConfig({ proxyUrl: proxyURL.value })
    proxyURL.value = networkConfig.value.proxyUrl
    message.value = networkConfig.value.proxyUrl ? `网络代理已启用：${networkConfig.value.proxyUrl}` : '网络代理已关闭。'
  } catch (err) {
    setError(err)
  } finally {
    networkBusy.value = false
  }
}

async function useLocalProxy() {
  proxyURL.value = '127.0.0.1:7897'
  await saveNetworkConfig()
}

async function clearProxy() {
  proxyURL.value = ''
  await saveNetworkConfig()
}

async function refreshFreebies() {
  clearError()
  freebieBusy.value = true
  try {
    freebies.value = await RefreshFreebies()
    message.value = `已刷新，找到 ${freebies.value.total} 个候选游戏。`
  } catch (err) {
    setError(err)
  } finally {
    freebieBusy.value = false
  }
}

async function claimFreebie(item: Freebie) {
  clearError()
  claimingAppID.value = item.appID
  try {
    const result: FreebieClaimResult = await ClaimFreebie(item.appID)
    if (result.snapshot) {
      freebies.value = result.snapshot
    }
    message.value = result.message
  } catch (err) {
    setError(err)
  } finally {
    claimingAppID.value = ''
  }
}

async function skipFreebie(item: Freebie) {
  clearError()
  try {
    freebies.value = await MarkFreebieStatus(item.appID, 'skipped', '手动跳过')
  } catch (err) {
    setError(err)
  }
}

async function openStore(item: Freebie) {
  try {
    await OpenStorePage(item.appID)
  } catch (err) {
    setError(err)
  }
}

async function startQR() {
  clearError()
  busy.value = true
  sessionTest.value = null
  try {
    const started = await StartSteamQRLogin()
    login.value = {
      loginId: started.loginId,
      status: started.status,
      pollIntervalSecs: started.pollIntervalSecs,
      validActions: started.validActions,
      expiresAt: started.expiresAt,
      safeStatusMessage: started.safeStatusMessage
    }
    qrDataUrl.value = await QRCode.toDataURL(started.qrChallengeUrl, { width: 220, margin: 1 })
    message.value = started.safeStatusMessage
    schedulePoll()
  } catch (err) {
    setError(err)
  } finally {
    busy.value = false
  }
}

async function startCredential() {
  clearError()
  busy.value = true
  sessionTest.value = null
  try {
    const started = await StartSteamCredentialLogin({
      accountName: accountName.value,
      password: password.value
    })
    password.value = ''
    login.value = started
    message.value = started.safeStatusMessage
    selectedGuardType.value = codeActions.value.find((item) => item.type === 'device_code')?.type
      ?? codeActions.value.find((item) => item.type === 'email_code')?.type
      ?? ''
    schedulePoll()
  } catch (err) {
    password.value = ''
    setError(err)
  } finally {
    busy.value = false
  }
}

async function submitGuard() {
  if (!login.value || !selectedGuardType.value) return
  clearError()
  busy.value = true
  try {
    await SubmitSteamGuardCode({
      loginId: login.value.loginId,
      code: guardCode.value,
      type: selectedGuardType.value
    })
    guardCode.value = ''
    message.value = '验证码已提交，继续等待 Steam 登录结果。'
    schedulePoll()
  } catch (err) {
    setError(err)
  } finally {
    busy.value = false
  }
}

async function pollStatus() {
  if (!login.value) return
  try {
    const next = await GetSteamLoginStatus(login.value.loginId)
    login.value = next
    message.value = next.safeStatusMessage
    if (next.status === 'authenticated') {
      stopPoll()
      await refreshAccount()
    } else if (['failed', 'timeout', 'canceled'].includes(next.status)) {
      stopPoll()
    } else {
      schedulePoll()
    }
  } catch (err) {
    setError(err)
    stopPoll()
  }
}

async function cancelLogin() {
  if (!login.value) return
  clearError()
  try {
    await CancelSteamLogin(login.value.loginId)
    stopPoll()
    login.value.status = 'canceled'
    login.value.safeStatusMessage = '登录已取消。'
  } catch (err) {
    setError(err)
  }
}

async function logout() {
  clearError()
  try {
    await LogoutSteam()
    account.value = { steamId: '', account: '', loggedIn: false }
    sessionTest.value = null
    message.value = '已退出登录，本地 refresh token 已删除。'
  } catch (err) {
    setError(err)
  }
}

async function testSession() {
  clearError()
  busy.value = true
  try {
    sessionTest.value = await TestWebSession()
    message.value = sessionTest.value.message
  } catch (err) {
    setError(err)
  } finally {
    busy.value = false
  }
}

function schedulePoll() {
  stopPoll()
  const seconds = Math.max(login.value?.pollIntervalSecs ?? 2, 1)
  pollTimer = window.setTimeout(pollStatus, seconds * 1000)
}

function stopPoll() {
  if (pollTimer) {
    window.clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

function clearError() {
  error.value = ''
}

function setError(err: unknown) {
  error.value = String(err)
}

onMounted(async () => {
  await loadNetworkConfig()
  await refreshAccount()
  await loadFreebies()
})
onBeforeUnmount(stopPoll)
</script>

<template>
  <main class="shell">
    <section class="sidebar">
      <div class="brand">
        <div class="brand-mark">S</div>
        <div>
          <p class="eyebrow">SteamScope Experimental</p>
          <h1>喜加一闭环验证</h1>
        </div>
      </div>

      <div class="account-panel">
        <p class="eyebrow">当前账号</p>
        <h2>{{ account.loggedIn ? account.account : '未登录' }}</h2>
        <p>{{ account.loggedIn ? account.steamId : 'refresh token 不存在或已退出。' }}</p>
        <button v-if="account.loggedIn" @click="logout">退出登录</button>
      </div>

      <div class="status-panel">
        <p class="eyebrow">登录状态</p>
        <strong>{{ statusText }}</strong>
        <span>{{ login?.safeStatusMessage || message }}</span>
      </div>

      <div class="status-panel">
        <p class="eyebrow">今日喜加一</p>
        <strong>{{ freebies?.total ?? 0 }}</strong>
        <span>待领取 {{ freebies?.todoCount ?? 0 }} · 已入库 {{ freebies?.claimedCount ?? 0 }} · 失败 {{ freebies?.failedCount ?? 0 }}</span>
      </div>

      <div class="network-panel">
        <p class="eyebrow">网络代理</p>
        <input v-model="proxyURL" placeholder="127.0.0.1:7897" />
        <div class="proxy-actions">
          <button :disabled="networkBusy" @click="saveNetworkConfig">保存</button>
          <button :disabled="networkBusy" @click="useLocalProxy">7897</button>
          <button :disabled="networkBusy" @click="clearProxy">关闭</button>
        </div>
        <span>{{ networkConfig.proxyUrl ? `当前：${networkConfig.proxyUrl}` : '当前未启用显式代理。' }}</span>
      </div>

      <p class="fine-print">
        本 POC 仅用于你自己的 Steam 账号。refresh token 使用 Windows DPAPI 加密，Cookie 只存在 Go 后端内存里。
      </p>
    </section>

    <section class="workspace">
      <header class="toolbar">
        <div>
          <p class="eyebrow">Steam Auth WebBrowser</p>
          <h2>登录会话</h2>
        </div>
        <button :disabled="busy || !account.loggedIn" @click="testSession">TestWebSession</button>
      </header>

      <p class="message" :class="{ error: error }">{{ error || message }}</p>

      <div class="tabs">
        <button :class="{ active: activeTab === 'qr' }" @click="activeTab = 'qr'">Steam App 扫码</button>
        <button :class="{ active: activeTab === 'credentials' }" @click="activeTab = 'credentials'">账号密码</button>
      </div>

      <section v-if="activeTab === 'qr'" class="login-surface">
        <div class="qr-block">
          <img v-if="qrDataUrl" :src="qrDataUrl" alt="Steam QR login code" />
          <div v-else class="qr-placeholder">QR</div>
        </div>
        <div class="flow-copy">
          <h3>使用 Steam 手机 App 扫码</h3>
          <p>点击开始后，用 Steam App 扫码并在手机上确认。确认完成后，前端只显示账号摘要。</p>
          <div class="action-row">
            <button class="primary" :disabled="busy" @click="startQR">生成二维码</button>
            <button :disabled="!login" @click="cancelLogin">取消</button>
          </div>
        </div>
      </section>

      <section v-else class="login-surface">
        <form class="credential-form" @submit.prevent="startCredential">
          <label>
            <span>Steam 登录名</span>
            <input v-model="accountName" autocomplete="username" />
          </label>
          <label>
            <span>Steam 密码</span>
            <input v-model="password" type="password" autocomplete="current-password" />
          </label>
          <button class="primary" :disabled="busy">登录</button>
        </form>
        <p class="form-note">密码只在本次请求中短暂进入 Go 后端内存，不写入日志或本地文件。</p>
      </section>

      <section v-if="needsGuardCode" class="guard-panel">
        <div>
          <p class="eyebrow">Steam Guard</p>
          <h3>输入验证码</h3>
        </div>
        <select v-model="selectedGuardType">
          <option v-for="item in codeActions" :key="item.type" :value="item.type">
            {{ item.type === 'email_code' ? '邮箱验证码' : '手机令牌验证码' }} {{ item.detail ? `(${item.detail})` : '' }}
          </option>
        </select>
        <input v-model="guardCode" placeholder="验证码" />
        <button class="primary" :disabled="busy || !guardCode" @click="submitGuard">提交</button>
      </section>

      <section v-else-if="remoteConfirmation" class="guard-panel confirm-only">
        <div>
          <p class="eyebrow">Steam Guard</p>
          <h3>{{ remoteConfirmation.type === 'device_confirmation' ? '等待手机批准' : '等待邮箱确认' }}</h3>
        </div>
        <p>{{ remoteConfirmation.detail || login?.safeStatusMessage || '确认完成后会自动继续。' }}</p>
        <button :disabled="busy" @click="pollStatus">刷新状态</button>
      </section>

      <section v-if="sessionTest" class="result-panel" :class="{ ok: sessionTest.ok }">
        <p class="eyebrow">Web Session Test</p>
        <h3>{{ sessionTest.ok ? '验证通过' : '验证失败' }}</h3>
        <p>{{ sessionTest.message }}</p>
        <span>Community: {{ sessionTest.communityOk ? 'OK' : 'FAIL' }} · Store: {{ sessionTest.storeOk ? 'OK' : 'FAIL' }} · Cookie domains: {{ sessionTest.cookieDomains }}</span>
      </section>

      <section class="freebie-section">
        <header class="section-head">
          <div>
            <p class="eyebrow">Steam Store Search</p>
            <h2>今日喜加一</h2>
          </div>
          <button class="primary" :disabled="freebieBusy" @click="refreshFreebies">
            {{ freebieBusy ? '刷新中' : '刷新列表' }}
          </button>
        </header>

        <div v-if="!freebies || freebies.items.length === 0" class="empty-state">
          <p>还没有本地列表。</p>
          <button :disabled="freebieBusy" @click="refreshFreebies">搜索今日喜加一</button>
        </div>

        <div v-else class="freebie-list">
          <article v-for="item in freebies.items" :key="item.appID" class="freebie-row" :class="item.status">
            <img :src="item.capsuleURL" :alt="item.title" />
            <div class="freebie-main">
              <div class="title-row">
                <h3>{{ item.title }}</h3>
                <span class="status-pill">{{ statusLabel(item.status) }}</span>
              </div>
              <p>{{ item.originalPrice }} → {{ item.finalPrice || 'Free' }} · {{ item.discount }} · App {{ item.appID }}</p>
              <p v-if="item.packageID" class="package-line">Sub {{ item.packageID }} · {{ item.packageTitle }}</p>
              <p v-if="item.note" class="note-line">{{ item.note }}</p>
            </div>
            <div class="freebie-actions">
              <button class="primary" :disabled="!account.loggedIn || !item.packageID || claimingAppID === item.appID" @click="claimFreebie(item)">
                {{ claimingAppID === item.appID ? '领取中' : item.status === 'claimed' ? '重试校验' : '领取入库' }}
              </button>
              <button :disabled="claimingAppID === item.appID" @click="skipFreebie(item)">跳过</button>
              <button @click="openStore(item)">商店</button>
            </div>
          </article>
        </div>
      </section>
    </section>
  </main>
</template>
