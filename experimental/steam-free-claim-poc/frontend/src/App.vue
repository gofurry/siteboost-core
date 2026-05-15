<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import {
  ListFreebies,
  MarkFreebieStatus,
  OpenSteamLogin,
  OpenSteamSearch,
  OpenStorePage,
  RefreshFreebies
} from '../wailsjs/go/main/App'

interface Freebie {
  appID: string
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

interface FreebieSnapshot {
  items: Freebie[]
  total: number
  todoCount: number
  claimedCount: number
  skippedCount: number
  failedCount: number
  lastRefreshAt: string
  sourceURL: string
}

const snapshot = ref<FreebieSnapshot>({
  items: [],
  total: 0,
  todoCount: 0,
  claimedCount: 0,
  skippedCount: 0,
  failedCount: 0,
  lastRefreshAt: '',
  sourceURL: ''
})
const loading = ref(false)
const message = ref('启动后先刷新一次，获取当前 Steam Store 中 100% 折扣且原价非零的候选。')
const error = ref('')

const visibleItems = computed(() => snapshot.value.items)
const lastRefreshLabel = computed(() => {
  if (!snapshot.value.lastRefreshAt) return '尚未刷新'
  return new Date(snapshot.value.lastRefreshAt).toLocaleString()
})

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    todo: '待处理',
    claimed: '已领取',
    skipped: '已跳过',
    failed: '失败'
  }
  return labels[status] ?? status
}

async function loadLocal() {
  try {
    snapshot.value = await ListFreebies()
  } catch (err) {
    error.value = String(err)
  }
}

async function refresh() {
  loading.value = true
  error.value = ''
  message.value = '正在从 Steam Store 搜索当前免费促销候选...'
  try {
    snapshot.value = await RefreshFreebies()
    message.value = snapshot.value.total
      ? `刷新完成，共发现 ${snapshot.value.total} 个候选。`
      : '刷新完成，当前没有发现符合条件的候选。'
  } catch (err) {
    error.value = String(err)
    message.value = '刷新失败，可以打开源页面手动核对。'
  } finally {
    loading.value = false
  }
}

async function mark(item: Freebie, status: string) {
  error.value = ''
  try {
    snapshot.value = await MarkFreebieStatus(item.appID, status, '')
  } catch (err) {
    error.value = String(err)
  }
}

async function openStore(item: Freebie) {
  error.value = ''
  try {
    await OpenStorePage(item.appID)
  } catch (err) {
    error.value = String(err)
  }
}

async function openLogin() {
  error.value = ''
  try {
    await OpenSteamLogin()
  } catch (err) {
    error.value = String(err)
  }
}

async function openSource() {
  error.value = ''
  try {
    await OpenSteamSearch()
  } catch (err) {
    error.value = String(err)
  }
}

onMounted(loadLocal)
</script>

<template>
  <main class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark">S</span>
        <div>
          <p class="eyebrow">SteamScope POC</p>
          <h1>每日喜加一</h1>
        </div>
      </div>

      <section class="metric-stack" aria-label="领取状态统计">
        <div class="metric">
          <span>待处理</span>
          <strong>{{ snapshot.todoCount }}</strong>
        </div>
        <div class="metric">
          <span>已领取</span>
          <strong>{{ snapshot.claimedCount }}</strong>
        </div>
        <div class="metric">
          <span>已跳过</span>
          <strong>{{ snapshot.skippedCount }}</strong>
        </div>
      </section>

      <div class="sidebar-actions">
        <button class="primary" :disabled="loading" @click="refresh">
          {{ loading ? '刷新中' : '刷新候选' }}
        </button>
        <button @click="openLogin">打开 Steam 登录</button>
        <button @click="openSource">打开搜索源</button>
      </div>

      <p class="fine-print">
        只打开官方商店页面，由你手动确认领取；本实验不保存密码、不导入令牌、不自动批量请求。
      </p>
    </aside>

    <section class="workspace">
      <header class="toolbar">
        <div>
          <p class="eyebrow">Source: Steam Store search</p>
          <h2>候选游戏</h2>
        </div>
        <div class="refresh-meta">
          <span>上次刷新</span>
          <strong>{{ lastRefreshLabel }}</strong>
        </div>
      </header>

      <p class="message" :class="{ error: error }">
        {{ error || message }}
      </p>

      <div v-if="visibleItems.length" class="list" aria-label="免费领取候选列表">
        <article v-for="item in visibleItems" :key="item.appID" class="row">
          <img :src="item.capsuleURL" :alt="item.title" class="capsule" />
          <div class="row-main">
            <div class="title-line">
              <h3>{{ item.title }}</h3>
              <span class="status" :data-status="item.status">{{ statusLabel(item.status) }}</span>
            </div>
            <div class="meta-line">
              <span>App {{ item.appID }}</span>
              <span>{{ item.released || '未知发行日' }}</span>
              <span>{{ item.originalPrice }} -> {{ item.finalPrice || '$0.00' }}</span>
              <span>{{ item.discount }}</span>
            </div>
          </div>
          <div class="row-actions">
            <button class="primary" @click="openStore(item)">打开领取页</button>
            <button @click="mark(item, 'claimed')">已领取</button>
            <button @click="mark(item, 'skipped')">跳过</button>
            <button @click="mark(item, 'failed')">失败</button>
          </div>
        </article>
      </div>

      <div v-else class="empty-state">
        <h3>还没有候选</h3>
        <p>点击刷新候选后，这里会列出当前 Steam Store 搜索到的 100% 折扣游戏。</p>
      </div>
    </section>
  </main>
</template>
