<template>
  <BaseModal :title="$t('board.edit_card')" @close="$emit('close')" :resizable="true" style="--modal-width: 700px">
    <div class="card-detail">
      <div class="form-group">
        <label class="form-label">{{ $t('board.card_title') }}</label>
        <input class="form-input" v-model="form.title" />
      </div>

      <div class="form-group">
        <label class="form-label">{{ $t('board.description') }}</label>
        <CardEditor v-model="form.description" />
      </div>

      <div class="detail-row">
        <div class="form-group half">
          <label class="form-label">{{ $t('board.priority') }}</label>
          <select class="form-input" v-model="form.priority">
            <option v-for="p in priorities" :key="p" :value="p">{{ $t(`board.priorities.${p}`) }}</option>
          </select>
        </div>
        <div class="form-group half">
          <label class="form-label">{{ $t('board.due_date') }}</label>
          <input class="form-input" type="date" v-model="form.due_date" />
          <span v-if="form.due_date" class="form-hint">{{ formatDate(form.due_date) }}</span>
        </div>
      </div>

      <div class="form-group">
        <label class="form-label">{{ $t('board.assignee') }}</label>
        <select class="form-input" v-model="form.assignee_id">
          <option :value="null">—</option>
          <option v-for="m in members" :key="m.user.id" :value="m.user.id">{{ m.user.display_name || m.user.username }}</option>
        </select>
      </div>

      <div class="form-group">
        <label class="form-label">{{ $t('board.labels') }}</label>
        <div class="labels-picker">
          <span
            v-for="label in labels"
            :key="label.id"
            class="label-chip"
            :class="{ active: hasLabel(label.id) }"
            :style="{ borderColor: label.color, color: hasLabel(label.id) ? '#fff' : label.color, background: hasLabel(label.id) ? label.color : 'transparent' }"
            @click="toggleLabel(label)"
          >{{ label.name }}</span>
        </div>
      </div>

      <div class="comments-section">
        <h4>{{ $t('board.comments') }}</h4>
        <div class="comment-list">
          <div v-for="comment in card.comments" :key="comment.id" class="comment">
            <div class="comment-avatar">
              <img v-if="avatarUrl(comment.user)" :src="avatarUrl(comment.user)" :alt="comment.user.display_name" class="comment-avatar-img" @error="e => e.target.style.display='none'" />
              <span v-else>{{ comment.user.display_name?.slice(0,2).toUpperCase() }}</span>
            </div>
            <div class="comment-body">
              <div class="comment-meta">
                <strong>{{ comment.user.display_name || comment.user.username }}</strong>
                <span class="comment-time">{{ formatDateTime(comment.created_at) }}</span>
                <span v-if="comment.is_edited" class="edited-badge">✎</span>
              </div>
              <div class="comment-text" v-html="renderMarkdown(comment.body)"></div>
              <button v-if="comment.user_id === authUser?.id" class="btn btn-ghost btn-sm" @click="deleteComment(comment)">🗑</button>
            </div>
          </div>
        </div>

        <div class="add-comment">
          <CardEditor v-model="newComment" :min-height="'80px'" :placeholder="$t('board.add_comment')" />
          <button class="btn btn-primary btn-sm" @click="submitComment" :disabled="!newComment.trim()">
            {{ $t('board.add_comment') }}
          </button>
        </div>
      </div>

      <div v-if="history.length" class="history-section">
        <h4>{{ $t('board.column_history') }}</h4>
        <div class="history-list">
          <div v-for="h in history" :key="h.id" class="history-entry">
            <span class="history-time">{{ formatDateTime(h.created_at) }}</span>
            <span class="history-who">{{ h.user.display_name || h.user.username }}</span>
            <span class="history-move">
              <span class="history-col">{{ h.from_column.name }}</span>
              →
              <span class="history-col">{{ h.to_column.name }}</span>
            </span>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <button class="btn btn-danger btn-sm" @click="confirmDelete">{{ $t('board.delete_card') }}</button>
      <button class="btn btn-secondary" @click="$emit('close')">{{ $t('common.cancel') }}</button>
      <button class="btn btn-primary" @click="save" :disabled="saving">{{ saving ? $t('common.loading') : $t('common.save') }}</button>
    </template>
  </BaseModal>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import BaseModal from '@/components/common/BaseModal.vue'
import CardEditor from './CardEditor.vue'
import { useAuthStore } from '@/stores/auth'
import { useBoardStore } from '@/stores/board'
import { projectsApi } from '@/api/projects'
import { useUIStore } from '@/stores/ui'
import { useDateFormat } from '@/composables/useDateFormat'
import { avatarUrl } from '@/composables/useAvatar'

const props = defineProps({
  card: { type: Object, required: true },
  labels: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  projectSlug: { type: String, required: true }
})
const emit = defineEmits(['close', 'deleted'])

