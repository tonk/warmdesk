<template>
  <main class="ticket-detail-main">
    <div v-if="loading" class="loading-state">
      <div class="spinner" style="width:32px;height:32px;border-width:3px"></div>
    </div>

    <template v-else-if="ticket">
      <header class="detail-header">
        <RouterLink :to="isInbox ? '/tickets/inbox' : `/customers/${customerId}/tickets`" class="back-link">{{ $t('common.go_back') }}</RouterLink>
        <div class="detail-title-row">
          <div class="title-edit-wrap">
            <h1 v-if="!editingTitle" @dblclick="startTitleEdit" :title="$t('common.double_click_edit')">{{ ticket.title }}</h1>
            <input v-else ref="titleInput" v-model="editTitleVal" class="form-input title-input" :aria-label="$t('ticket.title')" @blur="saveTitle" @keydown.enter="saveTitle" @keydown.escape="cancelTitleEdit" />
          </div>
          <div class="detail-actions">
            <select class="form-input form-input-sm" :value="ticket.status" :aria-label="$t('ticket.status')" @change="onStatusSelect($event)">
              <option value="new">{{ $t('ticket.status_new') }}</option>
              <option value="open">{{ $t('ticket.status_open') }}</option>
              <option value="in_progress">{{ $t('ticket.status_in_progress') }}</option>
              <option value="pending">{{ $t('ticket.status_pending') }}</option>
              <option value="resolved" :disabled="checklistBlocksClose">{{ $t('ticket.status_resolved') }}</option>
              <option value="pending_close" :disabled="checklistBlocksClose">{{ $t('ticket.status_pending_close') }}</option>
              <option value="closed" :disabled="checklistBlocksClose">{{ $t('ticket.status_closed') }}</option>
            </select>
            <select class="form-input form-input-sm" :value="ticket.priority" :aria-label="$t('ticket.priority')" @change="updatePriority($event.target.value)">
              <option value="low">{{ $t('ticket.priority_low') }}</option>
              <option value="medium">{{ $t('ticket.priority_medium') }}</option>
              <option value="high">{{ $t('ticket.priority_high') }}</option>
              <option value="critical">{{ $t('ticket.priority_critical') }}</option>
            </select>
            <select v-if="macros.length" class="form-input form-input-sm" :value="''" :disabled="applyingMacro" @change="applyMacro($event.target.value); $event.target.value = ''" :aria-label="$t('macro.apply_macro')">
              <option value="" disabled>{{ $t('macro.apply_macro') }}</option>
              <option v-for="m in macros" :key="m.id" :value="m.id">{{ m.name }}</option>
            </select>
            <HelpIcon v-if="macros.length" i18n-key="help.fields.ticket_macros" :label="$t('macro.apply_macro')" />
            <select v-if="checklistTemplates.length && !ticket.checklist_template_id" class="form-input form-input-sm" :value="''" :disabled="applyingChecklist" :aria-label="$t('ticketChecklist.apply')" @change="applyChecklist($event.target.value); $event.target.value = ''">
              <option value="" disabled>{{ $t('ticketChecklist.apply') }}</option>
              <option v-for="t in checklistTemplates" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
            <button v-if="!ticket.is_spam" class="btn btn-secondary btn-sm" @click="toggleSpam(true)">{{ $t('ticket.mark_spam') }}</button>
            <button v-else class="btn btn-secondary btn-sm" @click="toggleSpam(false)">{{ $t('ticket.unmark_spam') }}</button>
            <button class="btn btn-danger btn-sm" @click="deleteTicket">{{ $t('common.delete') }}</button>
          </div>
        </div>
        <div v-if="ticket.is_spam" class="spam-banner" role="alert">
          <span>⚠ {{ $t('ticket.spam_banner') }}</span>
        </div>
        <div class="detail-meta">
          <span class="ticket-type" :class="'type-' + ticket.type">{{ $t('ticket.type_' + ticket.type) }}</span>
          <span class="ticket-status" :class="'status-' + ticket.status">{{ $t('ticket.status_' + ticket.status) }}</span>
          <span v-if="ticket.is_major_incident" class="major-incident-badge">{{ $t('ticket.major_incident') }}</span>
          <span v-if="ticket.type === 'problem' && ticket.is_known_error" class="known-error-badge">{{ $t('ticket.known_error') }}</span>
          <span v-if="ticket.reopen_count > 0" class="reopen-badge" :title="$t('ticket.reopen_count_hint')">{{ $t('ticket.reopened_n', { n: ticket.reopen_count }) }}</span>
          <span v-if="ticket.category" class="category-chip">{{ categoryLabel(ticket.category_id) }}</span>
          <span v-if="ticket.sla_policy_id" :class="['sla-badge', ticket.sla_response_breached || ticket.sla_resolution_breached ? 'sla-breach' : slaWarning(ticket) ? 'sla-warning' : 'sla-ok']">
            {{ ticket.sla_policy?.name || $t('sla.sla') }}
            <template v-if="ticket.sla_response_breached">· {{ $t('sla.response_breached') }}</template>
            <template v-else-if="ticket.sla_resolution_breached">· {{ $t('sla.resolution_breached') }}</template>
          </span>
          <span class="ticket-id-copy" title="Click to copy ticket reference" @click="copyTicketRef">#{{ ticket.id }}</span>
          <span>{{ $t('ticket.created_by') }} {{ ticket.created_by?.display_name || ticket.created_by?.username }}</span>
          <span>{{ formatDateTime(ticket.created_at) }}</span>
          <span v-if="ticket.from_email" class="from-email-badge" :title="ticket.from_email">✉ {{ ticket.from_email }}</span>
          <span v-if="!isInbox" class="assignee-wrap">
            <label class="sr-only" for="assignee-select">{{ $t('ticket.assigned_to') }}</label>
            <select id="assignee-select" class="form-input form-input-sm" :value="ticket.assigned_to_id" @change="updateAssignedTo($event.target.value)">
              <option :value="null">—</option>
              <option v-for="u in customerUsers" :key="u.user_id" :value="u.user_id">{{ u.display_name || u.username }}</option>
            </select>
          </span>
          <span v-if="ticket.owner">{{ $t('ticket.owner') }} {{ ticket.owner.display_name || ticket.owner.username }}</span>
          <span v-if="ticket.group">{{ $t('ticket.group') }} {{ ticket.group.name }}</span>
        </div>
      </header>

      <div v-if="ticket.status === 'pending'" class="reminder-row">
        <span class="reminder-label">
          {{ $t('ticket.reminder_date') }}
          <HelpIcon i18n-key="help.fields.ticket_reminder" :label="$t('ticket.reminder_date')" />
        </span>
        <DatePicker :model-value="reminderDateValue" @update:model-value="updateReminderDate" />
      </div>
      <div v-if="ticket.status === 'pending_close'" class="reminder-row">
        <span class="reminder-label">
          {{ $t('ticket.close_date') }}
          <HelpIcon i18n-key="help.fields.ticket_close_date" :label="$t('ticket.close_date')" />
        </span>
        <DatePicker :model-value="closeDateValue" @update:model-value="updateCloseDate" />
      </div>

      <div v-if="!isInbox" class="detail-fields">
        <div class="detail-tags" v-if="ticket.tags?.length">
          <span v-for="tag in ticket.tags" :key="tag.id" class="tag-chip">
            #{{ tag.name }}
            <button class="tag-remove" @click="removeTag(tag)" title="Remove tag" aria-label="Remove tag">×</button>
          </span>
        </div>
        <div class="tag-input-row">
          <input class="form-input form-input-sm tag-input" v-model="newTagName" :placeholder="$t('ticket.add_tag_placeholder')" :aria-label="$t('ticket.tags')" @keydown.enter.prevent="addTag" @keydown.comma.prevent="addTag" />
          <button class="btn btn-secondary btn-sm" @click="addTag" :disabled="!newTagName.trim()">{{ $t('common.add') }}</button>
        </div>
      </div>

      <section class="itil-panel" aria-labelledby="itil-classification-heading">
        <h4 id="itil-classification-heading">{{ $t('ticket.classification') }}</h4>
        <div class="itil-grid">
          <div class="itil-field">
            <label for="itil-type">{{ $t('ticket.type') }}</label>
            <select id="itil-type" class="form-input form-input-sm" :value="ticket.type" @change="saveFields({ type: $event.target.value })">
              <option v-for="tp in TICKET_TYPES" :key="tp" :value="tp">{{ $t('ticket.type_' + tp) }}</option>
            </select>
          </div>
          <div class="itil-field">
            <label for="itil-impact">{{ $t('ticket.impact') }}</label>
            <select id="itil-impact" class="form-input form-input-sm" :value="ticket.impact || ''" @change="saveFields({ impact: $event.target.value })">
              <option value="">—</option>
              <option v-for="lv in LEVELS" :key="lv" :value="lv">{{ $t('ticket.impact_' + lv) }}</option>
            </select>
          </div>
          <div class="itil-field">
            <label for="itil-urgency">{{ $t('ticket.urgency') }}</label>
            <select id="itil-urgency" class="form-input form-input-sm" :value="ticket.urgency || ''" @change="saveFields({ urgency: $event.target.value })">
              <option value="">—</option>
              <option v-for="lv in LEVELS" :key="lv" :value="lv">{{ $t('ticket.urgency_' + lv) }}</option>
            </select>
          </div>
          <div class="itil-field">
            <label for="itil-category">{{ $t('ticket.category') }}</label>
            <select id="itil-category" class="form-input form-input-sm" :value="ticket.category_id || 0" @change="saveFields({ category_id: Number($event.target.value) })">
              <option :value="0">—</option>
              <option v-for="c in categoryOptions" :key="c.id" :value="c.id">{{ c.label }}</option>
            </select>
          </div>
          <div class="itil-field itil-field-check">
            <label class="checkbox-label">
              <input type="checkbox" :checked="ticket.is_major_incident" @change="saveFields({ is_major_incident: $event.target.checked })" />
              {{ $t('ticket.major_incident') }}
            </label>
          </div>
        </div>
        <p v-if="derivedPriorityHint" class="itil-hint" role="status">{{ derivedPriorityHint }}</p>

        <div v-if="isResolvedStatus(ticket.status) || ticket.resolution_code_id || ticket.resolution_notes" class="itil-resolution">
          <h5>{{ $t('ticket.resolution') }}</h5>
          <div class="itil-grid">
            <div class="itil-field">
              <label for="itil-resolution-code">{{ $t('ticket.resolution_code') }}</label>
              <select id="itil-resolution-code" class="form-input form-input-sm" :value="ticket.resolution_code_id || 0" @change="saveFields({ resolution_code_id: Number($event.target.value) })">
                <option :value="0">—</option>
                <option v-for="rc in resolutionCodeOptions" :key="rc.id" :value="rc.id">{{ rc.name }}</option>
              </select>
            </div>
            <div class="itil-field itil-field-wide">
              <label for="itil-resolution-notes">{{ $t('ticket.resolution_notes') }}</label>
              <textarea id="itil-resolution-notes" v-model="resolutionNotesDraft" class="form-input" rows="2" @blur="saveResolutionNotes"></textarea>
            </div>
          </div>
          <p v-if="ticket.resolved_at" class="itil-hint">{{ $t('ticket.resolved_at') }} {{ formatDateTime(ticket.resolved_at) }}</p>
        </div>
      </section>

      <section v-if="ticket.type === 'problem'" class="itil-panel problem-panel" aria-labelledby="problem-heading">
        <h4 id="problem-heading">{{ $t('ticket.problem_management') }}</h4>
        <div class="itil-grid">
          <div class="itil-field">
            <label for="problem-status">{{ $t('ticket.problem_status') }}</label>
            <select id="problem-status" class="form-input form-input-sm" :value="ticket.problem_status || 'investigating'" @change="saveFields({ problem_status: $event.target.value })">
              <option v-for="ps in PROBLEM_STATUSES" :key="ps" :value="ps">{{ $t('ticket.problem_status_' + ps) }}</option>
            </select>
          </div>
          <div class="itil-field itil-field-check">
            <label class="checkbox-label">
              <input type="checkbox" :checked="ticket.is_known_error" @change="saveFields({ is_known_error: $event.target.checked })" />
              {{ $t('ticket.known_error') }}
            </label>
            <HelpIcon i18n-key="ticket.known_error_hint" :label="$t('ticket.known_error')" />
          </div>
          <div class="itil-field itil-field-wide">
            <label for="problem-root-cause">{{ $t('ticket.root_cause') }}</label>
            <textarea id="problem-root-cause" v-model="problemDraft.root_cause" class="form-input" rows="3" @blur="saveProblemText('root_cause')"></textarea>
          </div>
          <div class="itil-field itil-field-wide">
            <label for="problem-workaround">{{ $t('ticket.workaround') }}</label>
            <textarea id="problem-workaround" v-model="problemDraft.workaround" class="form-input" rows="3" @blur="saveProblemText('workaround')"></textarea>
          </div>
        </div>
        <div v-if="!isInbox" class="problem-actions">
          <span class="itil-hint">{{ $t('ticket.linked_incidents_n', { n: linkedIncidents.length }) }}</span>
          <button class="btn btn-secondary btn-sm" :disabled="!openLinkedIncidents.length" @click="openResolveLinked">{{ $t('ticket.resolve_linked') }}</button>
        </div>
      </section>

      <div v-if="ticket.description" class="detail-description">
        <div class="detail-desc-header">
          <span class="detail-desc-label">{{ $t('ticket.original_email') }}</span>
          <span v-if="ticket.from_name" class="from-name">{{ ticket.from_name }}</span>
          <a v-if="ticket.from_email" :href="'mailto:' + ticket.from_email" class="from-email-link" :title="ticket.from_email">{{ ticket.from_email }}</a>
          <button v-if="ticket.email_message_id" class="btn btn-ghost btn-sm" @click="showOriginalEmail" style="margin-left:auto">{{ $t('ticket.show_original') }}</button>
        </div>
        <div class="email-body markdown-body selectable" v-html="renderMarkdown(ticket.description)"></div>
      </div>
      <AttachmentList v-if="ticket.attachments?.length" :attachments="ticket.attachments" :can-delete="false" />

      <div v-if="checklist.length || ticket.checklist_template_id" class="checklist-section">
        <div class="checklist-header">
          <h4>
            {{ $t('checklist.title') }}
            <HelpIcon i18n-key="help.fields.ticket_checklist" :label="$t('checklist.title')" />
          </h4>
          <span v-if="checklist.length" class="checklist-progress">
            {{ checklist.filter(i => i.is_completed).length }}/{{ checklist.length }}
          </span>
        </div>
        <p v-if="checklistBlocksClose" class="checklist-close-hint" role="status">{{ $t('ticketChecklist.close_blocked') }}</p>
        <div v-if="checklist.length" class="checklist-progress-bar">
          <div class="checklist-progress-fill" :style="{ width: checklistPct + '%' }"></div>
        </div>
        <div class="checklist-items" ref="checklistListEl">
          <div v-for="item in checklist" :key="item.id" class="checklist-item" :data-id="item.id">
            <span class="checklist-drag-handle" :title="$t('ticketChecklist.drag_reorder')" aria-hidden="true">⠿</span>
            <input
              type="checkbox"
              class="checklist-checkbox"
              :checked="item.is_completed"
              :aria-label="item.body"
              @change="toggleChecklistItem(item)"
            />
            <span class="checklist-body" :class="{ completed: item.is_completed }">{{ item.body }}</span>
            <button class="btn-icon-xs btn-danger" @click="removeChecklistItem(item)" :title="$t('ticketChecklist.delete_item')" :aria-label="$t('ticketChecklist.delete_item')">×</button>
          </div>
        </div>
      </div>

      <div v-if="ticket.sla_policy_id && !isInbox" class="sla-card">
        <h4>{{ ticket.sla_policy?.name || $t('sla.sla') }}</h4>
        <p v-if="ticket.sla_paused_at" class="sla-paused" role="status">⏸ {{ $t('sla.paused_since', { date: formatDateTime(ticket.sla_paused_at) }) }}</p>
        <div class="sla-card-body">
          <div class="sla-card-row" v-if="ticket.sla_response_deadline">
            <span class="sla-card-label">{{ $t('sla.response_by') }}</span>
            <span :class="['sla-card-value', ticket.sla_response_breached ? 'sla-card-breach' : '']">{{ formatDateTime(ticket.sla_response_deadline) }}</span>
            <span v-if="ticket.first_response_at" class="sla-card-check met"><span class="feat-check" style="color:var(--color-success)">✓</span> {{ $t('sla.first_response') }} {{ formatDateTime(ticket.first_response_at) }}</span>
            <span v-else-if="ticket.sla_response_breached" class="sla-card-status breach">{{ $t('sla.response_breached') }}</span>
            <span v-else-if="slaWarning(ticket)" class="sla-card-status warn">{{ $t('sla.warning') }}</span>
            <span v-else class="sla-card-status ok">{{ $t('sla.on_track') }}</span>
          </div>
          <div class="sla-card-row" v-if="ticket.sla_resolution_deadline">
            <span class="sla-card-label">{{ $t('sla.resolution_by') }}</span>
            <span :class="['sla-card-value', ticket.sla_resolution_breached ? 'sla-card-breach' : '']">{{ formatDateTime(ticket.sla_resolution_deadline) }}</span>
            <span v-if="ticket.resolved_at && !ticket.sla_resolution_breached" class="sla-card-check met"><span class="feat-check" style="color:var(--color-success)">✓</span> {{ $t('ticket.status_resolved') }} {{ formatDateTime(ticket.resolved_at) }}</span>
            <span v-else-if="ticket.sla_resolution_breached" class="sla-card-status breach">{{ $t('sla.resolution_breached') }}</span>
            <span v-else-if="slaWarning(ticket)" class="sla-card-status warn">{{ $t('sla.warning') }}</span>
            <span v-else class="sla-card-status ok">{{ $t('sla.on_track') }}</span>
          </div>
        </div>
      </div>

      <div class="linked-tickets-section" v-if="linkedTickets !== null && !isInbox">
        <h4>{{ $t('ticket.linked_tickets') }} <span v-if="linkedTickets.length" class="count">({{ linkedTickets.length }})</span></h4>
        <div v-if="linkedTickets.length" class="linked-list">
          <div v-for="lt in linkedTickets" :key="lt.link_id" class="linked-row" @click="openTicket(lt)">
            <span class="linked-type">{{ linkTypeLabel(lt) }}</span>
            <span class="linked-status" :class="'status-' + lt.status">{{ $t('ticket.status_' + lt.status) }}</span>
            <span class="linked-priority" :class="'pri-' + lt.priority">{{ lt.priority }}</span>
            <span class="linked-title">#{{ lt.id }} {{ lt.title }}</span>
            <span v-if="lt.type === 'problem' && lt.is_known_error" class="known-error-badge">{{ $t('ticket.known_error') }}</span>
            <button v-if="lt.workaround" class="btn btn-ghost btn-xs" @click.stop="useWorkaround(lt)">{{ $t('ticket.use_workaround') }}</button>
            <button class="btn-icon-xs" @click.stop="removeLinkedTicket(lt)" :title="$t('ticket.remove_link')" :aria-label="$t('ticket.remove_link')">✕</button>
          </div>
        </div>
        <div class="linked-add-row">
          <label class="sr-only" for="new-link-type">{{ $t('ticket.link_type') }}</label>
          <select id="new-link-type" class="form-input form-input-sm" v-model="newLinkType">
            <option v-for="lt in LINK_TYPES" :key="lt" :value="lt">{{ $t('ticket.link_out_' + lt) }}</option>
          </select>
          <input class="form-input form-input-sm" v-model="newLinkId" :placeholder="$t('ticket.add_link_placeholder')" :aria-label="$t('ticket.linked_tickets')" @keydown.enter.prevent="addLinkedTicket" />
          <button class="btn btn-secondary btn-sm" @click="addLinkedTicket" :disabled="!newLinkId.trim()">{{ $t('common.add') }}</button>
        </div>
      </div>

      <div v-if="linkedCards !== null && !isInbox" class="linked-tickets-section">
        <h4>{{ $t('card_ref.linked_cards') }} <span v-if="linkedCards.length" class="count">({{ linkedCards.length }})</span></h4>
        <div v-if="linkedCards.length" class="linked-list">
          <div v-for="lc in linkedCards" :key="lc.link_id" class="linked-row" @click="openCard(lc)">
            <span class="linked-ref">{{ lc.project_key }}-{{ lc.card_number }}</span>
            <span class="linked-title">{{ lc.title }}</span>
            <button class="btn-icon-xs" @click.stop="removeCardLink(lc)" :title="$t('ticket.remove_link')" aria-label="Remove link">✕</button>
          </div>
        </div>
        <div class="linked-add-row">
          <input class="form-input form-input-sm" v-model="newCardLink" :placeholder="$t('card_ref.add_link_placeholder')" :aria-label="$t('card_ref.linked_cards')" @keydown.enter.prevent="addCardLink" />
          <button class="btn btn-secondary btn-sm" @click="addCardLink" :disabled="!newCardLink.trim()">{{ $t('common.add') }}</button>
        </div>
      </div>

      <div v-if="isInbox" class="move-section">
        <h4>{{ $t('inbox.customer') }}</h4>
        <div class="move-row">
          <label class="sr-only" for="assign-customer-select">{{ $t('inbox.assign_customer') }}</label>
          <select id="assign-customer-select" class="form-input form-input-sm" v-model="moveTargetCustomer">
            <option :value="null">—</option>
            <option v-for="c in allCustomers" :key="c.id" :value="c.id">{{ c.name }}</option>
          </select>
          <button class="btn btn-primary btn-sm" @click="assignToCustomer" :disabled="!moveTargetCustomer || moving">{{ $t('inbox.assign') }}</button>
        </div>
      </div>
      <div v-else class="move-section">
        <h4>{{ $t('ticket.move_to_customer') }}</h4>
        <div class="move-row">
          <label class="sr-only" for="move-customer-select">{{ $t('ticket.move_to_customer') }}</label>
          <select id="move-customer-select" class="form-input form-input-sm" v-model="moveTargetCustomer">
            <option :value="null">—</option>
            <option v-for="c in allCustomers" :key="c.id" :value="c.id" :disabled="c.id === customerId">{{ c.name }}</option>
          </select>
          <button class="btn btn-secondary btn-sm" @click="moveTicket" :disabled="!moveTargetCustomer || moveTargetCustomer === customerId || moving">{{ $t('ticket.move') }}</button>
        </div>
        <h4 style="margin-top:16px">{{ $t('card_ref.create_card') }}</h4>
        <div class="move-row">
          <label class="sr-only" for="create-card-project">{{ $t('board.select_project') }}</label>
          <select id="create-card-project" class="form-input form-input-sm" v-model="createCardProjectSlug" @change="loadCreateCardColumns">
            <option value="" disabled>{{ $t('board.select_project') }}</option>
            <option v-for="p in allProjects" :key="p.slug" :value="p.slug">{{ p.name }}</option>
          </select>
          <label class="sr-only" for="create-card-column">{{ $t('board.select_column') }}</label>
          <select id="create-card-column" class="form-input form-input-sm" v-model="createCardColumnId" :disabled="!createCardProjectSlug">
            <option value="" disabled>{{ $t('board.select_column') }}</option>
            <option v-for="col in createCardColumns" :key="col.id" :value="col.id">{{ col.name }}</option>
          </select>
          <button class="btn btn-primary btn-sm" @click="doCreateCard" :disabled="!createCardColumnId || creatingCard">{{ creatingCard ? $t('common.creating') : $t('common.create') }}</button>
        </div>
      </div>

      <section v-if="auth.timeTrackingEnabled && !isInbox" class="time-section">
        <h4>{{ $t('ticket.time_logged') }} <span v-if="ticketTimeEntries.length" class="count">({{ formatMinutes(totalMinutes) }})</span></h4>

        <div v-if="ticketTimeEntries.length" class="time-list">
          <div v-for="te in ticketTimeEntries" :key="te.id" class="time-row">
            <span class="time-date">{{ formatDate(te.date) }}</span>
            <span class="time-who">{{ te.user?.display_name || te.user?.username }}</span>
            <span class="time-dur">{{ formatMinutes(te.minutes) }}</span>
            <span v-if="te.project" class="time-proj">{{ te.project.name }}</span>
            <span class="time-desc">{{ te.description }}</span>
            <button v-if="te.user_id === auth.user?.id" class="btn-icon-xs" @click="deleteTimeEntry(te)" :title="$t('ticket.time_delete_entry')" :aria-label="$t('ticket.time_delete_entry')">✕</button>
          </div>
        </div>
        <p v-else class="time-empty">{{ $t('ticket.no_time_logged') }}</p>

        <form class="time-add-form" @submit.prevent="logTime">
          <label class="sr-only" for="te-date">{{ $t('ticket.time_date') }}</label>
          <DatePicker id="te-date" :model-value="teDate" @update:model-value="teDate = $event" />
          <label class="sr-only" for="te-hours">{{ $t('ticket.time_hours') }}</label>
          <input id="te-hours" class="form-input form-input-sm te-num" type="number" min="0" max="23" v-model.number="teHours" :placeholder="$t('ticket.time_hours')" />
          <label class="sr-only" for="te-mins">{{ $t('ticket.time_minutes') }}</label>
          <input id="te-mins" class="form-input form-input-sm te-num" type="number" min="0" max="59" step="5" v-model.number="teMins" :placeholder="$t('ticket.time_minutes')" />
          <label class="sr-only" for="te-project">{{ $t('board.select_project') }}</label>
          <select id="te-project" class="form-input form-input-sm te-project" v-model="teProjectId">
            <option :value="null">— {{ $t('board.select_project') }} —</option>
            <option v-for="p in customerProjects" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
          <label class="sr-only" for="te-desc">{{ $t('ticket.time_note') }}</label>
          <input id="te-desc" class="form-input form-input-sm te-desc" type="text" v-model="teDesc" :placeholder="$t('ticket.time_note')" />
          <button type="submit" class="btn btn-secondary btn-sm" :disabled="teSubmitting || (teHours === 0 && teMins === 0)">{{ $t('ticket.log_time') }}</button>
        </form>
      </section>

      <section class="messages-section">
        <h2>{{ $t('ticket.messages') }} ({{ ticket.messages?.length || 0 }})</h2>

        <div v-if="!ticket.messages?.length" class="empty-state">
          {{ $t('ticket.no_messages') }}
        </div>

        <div v-else class="messages-list">
          <div v-for="item in threadedMessages" :key="item.msg.id" :class="['message', { 'message--private': item.msg.is_private }]" :style="item.depth > 0 ? { marginLeft: `${32 + (item.depth - 1) * 32}px` } : undefined">
            <div class="message-header">
              <span class="msg-author-group">
                <strong>{{ item.msg.from_name || item.msg.user?.display_name || item.msg.user?.username }}</strong>
                <span v-if="item.msg.email_sent" class="email-sent-badge" :title="$t('ticket.email_sent_hint')">✉</span>
                <span v-if="item.msg.is_private" class="private-badge" :title="$t('ticket.private_hint')">🔒 {{ $t('ticket.private') }}</span>
              </span>
              <span class="message-time">{{ formatDateTime(item.msg.created_at) }}</span>
            </div>
            <div class="message-body markdown-body selectable" v-html="renderMarkdown(item.msg.body)"></div>
            <div class="message-actions">
              <button v-if="auth.user?.global_role !== 'customer'" class="btn btn-ghost btn-xs" @click="togglePrivate(item.msg)">{{ item.msg.is_private ? $t('ticket.set_public') : $t('ticket.set_private') }}</button>
              <button class="btn btn-ghost btn-xs" @click="startReply(item.msg)">{{ $t('ticket.reply') }}</button>
            </div>
            <form v-if="replyingToMsgId === item.msg.id" class="message-reply-form" :style="{ marginLeft: `${32 + item.depth * 32}px` }" @submit.prevent="submitReply()">
              <textarea v-model="replyText" class="form-input reply-textarea" rows="3" :placeholder="$t('ticket.reply_placeholder')" :aria-label="$t('ticket.reply')" required></textarea>
              <div class="reply-form-actions">
                <button type="button" class="btn btn-secondary btn-sm" @click="cancelReply">{{ $t('common.cancel') }}</button>
                <button type="submit" class="btn btn-primary btn-sm" :disabled="!replyText.trim() || sendingReply">{{ $t('ticket.send') }}</button>
              </div>
            </form>
          </div>
        </div>

        <form class="message-form" @submit.prevent="submitMessage">
          <div class="md-editor">
            <div class="md-editor-tabs" role="tablist">
              <button role="tab" :aria-selected="msgEditorTab === 'edit'" aria-controls="ticket-msg-panel-edit" id="ticket-msg-tab-edit" :class="['md-tab', { active: msgEditorTab === 'edit' }]" type="button" @click="msgEditorTab = 'edit'">{{ $t('common.edit') }}</button>
              <button role="tab" :aria-selected="msgEditorTab === 'preview'" aria-controls="ticket-msg-panel-preview" id="ticket-msg-tab-preview" :class="['md-tab', { active: msgEditorTab === 'preview' }]" type="button" @click="msgEditorTab = 'preview'">{{ $t('common.preview') }}</button>
            </div>
            <textarea v-if="msgEditorTab === 'edit'" id="ticket-msg-panel-edit" role="tabpanel" aria-labelledby="ticket-msg-tab-edit" v-model="newMessage" class="form-input md-textarea" rows="8" :placeholder="$t('ticket.message_placeholder')" required @paste="onMsgPaste"></textarea>
            <div v-else id="ticket-msg-panel-preview" role="tabpanel" aria-labelledby="ticket-msg-tab-preview" class="md-preview markdown-body" v-html="renderMarkdown(newMessage)"></div>
          </div>
          <div v-if="pendingFiles.length" class="pending-attachments">
            <span class="pending-label">{{ $t('ticket.pending_files') }}</span>
            <AttachmentList :attachments="pendingFiles" :can-delete="true" @remove="removePending" />
          </div>
          <div class="message-form-actions">
            <FileUploadButton @files-selected="onFilesSelected" />
            <label v-if="auth.user?.global_role !== 'customer'" class="private-checkbox-label">
              <input type="checkbox" v-model="newMsgPrivate" />
              {{ $t('ticket.private') }}
              <HelpIcon i18n-key="ticket.private_hint" :label="$t('ticket.private')" />
            </label>
            <button v-if="newMessage.trim() || pendingFiles.length" type="button" class="btn btn-secondary btn-sm" @click="newMessage = ''; pendingFiles = []; newMsgPrivate = false; msgEditorTab = 'edit'">{{ $t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary btn-sm" :disabled="(!newMessage.trim() && !pendingFiles.length) || sending">{{ $t('ticket.send') }}<span v-if="pendingFiles.length" class="pending-badge">· {{ pendingFiles.length }}</span></button>
          </div>
        </form>
      </section>

      <section v-if="viewers.length" class="viewers-section">
        <div class="viewers-row">
          <div
            v-for="v in viewers"
            :key="v.user_id"
            class="viewer-wrap"
          >
            <div
              class="viewer-avatar"
              :style="viewerAvatarBg(v.user)"
              :aria-label="(v.user?.display_name || v.user?.username) + ' — ' + formatDateTime(v.viewed_at)"
            >
              <img v-if="viewerAvatarSrc(v.user)" :src="viewerAvatarSrc(v.user)" class="viewer-avatar-img" alt="" aria-hidden="true" @error="e => e.target.style.display='none'" />
              <span v-else class="viewer-initials" aria-hidden="true">{{ viewerInitials(v.user) }}</span>
            </div>
            <div class="viewer-tooltip" role="tooltip">
              <span class="viewer-tooltip-name">{{ v.user?.display_name || v.user?.username }}</span>
              <span class="viewer-tooltip-date">{{ formatDateTime(v.viewed_at) }}</span>
            </div>
          </div>
        </div>
      </section>

      <section v-if="!isInbox" class="activity-section">
        <button class="activity-toggle" @click="showActivity = !showActivity" :aria-expanded="showActivity">
          <span class="activity-toggle-icon" :class="{ rotated: showActivity }">▸</span>
          {{ $t('ticket.history') }} <span class="activity-count">({{ history.length }})</span>
        </button>
        <div v-if="showActivity">
          <div v-if="!history.length" class="activity-empty">{{ $t('common.no_results') }}</div>
          <div v-else class="activity-list">
            <div v-for="h in history" :key="h.id" class="activity-entry">
              <span class="activity-icon" :class="'activity-type-' + h.event_type" aria-hidden="true">{{ activityIcon(h.event_type) }}</span>
              <span class="activity-time">{{ formatDateTime(h.created_at) }}</span>
              <span class="activity-who">{{ h.user?.display_name || h.user?.username }}</span>
              <span class="activity-detail">{{ activityLabel(h) }}</span>
            </div>
          </div>
        </div>
      </section>
    </template>

    <div v-else class="empty-state">
      {{ $t('ticket.not_found') }}
    </div>

    <BaseModal v-if="resolveDialog" :title="$t('ticket.resolve_title')" @close="resolveDialog = null">
      <form id="resolve-form" class="resolve-form" @submit.prevent="confirmResolve">
        <p class="itil-hint">{{ $t('ticket.resolve_intro', { status: $t('ticket.status_' + resolveDialog.status) }) }}</p>
        <label for="resolve-code">{{ $t('ticket.resolution_code') }}<span v-if="helpdeskSettings.require_resolution_code" aria-hidden="true"> *</span></label>
        <select id="resolve-code" v-model.number="resolveDialog.code" class="form-input" :required="helpdeskSettings.require_resolution_code">
          <option :value="0">—</option>
          <option v-for="rc in resolutionCodeOptions" :key="rc.id" :value="rc.id">{{ rc.name }}</option>
        </select>
        <label for="resolve-notes">{{ $t('ticket.resolution_notes') }}</label>
        <textarea id="resolve-notes" v-model="resolveDialog.notes" class="form-input" rows="3"></textarea>
      </form>
      <template #footer>
        <button class="btn btn-secondary" @click="resolveDialog = null">{{ $t('common.cancel') }}</button>
        <button class="btn btn-primary" type="submit" form="resolve-form" :disabled="resolving || (helpdeskSettings.require_resolution_code && !resolveDialog.code)">{{ $t('ticket.status_' + resolveDialog.status) }}</button>
      </template>
    </BaseModal>

    <BaseModal v-if="resolveLinkedDialog" :title="$t('ticket.resolve_linked')" @close="resolveLinkedDialog = null">
      <form id="resolve-linked-form" class="resolve-form" @submit.prevent="confirmResolveLinked">
        <p class="itil-hint">{{ $t('ticket.resolve_linked_intro', { n: openLinkedIncidents.length }) }}</p>
        <label for="resolve-linked-code">{{ $t('ticket.resolution_code') }}<span v-if="helpdeskSettings.require_resolution_code" aria-hidden="true"> *</span></label>
        <select id="resolve-linked-code" v-model.number="resolveLinkedDialog.code" class="form-input" :required="helpdeskSettings.require_resolution_code">
          <option :value="0">—</option>
          <option v-for="rc in resolutionCodeOptions" :key="rc.id" :value="rc.id">{{ rc.name }}</option>
        </select>
        <label for="resolve-linked-msg">{{ $t('ticket.resolve_linked_message') }}</label>
        <textarea id="resolve-linked-msg" v-model="resolveLinkedDialog.message" class="form-input" rows="4"></textarea>
      </form>
      <template #footer>
        <button class="btn btn-secondary" @click="resolveLinkedDialog = null">{{ $t('common.cancel') }}</button>
        <button class="btn btn-primary" type="submit" form="resolve-linked-form" :disabled="resolving || (helpdeskSettings.require_resolution_code && !resolveLinkedDialog.code)">{{ $t('ticket.resolve_linked') }}</button>
      </template>
    </BaseModal>

    <BaseModal v-if="showRawEmail" title="Original email" @close="showRawEmail = false" :resizable="true" style="--modal-width: 800px">
      <pre class="raw-email-pre">{{ rawEmailContent }}</pre>
    </BaseModal>
  </main>
</template>

<script setup>
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Sortable from 'sortablejs'
import { ticketsApi } from '@/api/tickets'
import { helpdeskApi } from '@/api/helpdesk'
import { macrosApi } from '@/api/macros'
import { ticketChecklistsApi } from '@/api/ticketChecklists'
import client from '@/api/client'
import { attachmentsApi } from '@/api/attachments'
import { customersApi } from '@/api/customers'
import { projectsApi } from '@/api/projects'
import { timeEntriesApi } from '@/api/timeEntries'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { useTicketsStore } from '@/stores/tickets'
import { useDateFormat } from '@/composables/useDateFormat'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AttachmentList from '@/components/common/AttachmentList.vue'
import FileUploadButton from '@/components/common/FileUploadButton.vue'
import DatePicker from '@/components/common/DatePicker.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import HelpIcon from '@/components/common/HelpIcon.vue'
import { avatarUrl } from '@/composables/useAvatar'

const { formatDateTime, formatDate } = useDateFormat()
const { t } = useI18n()
const auth = useAuthStore()
const ticketsStore = useTicketsStore()

function renderMarkdown(text) {
  return DOMPurify.sanitize(marked.parse(text || ''))
}

const AVATAR_COLORS = ['#6366f1','#8b5cf6','#ec4899','#f59e0b','#10b981','#3b82f6','#ef4444','#14b8a6']
function viewerAvatarBg(user) {
  const idx = (user?.username?.charCodeAt(0) || 0) % AVATAR_COLORS.length
  return { background: AVATAR_COLORS[idx] }
}
function viewerAvatarSrc(user) { return avatarUrl(user) }
function viewerInitials(user) {
  const name = user?.display_name || user?.username || '?'
  return name.slice(0, 2).toUpperCase()
}

async function showOriginalEmail() {
  if (rawEmailContent.value) { showRawEmail.value = true; return }
  rawEmailLoading.value = true
  try {
    const res = await client.get(`/customers/${customerId.value}/tickets/${ticketId.value}/raw-email`, { responseType: 'text' })
    rawEmailContent.value = res.data
    showRawEmail.value = true
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to load original email')
  } finally {
    rawEmailLoading.value = false
  }
}

const descEditorTab = ref('preview')
const msgEditorTab = ref('edit')

const route = useRoute()
const router = useRouter()
const ui = useUIStore()

const isInbox = computed(() => route.name === 'inbox-ticket-detail')
const customerId = computed(() => isInbox.value ? null : Number(route.params.id))
const ticketId = computed(() => Number(route.params.ticketId))
const ticket = ref(null)
const showRawEmail = ref(false)
const rawEmailContent = ref('')
const rawEmailLoading = ref(false)
const history = ref([])
const viewers = ref([])
const showActivity = ref(false)
const loading = ref(true)
const newMessage = ref('')
const newMsgPrivate = ref(false)
const pendingFiles = ref([])
const sending = ref(false)
const newTagName = ref('')
const linkedTickets = ref(null)
const newLinkId = ref('')
const newLinkType = ref('related')
const linkedCards = ref(null)
const newCardLink = ref('')
const customerUsers = ref([])
const editingTitle = ref(false)
const editTitleVal = ref('')
const titleInput = ref(null)
const allCustomers = ref([])
const moveTargetCustomer = ref(null)
const moving = ref(false)
const createCardProjectSlug = ref('')
const createCardColumnId = ref(null)
const createCardColumns = ref([])
const allProjects = ref([])
const creatingCard = ref(false)
const macros = ref([])
const applyingMacro = ref(false)
const checklistTemplates = ref([])
const applyingChecklist = ref(false)
const checklist = ref([])
const checklistListEl = ref(null)
let sortableInstance = null

const checklistPct = computed(() => {
  if (!checklist.value.length) return 0
  return Math.round(checklist.value.filter(i => i.is_completed).length / checklist.value.length * 100)
})

const checklistBlocksClose = computed(() =>
  checklist.value.some(i => !i.is_completed)
)

function syncChecklistFromTicket(t) {
  checklist.value = t?.checklist_items ? [...t.checklist_items] : []
}

function checklistApi() {
  if (isInbox.value) {
    return {
      apply: (templateId) => ticketChecklistsApi.applyInbox(ticketId.value, templateId),
      updateItem: (itemId, data) => ticketChecklistsApi.updateItemInbox(ticketId.value, itemId, data),
      deleteItem: (itemId) => ticketChecklistsApi.deleteItemInbox(ticketId.value, itemId),
      reorder: (items) => ticketChecklistsApi.reorderInbox(ticketId.value, items),
    }
  }
  return {
    apply: (templateId) => ticketChecklistsApi.apply(customerId.value, ticketId.value, templateId),
    updateItem: (itemId, data) => ticketChecklistsApi.updateItem(customerId.value, ticketId.value, itemId, data),
    deleteItem: (itemId) => ticketChecklistsApi.deleteItem(customerId.value, ticketId.value, itemId),
    reorder: (items) => ticketChecklistsApi.reorder(customerId.value, ticketId.value, items),
  }
}

// ── ITIL: classification, resolution, problem management ────────────────────
const TICKET_TYPES = ['incident', 'problem', 'service_request', 'change_request']
const LEVELS = ['low', 'medium', 'high']
const PROBLEM_STATUSES = ['investigating', 'root_cause_identified', 'known_error', 'resolved']
const LINK_TYPES = ['related', 'duplicate_of', 'caused_by', 'parent_of']
const RESOLVED_STATUSES = ['resolved', 'pending_close', 'closed']
const isResolvedStatus = (s) => RESOLVED_STATUSES.includes(s)

const helpdeskSettings = ref({ priority_matrix: {}, require_resolution_code: false })
const categories = ref([])
const resolutionCodes = ref([])
const resolveDialog = ref(null)
const resolveLinkedDialog = ref(null)
const resolving = ref(false)
const resolutionNotesDraft = ref('')
const problemDraft = ref({ root_cause: '', workaround: '' })

async function loadHelpdeskConfig() {
  const [s, c, r] = await Promise.allSettled([
    helpdeskApi.getSettings(), helpdeskApi.listCategories(), helpdeskApi.listResolutionCodes(),
  ])
  if (s.status === 'fulfilled') helpdeskSettings.value = s.value.data
  if (c.status === 'fulfilled') categories.value = c.value.data || []
  if (r.status === 'fulfilled') resolutionCodes.value = r.value.data || []
}

// Categories as "Parent › Child", parents first. The ticket's own category
// stays selectable even when it has since been deactivated.
const categoryOptions = computed(() => {
  const byId = Object.fromEntries(categories.value.map(c => [c.id, c]))
  const opts = categories.value.map(c => ({
    id: c.id,
    label: c.parent_id && byId[c.parent_id] ? `${byId[c.parent_id].name} › ${c.name}` : c.name,
  }))
  const own = ticket.value?.category
  if (own && !byId[own.id]) opts.push({ id: own.id, label: own.name })
  return opts.sort((a, b) => a.label.localeCompare(b.label))
})

function categoryLabel(id) {
  return categoryOptions.value.find(c => c.id === id)?.label || ticket.value?.category?.name || ''
}

const resolutionCodeOptions = computed(() => {
  const opts = [...resolutionCodes.value]
  const own = ticket.value?.resolution_code
  if (own && !opts.some(rc => rc.id === own.id)) opts.push(own)
  return opts
})

const derivedPriorityHint = computed(() => {
  const tk = ticket.value
  if (!tk?.impact || !tk?.urgency) return ''
  const p = helpdeskSettings.value.priority_matrix?.[`${tk.impact}:${tk.urgency}`]
  if (!p) return ''
  return p === tk.priority
    ? t('ticket.priority_derived', { priority: t('ticket.priority_' + p) })
    : t('ticket.priority_overridden', { priority: t('ticket.priority_' + p) })
})

watch(ticket, (tk) => {
  resolutionNotesDraft.value = tk?.resolution_notes || ''
  problemDraft.value = { root_cause: tk?.root_cause || '', workaround: tk?.workaround || '' }
})

async function saveFields(fields, successMsg = t('ticket.updated')) {
  try {
    const { data } = await apiUpdate(fields)
    ticket.value = data
    syncChecklistFromTicket(data)
    ui.success(successMsg)
    return true
  } catch (e) {
    ui.error(e.response?.data?.error || t('ticket.update_failed'))
    return false
  }
}

function saveResolutionNotes() {
  if (resolutionNotesDraft.value === (ticket.value?.resolution_notes || '')) return
  saveFields({ resolution_notes: resolutionNotesDraft.value })
}

function saveProblemText(field) {
  if (problemDraft.value[field] === (ticket.value?.[field] || '')) return
  saveFields({ [field]: problemDraft.value[field] })
}

// Resolving goes through a dialog that collects the resolution code and
// notes; closing directly only asks when a code is required and missing.
function onStatusSelect(event) {
  const status = event.target.value
  const tk = ticket.value
  const entersResolved = isResolvedStatus(status) && !isResolvedStatus(tk.status)
  const needsCode = helpdeskSettings.value.require_resolution_code && !tk.resolution_code_id
  if (entersResolved && (status === 'resolved' || needsCode)) {
    event.target.value = tk.status
    resolveDialog.value = { status, code: tk.resolution_code_id || 0, notes: tk.resolution_notes || '' }
    return
  }
  updateStatus(status)
}

async function confirmResolve() {
  const d = resolveDialog.value
  resolving.value = true
  const ok = await saveFields({ status: d.status, resolution_code_id: d.code || 0, resolution_notes: d.notes }, t('ticket.status_updated'))
  resolving.value = false
  if (ok) resolveDialog.value = null
}

const linkedIncidents = computed(() =>
  (linkedTickets.value || []).filter(l => l.link_type === 'caused_by' && l.direction === 'incoming')
)
const openLinkedIncidents = computed(() => linkedIncidents.value.filter(l => !isResolvedStatus(l.status)))

function openResolveLinked() {
  resolveLinkedDialog.value = { code: ticket.value?.resolution_code_id || 0, message: problemDraft.value.workaround || '' }
}

async function confirmResolveLinked() {
  const d = resolveLinkedDialog.value
  resolving.value = true
  try {
    const { data } = await helpdeskApi.resolveLinked(customerId.value, ticketId.value, {
      resolution_code_id: d.code || undefined, message: d.message,
    })
    ui.success(t('ticket.resolve_linked_done', { n: data.resolved, skipped: data.skipped }))
    resolveLinkedDialog.value = null
    const { data: links } = await ticketsApi.listLinks(customerId.value, ticketId.value)
    linkedTickets.value = links || []
  } catch (e) {
    ui.error(e.response?.data?.error || t('ticket.update_failed'))
  } finally {
    resolving.value = false
  }
}

function linkTypeLabel(lt) {
  const type = lt.link_type || 'related'
  return t((lt.direction === 'incoming' ? 'ticket.link_in_' : 'ticket.link_out_') + type)
}

async function useWorkaround(lt) {
  const text = lt.workaround.trim()
  newMessage.value = newMessage.value.trim() ? `${newMessage.value.trim()}\n\n${text}` : text
  msgEditorTab.value = 'edit'
  await nextTick()
  document.getElementById('ticket-msg-panel-edit')?.focus()
}

function apiUpdate(data) {
  return isInbox.value
    ? ticketsApi.inboxUpdate(ticketId.value, data)
    : ticketsApi.update(customerId.value, ticketId.value, data)
}

function startTitleEdit() {
  editTitleVal.value = ticket.value?.title || ''
  editingTitle.value = true
  nextTick(() => titleInput.value?.focus())
}

async function saveTitle() {
  const val = editTitleVal.value.trim()
  if (!val || val === ticket.value?.title) {
    editingTitle.value = false
    return
  }
  try {
    const { data } = await apiUpdate({ title: val })
    ticket.value = data
    ui.success('Title updated')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update title')
  }
  editingTitle.value = false
}

function cancelTitleEdit() {
  editingTitle.value = false
}

function titleWithDatePrefix(title, isoDate) {
  const stripped = title.replace(/^\[\d{4}-\d{2}-\d{2}\]\s*/, '')
  if (!isoDate) return stripped
  return `[${isoDate}] ${stripped}`
}

const reminderDateValue = computed(() => {
  if (!ticket.value?.reminder_at) return null
  const d = new Date(ticket.value.reminder_at)
  return d.toISOString().slice(0, 10)
})

async function updateReminderDate(val) {
  try {
    const d = val ? new Date(val + 'T12:00:00Z').toISOString() : null
    const newTitle = titleWithDatePrefix(ticket.value.title, val || null)
    const payload = { reminder_at: d }
    if (newTitle !== ticket.value.title) payload.title = newTitle
    const { data } = await apiUpdate(payload)
    ticket.value = data
    ui.success(val ? 'Reminder date updated' : 'Reminder cleared')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update reminder')
  }
}

const closeDateValue = computed(() => {
  if (!ticket.value?.close_at) return null
  const d = new Date(ticket.value.close_at)
  return d.toISOString().slice(0, 10)
})

async function updateCloseDate(val) {
  try {
    const d = val ? new Date(val + 'T12:00:00Z').toISOString() : null
    const newTitle = titleWithDatePrefix(ticket.value.title, val || null)
    const payload = { close_at: d }
    if (newTitle !== ticket.value.title) payload.title = newTitle
    const { data } = await apiUpdate(payload)
    ticket.value = data
    ui.success(val ? 'Close date updated' : 'Close date cleared')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update close date')
  }
}

async function fetchTicket() {
  loading.value = true
  pendingFiles.value = []
  newMessage.value = ''
  linkedTickets.value = null
  linkedCards.value = null
  try {
    const { data } = isInbox.value
      ? await ticketsApi.inboxGet(ticketId.value)
      : await ticketsApi.get(customerId.value, ticketId.value)
    ticket.value = data
    syncChecklistFromTicket(data)
    if (ticket.value.attachments === null) ticket.value.attachments = []
    if (ticket.value.messages) {
      initMessageAttachments(ticket.value.messages)
    }

    // Load customers for assign/move panel
    try {
      const { data: customers } = await customersApi.list()
      allCustomers.value = customers || []
    } catch {
      allCustomers.value = []
    }

    if (!isInbox.value) {
      // Fetch customer members for assignee dropdown
      try {
        const { data: members } = await client.get(`/customers/${customerId.value}/members`)
        customerUsers.value = members || []
      } catch {
        customerUsers.value = []
      }
      // Fetch customer projects for time tracking
      try {
        const { data: custDetail } = await customersApi.get(customerId.value)
        const contractProjects = (custDetail.contracts || []).flatMap(cg => cg.projects || [])
        const unassigned = custDetail.projects || []
        customerProjects.value = [...contractProjects, ...unassigned]
      } catch {
        customerProjects.value = []
      }
      // Fetch linked tickets
      try {
        const { data: links } = await ticketsApi.listLinks(customerId.value, ticketId.value)
        linkedTickets.value = links || []
      } catch {
        linkedTickets.value = []
      }
      // Fetch linked cards
      try {
        const { data: cards } = await ticketsApi.listCards(customerId.value, ticketId.value)
        linkedCards.value = cards || []
      } catch {
        linkedCards.value = []
      }
      // Fetch time entries
      await fetchTimeEntries()
      // Fetch history
      try {
        const { data: h } = await ticketsApi.getHistory(customerId.value, ticketId.value)
        history.value = h || []
      } catch {
        history.value = []
      }
      // Fetch viewers
      try {
        const { data: v } = await ticketsApi.getViewers(customerId.value, ticketId.value)
        viewers.value = v || []
      } catch {
        viewers.value = []
      }
    } else {
      // Inbox: fetch viewers
      try {
        const { data: v } = await ticketsApi.inboxGetViewers(ticketId.value)
        viewers.value = v || []
      } catch {
        viewers.value = []
      }
    }
  } catch {}
  loading.value = false
}

async function loadMacros() {
  try {
    const { data } = await macrosApi.list()
    macros.value = data || []
  } catch {
    macros.value = []
  }
}

async function loadChecklistTemplates() {
  try {
    const { data } = await ticketChecklistsApi.listTemplates()
    checklistTemplates.value = data || []
  } catch {
    checklistTemplates.value = []
  }
}

async function applyChecklist(templateId) {
  if (!templateId) return
  applyingChecklist.value = true
  try {
    const { data } = await checklistApi().apply(templateId)
    ticket.value = data
    syncChecklistFromTicket(data)
    ui.success(t('ticketChecklist.applied'))
  } catch (e) {
    ui.error(e.response?.data?.error || t('ticketChecklist.apply_failed'))
  } finally {
    applyingChecklist.value = false
  }
}

async function toggleChecklistItem(item) {
  const completed = !item.is_completed
  try {
    const { data } = await checklistApi().updateItem(item.id, { is_completed: completed })
    const idx = checklist.value.findIndex(i => i.id === item.id)
    if (idx !== -1) checklist.value[idx] = data
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update checklist item')
  }
}

async function removeChecklistItem(item) {
  try {
    await checklistApi().deleteItem(item.id)
    checklist.value = checklist.value.filter(i => i.id !== item.id)
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to delete checklist item')
  }
}

function initChecklistSortable() {
  if (!checklistListEl.value || sortableInstance) return
  sortableInstance = new Sortable(checklistListEl.value, {
    animation: 150,
    handle: '.checklist-drag-handle',
    onEnd(evt) {
      const moved = checklist.value.splice(evt.oldIndex, 1)[0]
      checklist.value.splice(evt.newIndex, 0, moved)
      const items = checklist.value.map((it, i) => ({ id: it.id, position: (i + 1) * 1000 }))
      checklist.value.forEach((it, i) => { it.position = (i + 1) * 1000 })
      checklistApi().reorder(items).catch(() => {})
    },
  })
}

watch(checklist, async (val) => {
  if (sortableInstance) {
    sortableInstance.destroy()
    sortableInstance = null
  }
  if (val.length) {
    await nextTick()
    initChecklistSortable()
  }
})

async function toggleSpam(markAsSpam) {
  try {
    const { data } = isInbox.value
      ? (markAsSpam ? await ticketsApi.inboxMarkSpam(ticketId.value) : await ticketsApi.inboxUnmarkSpam(ticketId.value))
      : (markAsSpam ? await ticketsApi.markSpam(customerId.value, ticketId.value) : await ticketsApi.unmarkSpam(customerId.value, ticketId.value))
    ticket.value = { ...ticket.value, ...data }
    ui.success(markAsSpam ? 'Marked as spam' : 'Removed from spam')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update spam status')
  }
}

async function applyMacro(macroId) {
  if (!macroId) return
  applyingMacro.value = true
  try {
    const { data } = isInbox.value
      ? await macrosApi.applyInbox(ticketId.value, macroId)
      : await macrosApi.apply(customerId.value, ticketId.value, macroId)
    ticket.value = data.ticket
    syncChecklistFromTicket(data.ticket)
    if (data.macro_messages?.length) {
      newMessage.value = data.macro_messages.join('\n\n')
      msgEditorTab.value = 'edit'
      await nextTick()
      document.getElementById('ticket-msg-panel-edit')?.focus()
    }
    ui.success('Macro applied')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to apply macro')
  } finally {
    applyingMacro.value = false
  }
}

watch([() => route.params.ticketId, () => route.name], () => {
  fetchTicket()
})

onMounted(() => { fetchTicket(); loadMacros(); loadChecklistTemplates(); loadHelpdeskConfig() })

// Refresh messages when IMAP delivers a reply to this ticket
async function refreshMessages() {
  try {
    const { data } = isInbox.value
      ? await ticketsApi.inboxGet(ticketId.value)
      : await ticketsApi.get(customerId.value, ticketId.value)
    if (data?.messages && ticket.value) {
      for (const m of data.messages) {
        if (m.attachments === null) m.attachments = []
      }
      ticket.value = { ...ticket.value, messages: data.messages }
    }
  } catch {}
}

watch(() => ticketsStore.ticketRefreshKey, () => {
  if (ticketsStore.refreshForTicketId === ticketId.value) {
    refreshMessages()
  }
})

async function updateStatus(status) {
  try {
    const payload = { status }
    if (status === 'pending' && ticket.value.reminder_at) {
      const iso = new Date(ticket.value.reminder_at).toISOString().slice(0, 10)
      const newTitle = titleWithDatePrefix(ticket.value.title, iso)
      if (newTitle !== ticket.value.title) payload.title = newTitle
    }
    if (status === 'pending_close' && ticket.value.close_at) {
      const iso = new Date(ticket.value.close_at).toISOString().slice(0, 10)
      const newTitle = titleWithDatePrefix(ticket.value.title, iso)
      if (newTitle !== ticket.value.title) payload.title = newTitle
    }
    const { data } = await apiUpdate(payload)
    ticket.value = data
    syncChecklistFromTicket(data)
    ui.success('Status updated')
    if (status === 'pending' || status === 'pending_close') {
      await nextTick()
      document.querySelector('.reminder-row')?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update status')
  }
}

async function updatePriority(priority) {
  try {
    const { data } = await apiUpdate({ priority })
    ticket.value = data
    ui.success('Priority updated')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update priority')
  }
}

async function updateAssignedTo(userId) {
  try {
    const val = userId ? Number(userId) : null
    const { data } = await ticketsApi.update(customerId.value, ticketId.value, { assigned_to_id: val })
    ticket.value = data
    ui.success('Assignee updated')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to update assignee')
  }
}

function onFilesSelected(files) {
  for (const f of files) {
    pendingFiles.value.push({
      id: Math.random(),
      filename: f.name,
      size_bytes: f.size,
      mime_type: f.type || 'application/octet-stream',
      _file: f,
      _previewUrl: f.type?.startsWith('image/') ? URL.createObjectURL(f) : null,
    })
  }
}

function removePending(a) {
  if (a._previewUrl) URL.revokeObjectURL(a._previewUrl)
  pendingFiles.value = pendingFiles.value.filter(p => p.id !== a.id)
}

async function onMsgPaste(e) {
  const items = Array.from(e.clipboardData?.items || [])
  const images = items.filter(it => it.kind === 'file' && it.type.startsWith('image/'))
  if (images.length) {
    e.preventDefault()
    const files = await Promise.all(images.map(it => {
      const file = it.getAsFile()
      if (file) return file
      return it.getType(it.type).then(blob => {
        const ext = it.type.split('/')[1]?.split('+')[0] || 'png'
        return new File([blob], `clipboard.${ext}`, { type: it.type })
      })
    }))
    const valid = files.filter(Boolean)
    if (valid.length) {
      onFilesSelected(valid)
      ui.success(valid.length > 1 ? `${valid.length} images pasted` : 'Image pasted')
    }
    return
  }
  if (window.__TAURI_INTERNALS__ && navigator.clipboard?.read) {
    try {
      const clipItems = await navigator.clipboard.read()
      const files = []
      for (const item of clipItems) {
        for (const type of item.types) {
          if (type.startsWith('image/')) {
            const blob = await item.getType(type)
            const ext = type.split('/')[1]?.split('+')[0] || 'png'
            files.push(new File([blob], `paste.${ext}`, { type }))
          }
        }
      }
      if (files.length) { e.preventDefault(); onFilesSelected(files); ui.success('Image pasted') }
    } catch {}
  }
}

async function submitMessage() {
  const body = newMessage.value.trim()
  if (!body && !pendingFiles.value.length) return
  sending.value = true
  try {
    const sendBody = body || '📎'
    let newMsg
    if (isInbox.value) {
      const { data } = await ticketsApi.inboxMessage(ticketId.value, sendBody, newMsgPrivate.value)
      newMsg = { ...data, attachments: [] }
    } else {
      const { data } = await ticketsApi.addMessage(customerId.value, ticketId.value, sendBody, newMsgPrivate.value)
      newMsg = { ...data, attachments: [] }
      if (pendingFiles.value.length) {
        const filesToUpload = [...pendingFiles.value]
        pendingFiles.value = []
        filesToUpload.forEach(pf => { if (pf._previewUrl) URL.revokeObjectURL(pf._previewUrl) })
        for (const pf of filesToUpload) {
          const fd = new FormData()
          fd.append('file', pf._file)
          fd.append('owner_type', 'ticket_message')
          fd.append('owner_id', String(newMsg.id))
          try {
            const { data: att } = await attachmentsApi.upload(fd)
            newMsg.attachments.push(att)
          } catch {}
        }
      }
    }

    if (!ticket.value.messages) ticket.value.messages = []
    ticket.value.messages.push(newMsg)
    newMessage.value = ''
    newMsgPrivate.value = false
    await nextTick()
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to send message')
  } finally {
    sending.value = false
  }
}

const topLevelMessages = computed(() =>
  (ticket.value?.messages || []).filter(m => !m.parent_id)
)

function flattenMessages(messages, depth = 0) {
  const result = []
  for (const msg of messages) {
    result.push({ msg, depth })
    if (msg.replies && msg.replies.length) {
      result.push(...flattenMessages(msg.replies, depth + 1))
    }
  }
  return result
}

const threadedMessages = computed(() => flattenMessages(topLevelMessages.value))

const replyingToMsgId = ref(null)
const replyText = ref('')
const sendingReply = ref(false)

function initMessageAttachments(messages) {
  for (const m of messages) {
    if (m.attachments === null) m.attachments = []
    if (m.replies) initMessageAttachments(m.replies)
  }
}

function findMessageById(msgId, messages = topLevelMessages.value) {
  for (const msg of messages) {
    if (msg.id === msgId) return msg
    if (msg.replies) {
      const found = findMessageById(msgId, msg.replies)
      if (found) return found
    }
  }
  return null
}

function startReply(msg) {
  replyingToMsgId.value = msg.id
  replyText.value = ''
}

function cancelReply() {
  replyingToMsgId.value = null
  replyText.value = ''
}

async function submitReply() {
  const body = replyText.value.trim()
  const targetId = replyingToMsgId.value
  if (!body || !targetId) return
  sendingReply.value = true
  try {
    const targetMsg = findMessageById(targetId)
    const isPrivate = targetMsg?.is_private || false
    let newMsg
    if (isInbox.value) {
      const { data } = await ticketsApi.inboxMessage(ticketId.value, body, isPrivate, targetId)
      newMsg = { ...data, attachments: [] }
    } else {
      const { data } = await ticketsApi.addMessage(customerId.value, ticketId.value, body, isPrivate, targetId)
      newMsg = { ...data, attachments: [] }
    }
    newMsg.user = auth.user
    if (targetMsg) {
      if (!targetMsg.replies) targetMsg.replies = []
      targetMsg.replies.push(newMsg)
    }
    cancelReply()
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to send reply')
  } finally {
    sendingReply.value = false
  }
}

async function togglePrivate(msg) {
  const newVal = !msg.is_private
  msg.is_private = newVal
  try {
    if (isInbox.value) {
      await ticketsApi.inboxUpdateMessage(ticketId.value, msg.id, { is_private: newVal })
    } else {
      await ticketsApi.updateMessage(customerId.value, ticketId.value, msg.id, { is_private: newVal })
    }
  } catch (e) {
    msg.is_private = !newVal
    ui.error(e.response?.data?.error || 'Failed to update message')
  }
}

async function addTag() {
  const name = newTagName.value.trim().replace(/^#/, '').toLowerCase()
  if (!name) return
  try {
    const { data } = await ticketsApi.addTag(customerId.value, ticketId.value, name)
    if (!ticket.value.tags) ticket.value.tags = []
    if (!ticket.value.tags.some(t => t.id === data.id)) {
      ticket.value.tags = [...ticket.value.tags, data]
    }
    newTagName.value = ''
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to add tag')
  }
}

async function removeTag(tag) {
  try {
    await ticketsApi.removeTag(customerId.value, ticketId.value, tag.id)
    ticket.value.tags = (ticket.value.tags || []).filter(t => t.id !== tag.id)
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to remove tag')
  }
}

async function addLinkedTicket() {
  const targetId = parseInt(newLinkId.value.trim(), 10)
  if (!targetId) return
  try {
    const { data } = await ticketsApi.addLink(customerId.value, ticketId.value, targetId, newLinkType.value)
    if (!linkedTickets.value) linkedTickets.value = []
    linkedTickets.value.push(data)
    newLinkId.value = ''
    newLinkType.value = 'related'
    ui.success('Ticket linked')
  } catch (e) {
    const msg = e.response?.data?.error || 'Failed to link ticket'
    ui.error(msg)
  }
}

async function removeLinkedTicket(lt) {
  try {
    await ticketsApi.removeLink(customerId.value, ticketId.value, lt.link_id)
    linkedTickets.value = (linkedTickets.value || []).filter(l => l.link_id !== lt.link_id)
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to remove link')
  }
}

async function addCardLink() {
  const val = newCardLink.value.trim()
  if (!val) return
  const body = /^\d+$/.test(val) ? { card_id: parseInt(val, 10) } : { ref: val }
  try {
    await ticketsApi.addCardLink(customerId.value, ticketId.value, body)
    const { data: cards } = await ticketsApi.listCards(customerId.value, ticketId.value)
    linkedCards.value = cards || []
    newCardLink.value = ''
    ui.success('Card linked')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to link card')
  }
}

async function removeCardLink(lc) {
  try {
    await ticketsApi.removeCardLink(customerId.value, ticketId.value, lc.link_id)
    linkedCards.value = (linkedCards.value || []).filter(c => c.link_id !== lc.link_id)
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to remove link')
  }
}

function openTicket(t) {
  const cid = t.customer_id || customerId.value
  router.push(`/customers/${cid}/tickets/${t.id}`)
}

function openCard(c) {
  router.push({ name: 'board', params: { slug: c.project_slug }, query: { card: c.card_id } })
}

async function loadCreateCardProjects() {
  if (allProjects.value.length) return
  try {
    const { data } = await projectsApi.list()
    allProjects.value = data || []
  } catch {
    allProjects.value = []
  }
}

async function loadCreateCardColumns() {
  await loadCreateCardProjects()
  if (!createCardProjectSlug.value) { createCardColumns.value = []; return }
  try {
    const { data } = await projectsApi.listColumns(createCardProjectSlug.value)
    createCardColumns.value = data || []
  } catch {
    createCardColumns.value = []
  }
  createCardColumnId.value = null
}

async function doCreateCard() {
  if (!createCardColumnId.value) return
  creatingCard.value = true
  try {
    const { data } = await ticketsApi.createCard(customerId.value, ticketId.value, {
      project_slug: createCardProjectSlug.value,
      column_id: createCardColumnId.value,
    })
    // Reload linked cards to show the new one
    const { data: cards } = await ticketsApi.listCards(customerId.value, ticketId.value)
    linkedCards.value = cards || []
    ui.success('Card created and linked')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to create card')
  } finally {
    creatingCard.value = false
  }
}

function slaWarning(t) {
  if (!t.sla_response_deadline && !t.sla_resolution_deadline) return false
  if (isResolvedStatus(t.status) || t.sla_paused_at) return false
  const now = Date.now()
  if (t.sla_response_deadline && !t.first_response_at) {
    const deadline = new Date(t.sla_response_deadline).getTime()
    if (deadline - now < 3600000 && deadline - now > 0) return true
  }
  if (t.sla_resolution_deadline) {
    const deadline = new Date(t.sla_resolution_deadline).getTime()
    if (deadline - now < 3600000 && deadline - now > 0) return true
  }
  return false
}

function activityIcon(type) {
  const icons = {
    created: '✦',
    status_changed: '⟳',
    title_changed: '✎',
    priority_changed: '⚑',
    type_changed: '▤',
    assignee_changed: '◉',
    owner_changed: '◎',
    group_changed: '⊞',
    comment_added: '✉',
    ticket_linked: '🔗',
    ticket_unlinked: '⛓',
    card_linked: '📋',
    card_unlinked: '📄',
    tag_added: '+',
    tag_removed: '−',
    dates_changed: '📅',
    customer_moved: '→',
    impact_changed: '⚑',
    urgency_changed: '⚑',
    category_changed: '▤',
    resolution_changed: '✓',
    major_incident_changed: '⚠',
    problem_status_changed: '⟳',
    root_cause_changed: '✎',
    workaround_changed: '✎',
    known_error_changed: '★',
    sla_warning: '⏰',
    sla_escalated: '⚠',
    linked_incidents_resolved: '✓',
  }
  return icons[type] || '•'
}

function activityLabel(h) {
  const t = (key) => { try { return h.detail || key } catch { return h.detail || key } }
  switch (h.event_type) {
    case 'created': return 'created this ticket'
    case 'status_changed': return `changed status to ${h.detail}`
    case 'title_changed': return `changed title to "${h.detail}"`
    case 'priority_changed': return `changed priority to ${h.detail}`
    case 'type_changed': return `changed type to ${h.detail}`
    case 'assignee_changed': return `changed assignee to ${h.detail}`
    case 'owner_changed': return `changed owner to ${h.detail}`
    case 'group_changed': return `changed group to ${h.detail}`
    case 'comment_added': return 'added a comment'
    case 'ticket_linked': return `linked ticket: ${h.detail}`
    case 'ticket_unlinked': return 'removed a ticket link'
    case 'card_linked': return 'linked a card'
    case 'card_unlinked': return 'unlinked a card'
    case 'tag_added': return `added tag #${h.detail}`
    case 'tag_removed': return `removed tag #${h.detail}`
    case 'dates_changed': return 'updated reminder / close dates'
    case 'customer_moved': return `moved to customer: ${h.detail}`
    case 'impact_changed': return `changed impact to ${h.detail || '—'}`
    case 'urgency_changed': return `changed urgency to ${h.detail || '—'}`
    case 'category_changed': return `changed category to ${h.detail || '—'}`
    case 'resolution_changed': return `set resolution code to ${h.detail || '—'}`
    case 'major_incident_changed': return h.detail === 'true' ? 'flagged as major incident' : 'removed major incident flag'
    case 'problem_status_changed': return `changed problem status to ${h.detail}`
    case 'root_cause_changed': return 'updated the root cause'
    case 'workaround_changed': return 'updated the workaround'
    case 'known_error_changed': return h.detail === 'true' ? 'marked as known error' : 'unmarked as known error'
    case 'sla_warning': return 'SLA warning threshold reached'
    case 'sla_escalated': return `SLA breached — escalated (${h.detail})`
    case 'linked_incidents_resolved': return `resolved ${h.detail} linked incident(s)`
    default: return h.detail || h.event_type
  }
}

// --- Time tracking ---
const ticketTimeEntries = ref([])
const customerProjects = ref([])
const teDate = ref(new Date().toISOString().slice(0, 10))
const teHours = ref(0)
const teMins = ref(0)
const teProjectId = ref(null)
const teDesc = ref('')
const teSubmitting = ref(false)

const totalMinutes = computed(() => ticketTimeEntries.value.reduce((s, e) => s + e.minutes, 0))

function formatMinutes(mins) {
  const h = Math.floor(mins / 60)
  const m = mins % 60
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

async function fetchTimeEntries() {
  if (!auth.timeTrackingEnabled) return
  try {
    const { data } = await timeEntriesApi.list({ ticket_id: ticketId.value })
    ticketTimeEntries.value = data || []
  } catch {
    ticketTimeEntries.value = []
  }
}

async function logTime() {
  const minutes = teHours.value * 60 + teMins.value
  if (minutes <= 0) return
  teSubmitting.value = true
  try {
    const { data } = await timeEntriesApi.create({
      ticket_id: ticketId.value,
      project_id: teProjectId.value || undefined,
      date: teDate.value,
      minutes,
      description: teDesc.value.trim(),
    })
    ticketTimeEntries.value.unshift(data)
    teHours.value = 0
    teMins.value = 0
    teProjectId.value = null
    teDesc.value = ''
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to log time')
  } finally {
    teSubmitting.value = false
  }
}

async function deleteTimeEntry(te) {
  if (!await ui.confirm('Delete this time entry?', { destructive: true })) return
  try {
    await timeEntriesApi.remove(te.id)
    ticketTimeEntries.value = ticketTimeEntries.value.filter(e => e.id !== te.id)
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to delete time entry')
  }
}

async function copyTicketRef() {
  const text = `Ticket#${ticket.value.id}`
  try {
    await navigator.clipboard.writeText(text)
    ui.success('Copied ' + text)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    ui.success('Copied ' + text)
  }
}

async function deleteTicket() {
  if (!await ui.confirm('Delete this ticket?', { destructive: true })) return
  try {
    if (isInbox.value) {
      await ticketsApi.inboxDelete(ticketId.value)
      router.push('/tickets/inbox')
    } else {
      await ticketsApi.delete(customerId.value, ticketId.value)
      router.push(`/customers/${customerId.value}/tickets`)
    }
    ui.success('Ticket deleted')
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to delete ticket')
  }
}

async function moveTicket() {
  if (!moveTargetCustomer.value || moveTargetCustomer.value === customerId.value) return
  moving.value = true
  try {
    await ticketsApi.move(customerId.value, ticketId.value, moveTargetCustomer.value)
    ui.success('Ticket moved')
    router.push(`/customers/${moveTargetCustomer.value}/tickets/${ticketId.value}`)
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to move ticket')
  } finally {
    moving.value = false
  }
}

async function assignToCustomer() {
  if (!moveTargetCustomer.value) return
  moving.value = true
  try {
    await ticketsApi.inboxUpdate(ticketId.value, { customer_id: moveTargetCustomer.value })
    ui.success('Ticket assigned')
    router.push(`/customers/${moveTargetCustomer.value}/tickets/${ticketId.value}`)
  } catch (e) {
    ui.error(e.response?.data?.error || 'Failed to assign ticket')
  } finally {
    moving.value = false
  }
}
</script>

<style scoped>
.ticket-detail-main { padding: 24px; max-width: 1400px; margin: 0 auto; }
.loading-state { display: flex; justify-content: center; padding: 48px; }
.empty-state { text-align: center; padding: 64px 24px; color: var(--color-text-muted); }
.back-link { font-size: 13px; color: var(--color-primary); text-decoration: none; }
.back-link:hover { text-decoration: underline; }
.detail-header { margin-bottom: 24px; }
.spam-banner { background: #fef2f2; border: 1px solid #fecaca; border-radius: 6px; padding: 8px 14px; font-size: 13px; font-weight: 600; color: #b91c1c; margin: 6px 0; }
[data-theme="dark"] .spam-banner { background: #3b1010; border-color: #7f1d1d; color: #fca5a5; }
.detail-title-row { display: flex; align-items: center; gap: 12px; margin: 8px 0; }
.detail-title-row h1 { flex: 1; margin: 0; font-size: 22px; }
.detail-actions { display: flex; gap: 8px; }
.detail-meta { display: flex; gap: 12px; align-items: center; font-size: 12px; color: var(--color-text-muted); flex-wrap: wrap; }
.from-email-badge { font-size: 11px; color: var(--color-text-muted); font-style: italic; }
.ticket-status { padding: 2px 8px; border-radius: 4px; font-weight: 600; font-size: 12px; }
.status-new { background: #dbeafe; color: #1e40af; }
.status-open { background: #fef3c7; color: #92400e; }
.status-pending { background: #f0e6ff; color: #6b21a8; }
.status-pending_close { background: #d1fae5; color: #065f46; }
.status-closed { background: #e5e7eb; color: #374151; }
.status-in_progress { background: color-mix(in srgb, var(--color-primary) 18%, transparent); color: var(--color-text); }
.status-resolved { background: color-mix(in srgb, var(--color-success) 22%, transparent); color: var(--color-text); }
.major-incident-badge,
.known-error-badge,
.reopen-badge,
.category-chip { font-size: 11px; font-weight: 600; padding: 1px 7px; border-radius: 4px; border: 1px solid var(--color-border); color: var(--color-text); }
.major-incident-badge { background: color-mix(in srgb, var(--color-danger) 18%, transparent); border-color: var(--color-danger); }
.known-error-badge { background: color-mix(in srgb, var(--color-warning) 22%, transparent); border-color: var(--color-warning); }
.reopen-badge { background: color-mix(in srgb, var(--color-warning) 12%, transparent); }
.category-chip { background: var(--color-surface); }
.itil-panel {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 14px 16px;
  margin-bottom: 16px;
}
.itil-panel h4 { margin: 0 0 10px; font-size: 14px; }
.itil-panel h5 { margin: 14px 0 8px; font-size: 13px; }
.itil-grid { display: flex; flex-wrap: wrap; gap: 12px 16px; align-items: flex-end; }
.itil-field { display: flex; flex-direction: column; gap: 4px; font-size: 12px; min-width: 150px; }
.itil-field > label { color: var(--color-text-muted); font-weight: 600; }
.itil-field-wide { flex: 1 1 100%; }
.itil-field-check { flex-direction: row; align-items: center; min-width: 0; }
.itil-hint { font-size: 12px; color: var(--color-text-muted); margin: 8px 0 0; }
.problem-panel { border-left: 3px solid var(--color-warning); }
.problem-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 12px; }
.problem-actions .itil-hint { margin: 0; }
.linked-type { font-size: 11px; color: var(--color-text-muted); white-space: nowrap; }
.sla-paused { font-size: 12px; color: var(--color-text-muted); margin: 0 0 8px; }
.resolve-form { display: flex; flex-direction: column; gap: 6px; }
.resolve-form label { font-size: 13px; font-weight: 600; margin-top: 6px; }
.detail-desc-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}
.detail-desc-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--color-text-muted);
}
.from-email-link {
  font-size: 12px;
  color: var(--color-primary);
  text-decoration: none;
}
.from-email-link:hover {
  text-decoration: underline;
}
.from-name {
  font-size: 13px;
  color: var(--color-text);
  font-weight: 500;
  margin-left: 8px;
}
.email-body {
  font-size: 14px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--color-text);
}
.raw-email-pre {
  font-size: 12px;
  line-height: 1.4;
  white-space: pre;
  word-break: normal;
  overflow-x: auto;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: 4px;
  padding: 12px;
  max-height: 70vh;
  margin: 0;
  font-family: var(--font-mono);
}
.detail-description {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 24px;
}
.msg-author-group {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.email-sent-badge {
  font-size: 13px;
  color: var(--color-primary);
  cursor: default;
}
.messages-section h2 { font-size: 16px; margin: 0 0 16px; }
.messages-list { display: flex; flex-direction: column; gap: 12px; margin-bottom: 16px; }
.message {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 12px;
}
.message--private {
  background: #fffbeb;
  border-color: #f59e0b;
}
[data-theme="dark"] .message--private {
  background: rgba(245, 158, 11, 0.08);
  border-color: #d97706;
}
.private-badge {
  font-size: 11px;
  font-weight: 700;
  color: #92400e;
  background: #fde68a;
  padding: 1px 6px;
  border-radius: 3px;
}
[data-theme="dark"] .private-badge {
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
}
.private-checkbox-label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-muted);
  cursor: pointer;
  margin-right: auto;
  user-select: none;
}
.private-checkbox-label:has(input:checked) { color: #92400e; }
[data-theme="dark"] .private-checkbox-label:has(input:checked) { color: #fbbf24; }
.message-actions { display: flex; gap: 4px; margin-top: 8px; }
.btn-ghost.btn-xs { background: none; border: none; cursor: pointer; font-size: 11px; font-weight: 600; color: var(--color-text-muted); padding: 2px 6px; border-radius: 4px; }
.btn-ghost.btn-xs:hover { background: var(--color-bg-alt); color: var(--color-text); }
.message-reply-form { margin-bottom: 12px; padding: 8px; background: var(--color-bg-alt); border-radius: 6px; }
.reply-textarea { width: 100%; resize: vertical; min-height: 60px; font-size: 13px; }
.reply-form-actions { display: flex; gap: 6px; justify-content: flex-end; margin-top: 6px; }
.message-header { display: flex; justify-content: space-between; font-size: 12px; margin-bottom: 6px; }
.message-time { color: var(--color-text-muted); }
.message-body { font-size: 14px; }
.message-form { display: flex; flex-direction: column; gap: 8px; }
.message-form-actions { display: flex; align-items: center; gap: 8px; justify-content: flex-end; }
.form-input-sm { height: 28px; font-size: 12px; padding: 4px 8px; }
.pending-badge { font-size: 11px; color: var(--color-primary); font-weight: 600; }
.pending-attachments { background: var(--color-bg-alt); border-radius: 6px; padding: 8px; margin-top: 4px; }
.pending-label { font-size: 11px; font-weight: 600; color: var(--color-text-muted); display: block; margin-bottom: 4px; }
.sla-badge { font-size: 10px; font-weight: 700; padding: 2px 6px; border-radius: 4px; text-transform: uppercase; }
.sla-ok { background: #d1fae5; color: #065f46; }
.sla-warning { background: #fef3c7; color: #92400e; }
.sla-breach { background: #fecaca; color: #b91c1c; }
.sla-deadline { font-size: 12px; color: var(--color-text-muted); }
.sla-deadline-breach { color: #b91c1c; font-weight: 600; }
.sla-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 14px 16px;
  margin-bottom: 24px;
}
.sla-card h4 { margin: 0 0 10px; font-size: 14px; }
.sla-card-body { display: flex; flex-direction: column; gap: 6px; }
.sla-card-row { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.sla-card-label { color: var(--color-text-muted); min-width: 100px; }
.sla-card-value { font-weight: 600; }
.sla-card-breach { color: #b91c1c; }
.sla-card-status { font-size: 11px; font-weight: 700; padding: 1px 6px; border-radius: 3px; text-transform: uppercase; }
.sla-card-status.ok { background: #d1fae5; color: #065f46; }
.sla-card-status.warn { background: #fef3c7; color: #92400e; }
.sla-card-status.breach { background: #fecaca; color: #b91c1c; }
.sla-card-check { font-size: 11px; color: var(--color-text-muted); }
.ticket-id-copy { cursor: pointer; font-variant-numeric: tabular-nums; }
.ticket-id-copy:hover { color: var(--color-primary); }
.ticket-type { font-size: 11px; font-weight: 600; padding: 2px 8px; border-radius: 4px; }
.type-incident { background: #fecaca; color: #b91c1c; }
.type-problem { background: #fed7aa; color: #9a3412; }
.type-service_request { background: #d1fae5; color: #065f46; }
.type-change_request { background: #dbeafe; color: #1e40af; }
.detail-fields { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-bottom: 16px; }
.detail-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.tag-chip { display: inline-flex; align-items: center; gap: 3px; font-size: 12px; background: var(--color-bg-alt); padding: 2px 8px; border-radius: 4px; }
.tag-remove { background: none; border: none; cursor: pointer; font-size: 14px; line-height: 1; color: var(--color-text-muted); padding: 0; }
.tag-input-row { display: flex; gap: 4px; align-items: center; }
.tag-input { flex: 1; min-width: 120px; }
.linked-tickets-section { margin-bottom: 24px; }
.linked-tickets-section h4 { font-size: 14px; margin: 0 0 8px; display: flex; align-items: center; gap: 6px; }
.linked-tickets-section .count { font-weight: 400; color: var(--color-text-muted); }
.linked-list { display: flex; flex-direction: column; gap: 4px; margin-bottom: 8px; }
.linked-row { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-radius: 6px; cursor: pointer; font-size: 13px; }
.linked-row:hover { background: var(--color-bg-alt); }
.linked-row .linked-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.linked-status { padding: 1px 5px; border-radius: 3px; font-size: 10px; font-weight: 600; }
.linked-ref { font-family: monospace; font-size: 11px; font-weight: 700; color: var(--color-primary); min-width: 70px; }
.linked-priority { font-size: 10px; font-weight: 600; color: var(--color-text-muted); }
.linked-add-row { display: flex; gap: 4px; }
.linked-add-row input { flex: 1; }
.btn-icon-xs { background: none; border: none; cursor: pointer; color: var(--color-text-muted); padding: 2px; font-size: 12px; line-height: 1; }
.btn-icon-xs:hover { color: var(--color-danger, #ef4444); }

/* Markdown editor tabs (matches AdminView pattern) */
:deep(.md-editor) { border: 1px solid var(--color-border); border-radius: var(--radius); overflow: hidden; }
:deep(.md-editor-tabs) { display: flex; background: var(--color-bg-alt); border-bottom: 1px solid var(--color-border); }
:deep(.md-tab) { padding: 6px 16px; font-size: 12px; font-weight: 600; cursor: pointer; background: none; border: none; border-bottom: 2px solid transparent; color: var(--color-text-muted); transition: color .15s, border-color .15s; }
:deep(.md-tab:hover) { color: var(--color-text); }
:deep(.md-tab.active) { color: var(--color-primary); border-bottom-color: var(--color-primary); }
:deep(.md-textarea) { border: none !important; border-radius: 0 !important; resize: vertical; min-height: 180px; }
:deep(.md-preview) { padding: 10px 12px; min-height: 180px; }

.assignee-wrap { display: inline-flex; align-items: center; }
.assignee-wrap select { font-size: 12px; padding: 1px 6px; height: 22px; max-width: 140px; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); border: 0; }
.title-edit-wrap h1 { cursor: pointer; }
.title-edit-wrap h1:hover { background: var(--color-bg-alt); border-radius: 4px; }
.title-input { font-size: 22px; font-weight: 700; width: 100%; }
.reminder-row { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; padding: 8px 12px; background: #f0e6ff; border-radius: 6px; }
.reminder-label { font-size: 12px; font-weight: 600; color: #6b21a8; }
.reminder-row :deep(.dp-trigger) { color: #6b21a8; border-color: color-mix(in srgb, #6b21a8 30%, transparent); background: transparent; font-weight: 600; }
.reminder-row :deep(.dp-trigger:hover) { background: color-mix(in srgb, #6b21a8 10%, transparent); }
.reminder-row :deep(.dp-trigger-empty) { color: #6b21a8; opacity: 0.6; }

/* Time tracking section */
.time-section { margin-bottom: 24px; }
.time-section h4 { font-size: 14px; margin: 0 0 8px; display: flex; align-items: center; gap: 6px; }
.time-section .count { font-weight: 400; color: var(--color-text-muted); }
.time-list { display: flex; flex-direction: column; gap: 4px; margin-bottom: 8px; }
.time-row { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-radius: 6px; font-size: 13px; background: var(--color-bg); border: 1px solid var(--color-border); }
.time-date { color: var(--color-text-muted); min-width: 80px; font-size: 12px; }
.time-who { color: var(--color-text-muted); min-width: 80px; font-size: 12px; }
.time-dur { font-weight: 600; min-width: 50px; }
.time-desc { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.time-proj { font-size: 11px; color: var(--color-primary); background: var(--color-bg-alt); padding: 1px 6px; border-radius: 3px; white-space: nowrap; }
.time-empty { font-size: 13px; color: var(--color-text-muted); margin: 0 0 8px; }
.time-add-form { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.te-num { width: 60px; text-align: center; }
.te-project { min-width: 130px; max-width: 180px; }
.te-desc { flex: 1; min-width: 120px; }

.viewers-section { margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.viewers-row { display: flex; flex-wrap: wrap; gap: 6px; }
.viewer-wrap { position: relative; display: inline-flex; }
.viewer-avatar { width: 28px; height: 28px; border-radius: 50%; display: flex; align-items: center; justify-content: center; cursor: default; flex-shrink: 0; border: 2px solid var(--color-surface, #fff); }
.viewer-avatar-img { width: 100%; height: 100%; border-radius: 50%; object-fit: cover; }
.viewer-initials { font-size: 10px; font-weight: 700; color: #fff; line-height: 1; }
.viewer-tooltip { display: none; position: absolute; bottom: calc(100% + 6px); left: 50%; transform: translateX(-50%); background: var(--color-surface-raised, #1e1e2e); color: var(--color-text); border: 1px solid var(--color-border); border-radius: 6px; padding: 6px 10px; white-space: nowrap; font-size: 12px; box-shadow: 0 4px 12px rgba(0,0,0,.15); z-index: 100; pointer-events: none; }
.viewer-tooltip::after { content: ''; position: absolute; top: 100%; left: 50%; transform: translateX(-50%); border: 5px solid transparent; border-top-color: var(--color-border); }
.viewer-wrap:hover .viewer-tooltip, .viewer-wrap:focus-within .viewer-tooltip { display: flex; flex-direction: column; gap: 2px; }
.viewer-tooltip-name { font-weight: 600; }
.viewer-tooltip-date { color: var(--color-text-muted); font-size: 11px; }

.activity-section { margin-top: 24px; border-top: 1px solid var(--color-border); padding-top: 20px; }
.activity-toggle { display: flex; align-items: center; gap: 6px; width: 100%; background: none; border: none; cursor: pointer; font-size: 14px; color: var(--color-text-muted); padding: 4px 0; margin-bottom: 12px; }
.activity-toggle:hover { color: var(--color-text); }
.activity-toggle-icon { transition: transform .15s; font-size: 10px; }
.activity-toggle-icon.rotated { transform: rotate(90deg); }
.activity-count { font-weight: 400; color: var(--color-text-muted); }
.activity-empty { font-size: 12px; color: var(--color-text-muted); }
.activity-list { display: flex; flex-direction: column; gap: 6px; }
.activity-entry { display: flex; align-items: baseline; gap: 8px; font-size: 12px; }
.activity-icon { font-size: 11px; flex-shrink: 0; width: 14px; text-align: center; }
.activity-time { color: var(--color-text-muted); flex-shrink: 0; }
.activity-who { font-weight: 600; flex-shrink: 0; }
.activity-detail { color: var(--color-text-muted); }
.activity-type-created { color: #22c55e; }
.activity-type-status_changed { color: var(--color-primary); }
.activity-type-priority_changed { color: #f59e0b; }
.activity-type-title_changed,
.activity-type-type_changed,
.activity-type-assignee_changed,
.activity-type-owner_changed,
.activity-type-group_changed,
.activity-type-comment_added,
.activity-type-ticket_linked,
.activity-type-ticket_unlinked,
.activity-type-card_linked,
.activity-type-card_unlinked,
.activity-type-tag_added,
.activity-type-tag_removed,
.activity-type-dates_changed,
.activity-type-customer_moved { color: var(--color-text-muted); }

.move-section { margin-bottom: 24px; }
.move-section h4 { font-size: 14px; margin: 0 0 8px; }
.move-row { display: flex; gap: 6px; align-items: center; }
.move-row select { flex: 1; max-width: 280px; }

.checklist-section { margin-bottom: 24px; border-top: 1px solid var(--color-border); padding-top: 20px; }
.checklist-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.checklist-header h4 { margin: 0; font-size: 14px; }
.checklist-progress { font-size: 12px; font-weight: 600; color: var(--color-text-muted); }
.checklist-close-hint { font-size: 12px; color: var(--color-warning, #92400e); margin: 0 0 8px; }
.checklist-progress-bar { height: 4px; background: var(--color-border); border-radius: 2px; margin-bottom: 12px; overflow: hidden; }
.checklist-progress-fill { height: 100%; background: var(--color-primary); border-radius: 2px; transition: width .3s; }
.checklist-items { display: flex; flex-direction: column; gap: 4px; }
.checklist-item { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
.checklist-drag-handle { color: var(--color-text-muted); cursor: grab; font-size: 14px; flex-shrink: 0; user-select: none; }
.checklist-drag-handle:active { cursor: grabbing; }
.checklist-drag-handle:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.checklist-checkbox { width: 15px; height: 15px; cursor: pointer; flex-shrink: 0; accent-color: var(--color-primary); }
.checklist-body { flex: 1; font-size: 13px; line-height: 1.4; }
.checklist-body.completed { text-decoration: line-through; color: var(--color-text-muted); }
.checklist-item .btn-icon-xs:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
</style>
