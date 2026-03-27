<template>
  <div class="dm-layout">
      <!-- Sidebar: conversations -->
      <aside class="dm-sidebar">
        <div class="dm-sidebar-header">
          <h2>{{ $t('dm.title') }}</h2>
          <button class="btn btn-primary btn-sm" @click="showNewConv = !showNewConv">+</button>
        </div>

        <!-- New conversation: user search -->
        <div v-if="showNewConv" class="new-conv">
          <input
            class="form-input"
            v-model="userSearch"
            :placeholder="$t('dm.search_users')"
            @input="filterUsers"
          />
          <div class="user-list">
            <div
              v-for="u in filteredUsers"
              :key="u.id"
              class="user-item"
              @click="openConversation(u)"
            >
              <div class="user-avatar">{{ initials(u) }}</div>
              <div>
                <div class="user-name">{{ u.display_name || u.username }}</div>
                <div class="user-handle">@{{ u.username }}</div>
              </div>
            </div>
          </div>
        </div>

        <div class="conv-list">
          <div
            v-for="conv in conversations"
            :key="conv.user_id"
            :class="['conv-item', { active: activeUserId === conv.user_id }]"
            @click="openConversationById(conv)"
          >
            <div class="user-avatar">{{ convInitials(conv) }}</div>
            <div>
              <div class="user-name">{{ conv.display_name || conv.username }}</div>
              <div class="user-handle">@{{ conv.username }}</div>
            </div>
          </div>
          <div v-if="!conversations.length" class="empty-convs">
            {{ $t('dm.no_conversations') }}
          </div>
        </div>
      </aside>

      <!-- Chat area -->
      <main class="dm-main">
        <div v-if="!activeUser" class="dm-empty">
          <p>{{ $t('dm.select_conversation') }}</p>
        </div>

        <template v-else>
          <div class="dm-chat-header">
            <div class="user-avatar">{{ initials(activeUser) }}</div>
            <div>
              <div class="user-name">{{ activeUser.display_name || activeUser.username }}</div>
              <div class="user-handle">@{{ activeUser.username }}</div>
            </div>
          </div>

          <div class="dm-messages" ref="messagesEl">
            <div
              v-for="msg in messages"
              :key="msg.id"
              :class="['dm-msg', { 'dm-msg-own': msg.sender_id === auth.user?.id, deleted: msg.is_deleted }]"
            >
              <div class="dm-msg-body">
                <span v-if="msg.is_deleted" class="deleted-text">{{ $t('chat.deleted') }}</span>
                <span v-else>{{ msg.body }}</span>
                <span v-if="msg.is_edited && !msg.is_deleted" class="edited-badge"> ({{ $t('chat.edited') }})</span>
              </div>
              <div class="dm-msg-meta">
                {{ formatTime(msg.created_at) }}
                <button
                  v-if="msg.sender_id === auth.user?.id && !msg.is_deleted"
                  class="delete-btn"
                  @click="deleteMsg(msg)"
                >✕</button>
              </div>
            </div>
          </div>

          <form class="dm-input-area" @submit.prevent="send">
            <input
              class="form-input dm-input"
              v-model="newMessage"
              :placeholder="$t('chat.placeholder')"
              :disabled="sending"
            />
            <button type="submit" class="btn btn-primary" :disabled="!newMessage.trim() || sending">
              {{ $t('chat.send') }}
            </button>
          </form>
        </template>
      </main>
  </div>
</template>

