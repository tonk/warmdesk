<template>
  <div class="chat-panel" :class="{ open }">
    <div class="chat-header">
      <span>💬 {{ $t('chat.title') }}</span>
      <button class="btn btn-ghost btn-sm" @click="$emit('close')">✕</button>
    </div>

    <div class="chat-messages" ref="messagesEl" @scroll="onScroll">
      <button v-if="chatStore.hasMore && !chatStore.loading" class="btn btn-ghost btn-sm load-more" @click="loadMore">
        {{ $t('chat.load_more') }}
      </button>
      <div v-if="chatStore.loading" class="chat-loading">
        <div class="spinner"></div>
      </div>
      <div
        v-for="msg in chatStore.messages"
        :key="msg.id"
        :class="['message', { own: msg.user_id === authUser?.id }]"
      >
        <div class="message-avatar" v-if="msg.user_id !== authUser?.id">
          {{ msg.user?.display_name?.slice(0, 2).toUpperCase() }}
        </div>
        <div class="message-bubble">
          <div class="message-meta">
            <strong v-if="msg.user_id !== authUser?.id">{{ msg.user?.display_name || msg.user?.username }}</strong>
            <span class="message-time">{{ formatTime(msg.created_at) }}</span>
            <span v-if="msg.is_edited" class="edited">{{ $t('chat.edited') }}</span>
          </div>
          <div v-if="msg.is_deleted" class="message-deleted">{{ $t('chat.deleted') }}</div>
          <div v-else class="message-body" v-html="renderMarkdown(msg.body)"></div>
        </div>
      </div>
    </div>

    <div class="chat-compose">
      <CardEditor v-model="draft" :placeholder="$t('chat.placeholder')" min-height="60px" />
      <button class="btn btn-primary btn-sm" @click="sendMessage" :disabled="!draft.trim()">
        {{ $t('chat.send') }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import CardEditor from '@/components/board/CardEditor.vue'
import { useChatStore } from '@/stores/chat'
import { useAuthStore } from '@/stores/auth'
import { useDateFormat } from '@/composables/useDateFormat'

const props = defineProps({
  open: Boolean,
  projectSlug: String,
  wsSend: Function
})
defineEmits(['close'])

const chatStore = useChatStore()
const auth = useAuthStore()
const authUser = computed(() => auth.user)
const messagesEl = ref(null)
const draft = ref('')

onMounted(async () => {
  if (props.projectSlug) {
    await chatStore.loadMessages(props.projectSlug)
    scrollToBottom()
  }
})

watch(() => chatStore.messages.length, () => nextTick(scrollToBottom))
watch(() => props.open, (val) => { if (val) nextTick(scrollToBottom) })

function scrollToBottom() {
  if (messagesEl.value) messagesEl.value.scrollTop = messagesEl.value.scrollHeight
}

async function loadMore() {
  const firstId = chatStore.messages[0]?.id
  await chatStore.loadMessages(props.projectSlug, firstId)
}

function onScroll() {
  // auto-load could be added here
}

function sendMessage() {
  if (!draft.value.trim()) return
  props.wsSend?.('chat.send', { body: draft.value })
  draft.value = ''
}

function renderMarkdown(text) {
  return DOMPurify.sanitize(marked.parse(text || ''))
}

const { formatTime } = useDateFormat()
</script>

<style scoped>
.chat-panel {
  position: fixed;
  right: 0;
  top: 56px;
  bottom: 0;
  width: 340px;
  background: #fff;
  border-left: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  transform: translateX(100%);
  transition: transform .25s ease;
  z-index: 50;
  box-shadow: -4px 0 12px rgba(0,0,0,.05);
}
.chat-panel.open { transform: translateX(0); }

.chat-header {
  padding: 12px 16px;
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
  font-size: 14px;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.message {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}
.message.own { flex-direction: row-reverse; }

.message-avatar {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: var(--color-primary);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.message-bubble {
  max-width: 80%;
  background: #f1f5f9;
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  font-size: 13px;
}
.message.own .message-bubble { background: var(--color-primary); color: #fff; }

.message-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
  font-size: 11px;
  color: var(--color-text-muted);
}
.message.own .message-meta { color: rgba(255,255,255,.7); justify-content: flex-end; }
.message-deleted { font-style: italic; opacity: .6; }
.edited { font-style: italic; }

.message-body :deep(p) { margin: 0; }
.message-body :deep(code) { background: rgba(0,0,0,.1); padding: 1px 3px; border-radius: 3px; }

.chat-loading { display: flex; justify-content: center; padding: 8px; }

.load-more { align-self: center; font-size: 12px; }

.chat-compose {
  padding: 10px;
  border-top: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.chat-compose .btn { align-self: flex-end; }
</style>
