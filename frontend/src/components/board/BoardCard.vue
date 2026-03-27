<template>
  <div class="board-card" @click="$emit('open', card)">
    <!-- Assignee avatar — top right -->
    <div v-if="card.assignee" class="card-avatar">
      <img
        v-if="assigneeAvatar && !avatarErr"
        :src="assigneeAvatar"
        :alt="card.assignee.display_name || card.assignee.username"
        class="avatar-img"
        @error="avatarErr = true"
      />
      <div v-else class="avatar-initials">
        {{ (card.assignee.display_name || card.assignee.username || '?').slice(0, 2).toUpperCase() }}
      </div>
    </div>

    <div class="card-priority" v-if="card.priority !== 'none'">
      <span :class="`badge priority-${card.priority}`">{{ $t(`board.priorities.${card.priority}`) }}</span>
    </div>
    <div class="card-title">{{ card.title }}</div>
    <div class="card-labels" v-if="card.labels?.length">
      <span
        v-for="label in card.labels"
        :key="label.id"
        class="card-label"
        :style="{ background: label.color + '33', color: label.color, border: `1px solid ${label.color}66` }"
      >{{ label.name }}</span>
    </div>
    <div class="card-footer" v-if="card.due_date">
      <span class="card-due" :class="{ overdue: isOverdue }">
        📅 {{ formatDate(card.due_date) }}
      </span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useDateFormat } from '@/composables/useDateFormat'
import { avatarUrl } from '@/composables/useAvatar'

const props = defineProps({ card: { type: Object, required: true } })
defineEmits(['open'])

const { formatDate } = useDateFormat()
const avatarErr = ref(false)

const assigneeAvatar = computed(() => avatarUrl(props.card.assignee))

const isOverdue = computed(() => {
  if (!props.card.due_date) return false
  return new Date(props.card.due_date) < new Date()
})
</script>

<style scoped>
.board-card {
  position: relative;
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: 10px 12px;
  padding-top: 14px;
  cursor: pointer;
  transition: box-shadow .15s;
  user-select: none;
}
.board-card:hover { box-shadow: var(--shadow-md); }

.card-avatar {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  overflow: hidden;
  flex-shrink: 0;
}

.avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 50%;
}

.avatar-initials {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  background: var(--color-primary);
  color: #fff;
  font-size: 9px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}

.card-priority { margin-bottom: 6px; }
.card-title {
  font-size: 13px;
  font-weight: 500;
  line-height: 1.4;
  margin-bottom: 8px;
  /* leave room for avatar on the right */
  padding-right: 20px;
}

.card-labels { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 8px; }
.card-label { font-size: 11px; font-weight: 600; padding: 2px 6px; border-radius: 9999px; }

.card-footer { display: flex; align-items: center; }
.card-due { font-size: 11px; color: var(--color-text-muted); }
.card-due.overdue { color: var(--color-danger); }
</style>