<script setup>
import { ref, nextTick, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { messagesApi } from '@/api/messages'
import { useDateFormat } from '@/composables/useDateFormat'

const route = useRoute()
const auth = useAuthStore()
const { formatTime } = useDateFormat()
const ui = useUIStore()

const conversations = ref([])
const allUsers = ref([])
const filteredUsers = ref([])
const userSearch = ref('')
const showNewConv = ref(false)
const activeUser = ref(null)
const activeUserId = ref(null)
const messages = ref([])
const newMessage = ref('')
const sending = ref(false)
const messagesEl = ref(null)

onMounted(async () => {
  const [convRes, userRes] = await Promise.all([
    messagesApi.listConversations(),
    messagesApi.listUsers()
  ])
  conversations.value = convRes.data
  allUsers.value = userRes.data.filter(u => u.id !== auth.user?.id)
  filteredUsers.value = allUsers.value

  // Auto-open conversation when navigated from sidebar user click
  const targetId = route.query.user ? Number(route.query.user) : null
  if (targetId) {
    const target = allUsers.value.find(u => u.id === targetId)
    if (target) openConversation(target)
  }
})

function filterUsers() {
  const q = userSearch.value.toLowerCase()
  filteredUsers.value = allUsers.value.filter(u =>
    u.username.toLowerCase().includes(q) ||
    (u.display_name || '').toLowerCase().includes(q)
  )
}

function initials(u) {
  const name = u.display_name || u.username || '?'
  return name.slice(0, 2).toUpperCase()
}
function convInitials(c) {
  const name = c.display_name || c.username || '?'
  return name.slice(0, 2).toUpperCase()
}

async function openConversation(user) {
  activeUser.value = user
  activeUserId.value = user.id
  showNewConv.value = false
  userSearch.value = ''
  filteredUsers.value = allUsers.value
  await loadMessages(user.id)
}

async function openConversationById(conv) {
  activeUser.value = { id: conv.user_id, username: conv.username, display_name: conv.display_name, avatar_url: conv.avatar_url }
  activeUserId.value = conv.user_id
  await loadMessages(conv.user_id)
}

async function loadMessages(userId) {
  try {
    const { data } = await messagesApi.listMessages(userId)
    messages.value = data
    await nextTick()
    scrollToBottom()
  } catch (e) {
    ui.error('Failed to load messages')
  }
}

function scrollToBottom() {
  if (messagesEl.value) {
    messagesEl.value.scrollTop = messagesEl.value.scrollHeight
  }
}

async function send() {
  const body = newMessage.value.trim()
  if (!body || !activeUser.value) return
  sending.value = true
  try {
    const { data } = await messagesApi.sendMessage(activeUser.value.id, { body })
    messages.value.push(data)
    newMessage.value = ''
    // Ensure conversation appears in sidebar
    if (!conversations.value.find(c => c.user_id === activeUser.value.id)) {
      conversations.value.unshift({
        user_id: activeUser.value.id,
        username: activeUser.value.username,
        display_name: activeUser.value.display_name,
        avatar_url: activeUser.value.avatar_url
      })
    }
    await nextTick()
    scrollToBottom()
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to send message')
  } finally {
    sending.value = false
  }
}

async function deleteMsg(msg) {
  try {
    await messagesApi.deleteMessage(activeUser.value.id, msg.id)
    msg.is_deleted = true
  } catch {
    ui.error('Failed to delete message')
  }
}
</script>

<style scoped>
.dm-layout { flex: 1; display: flex; overflow: hidden; }

.dm-sidebar {
  width: 280px;
  flex-shrink: 0;
  border-right: 1px solid var(--color-border);
  background: var(--color-surface);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.dm-sidebar-header {
  padding: 16px;
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.dm-sidebar-header h2 { font-size: 15px; font-weight: 600; }

.new-conv { padding: 12px; border-bottom: 1px solid var(--color-border); }
.user-list { margin-top: 8px; max-height: 200px; overflow-y: auto; }
.user-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}
.user-item:hover { background: var(--color-bg); }

.conv-list { flex: 1; overflow-y: auto; }
.conv-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  cursor: pointer;
}
.conv-item:hover, .conv-item.active { background: var(--color-bg); }
.empty-convs { padding: 24px 16px; color: var(--color-text-muted); font-size: 13px; text-align: center; }

.user-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: var(--color-primary);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 700;
  flex-shrink: 0;
}
.user-name { font-size: 14px; font-weight: 500; }
.user-handle { font-size: 12px; color: var(--color-text-muted); }

.dm-main { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.dm-empty { flex: 1; display: flex; align-items: center; justify-content: center; color: var(--color-text-muted); }

.dm-chat-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 20px;
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
}

.dm-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.dm-msg {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  max-width: 70%;
}
.dm-msg-own { align-self: flex-end; align-items: flex-end; }

.dm-msg-body {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  border-bottom-left-radius: 4px;
  padding: 8px 12px;
  font-size: 14px;
  word-break: break-word;
}
.dm-msg-own .dm-msg-body {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
  border-bottom-left-radius: 12px;
  border-bottom-right-radius: 4px;
}

.dm-msg.deleted .dm-msg-body { opacity: 0.5; font-style: italic; }
.deleted-text { color: var(--color-text-muted); }
.edited-badge { font-size: 11px; opacity: 0.7; }

.dm-msg-meta {
  font-size: 11px;
  color: var(--color-text-muted);
  margin-top: 2px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.delete-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--color-text-muted);
  font-size: 11px;
  padding: 0;
}
.delete-btn:hover { color: var(--color-danger); }

.dm-input-area {
  display: flex;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid var(--color-border);
  background: var(--color-surface);
}
.dm-input { flex: 1; }
</style>