const auth = useAuthStore()
const boardStore = useBoardStore()
const ui = useUIStore()
const { formatDateTime, formatDate } = useDateFormat()
const authUser = computed(() => auth.user)
const newComment = ref('')
const history = ref([])
const saving = ref(false)

onMounted(async () => {
  try {
    const { data } = await projectsApi.getCardHistory(props.projectSlug, props.card.id)
    history.value = data
  } catch {}
})

const priorities = ['none', 'low', 'medium', 'high', 'critical']

const todayISO = new Date().toISOString().slice(0, 10)

const form = ref({
  title: props.card.title,
  description: props.card.description || '',
  priority: props.card.priority || 'none',
  due_date: props.card.due_date ? props.card.due_date.slice(0, 10) : todayISO,
  assignee_id: props.card.assignee_id || null
})

function hasLabel(labelId) {
  return props.card.labels?.some(l => l.id === labelId)
}

async function toggleLabel(label) {
  try {
    if (hasLabel(label.id)) {
      await projectsApi.removeLabel(props.projectSlug, props.card.id, label.id)
      props.card.labels = props.card.labels.filter(l => l.id !== label.id)
    } else {
      await projectsApi.assignLabel(props.projectSlug, props.card.id, label.id)
      props.card.labels = [...(props.card.labels || []), label]
    }
    boardStore.updateCard({ ...props.card })
  } catch (e) {
    ui.error('Failed to update label')
  }
}

async function save() {
  saving.value = true
  try {
    const payload = {
      title: form.value.title,
      description: form.value.description,
      priority: form.value.priority,
      due_date: form.value.due_date || null,
      assignee_id: form.value.assignee_id
    }
    await boardStore.updateCardData(props.card.id, payload)
    ui.success('Saved')
  } catch (e) {
    ui.error('Failed to save')
  } finally {
    saving.value = false
  }
}

async function submitComment() {
  if (!newComment.value.trim()) return
  try {
    const { data } = await projectsApi.createComment(props.projectSlug, props.card.id, newComment.value)
    props.card.comments = [...(props.card.comments || []), data]
    newComment.value = ''
  } catch (e) {
    ui.error('Failed to post comment')
  }
}

async function deleteComment(comment) {
  try {
    await projectsApi.deleteComment(props.projectSlug, props.card.id, comment.id)
    props.card.comments = props.card.comments.filter(c => c.id !== comment.id)
  } catch (e) {
    ui.error('Failed to delete comment')
  }
}

async function confirmDelete() {
  if (!confirm('Delete this card?')) return
  try {
    await boardStore.deleteCard(props.card.id, props.card.column_id)
    emit('deleted')
    emit('close')
  } catch (e) {
    ui.error('Failed to delete card')
  }
}

function renderMarkdown(text) {
  return DOMPurify.sanitize(marked.parse(text || ''))
}
</script>

<style scoped>
.card-detail { padding-bottom: 8px; }

.form-hint { font-size: 11px; color: var(--color-text-muted); margin-top: 4px; display: block; }

.detail-row { display: flex; gap: 16px; }
.half { flex: 1; }

.labels-picker { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip {
  padding: 3px 10px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 600;
  border: 2px solid;
  cursor: pointer;
  transition: all .15s;
}

.comments-section { margin-top: 24px; border-top: 1px solid var(--color-border); padding-top: 20px; }
.comments-section h4 { margin-bottom: 16px; font-size: 14px; }

.comment-list { display: flex; flex-direction: column; gap: 14px; margin-bottom: 20px; }
.comment { display: flex; gap: 10px; }

.comment-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--color-primary);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
}
.comment-avatar-img { width: 100%; height: 100%; object-fit: cover; border-radius: 50%; }

.comment-body { flex: 1; }
.comment-meta { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; font-size: 12px; }
.comment-time { color: var(--color-text-muted); }
.edited-badge { color: var(--color-text-muted); font-style: italic; font-size: 11px; }

.comment-text { font-size: 13px; line-height: 1.5; }
.comment-text :deep(p) { margin-bottom: 6px; }
.comment-text :deep(code) { background: #f1f5f9; padding: 1px 4px; border-radius: 3px; font-size: 12px; }

.add-comment { display: flex; flex-direction: column; gap: 8px; }
.add-comment .btn { align-self: flex-end; }

.history-section { margin-top: 24px; border-top: 1px solid var(--color-border); padding-top: 20px; }
.history-section h4 { margin-bottom: 12px; font-size: 14px; color: var(--color-text-muted); }
.history-list { display: flex; flex-direction: column; gap: 6px; }
.history-entry { display: flex; align-items: center; gap: 10px; font-size: 12px; }
.history-time { color: var(--color-text-muted); flex-shrink: 0; }
.history-who { font-weight: 600; flex-shrink: 0; }
.history-move { display: flex; align-items: center; gap: 6px; color: var(--color-text-muted); }
.history-col { background: var(--color-bg); border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: 1px 6px; color: var(--color-text); font-size: 11px; }
</style>
