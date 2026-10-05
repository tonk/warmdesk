import client from './client'

// Knowledge base. Agents see every article (drafts included); customer-portal
// users only published public articles (global or for their customers).
// Article: { id, title, summary, body (markdown, omitted in lists), status:
// draft|published|archived, visibility: internal|public, customer_id,
// category_id, keywords, author, updated_by, views, helpful_yes, helpful_no,
// published_at, created_at, updated_at, category, customer }
export const kbApi = {
  list:     (params)    => client.get('/kb/articles', { params }),   // { q, status, visibility, category_id, customer_id, include_archived }
  suggest:  (params)    => client.get('/kb/suggest', { params }),    // { q, customer_id, limit }
  get:      (id, countView = true) => client.get(`/kb/articles/${id}`, { params: countView ? {} : { count_view: false } }),
  create:   (data)      => client.post('/kb/articles', data),        // data.ticket_id links the new article to that ticket
  update:   (id, data)  => client.put(`/kb/articles/${id}`, data),
  remove:   (id)        => client.delete(`/kb/articles/${id}`),      // author or admin
  feedback: (id, helpful) => client.post(`/kb/articles/${id}/feedback`, { helpful }),
  ticketArticles: (cid, tid)        => client.get(`/customers/${cid}/tickets/${tid}/articles`),
  linkArticle:    (cid, tid, aid)   => client.post(`/customers/${cid}/tickets/${tid}/articles`, { article_id: aid }),
  unlinkArticle:  (cid, tid, aid)   => client.delete(`/customers/${cid}/tickets/${tid}/articles/${aid}`),
}

// Service catalogue. Item: { id, name, description, icon, category_id,
// customer_id, fields: [{key, label, type: text|textarea|select|checkbox|date|number,
// required, options, help}], default_priority, group_id, approval_group_id,
// approvals_required, checklist_template_id, is_active, sort_order }
export const catalogApi = {
  list:        (params)    => client.get('/service-catalog', { params }), // { customer_id }
  adminList:   ()          => client.get('/admin/service-catalog'),
  create:      (data)      => client.post('/admin/service-catalog', data),
  update:      (id, data)  => client.put(`/admin/service-catalog/${id}`, data),
  remove:      (id)        => client.delete(`/admin/service-catalog/${id}`),
  // Order an item: { catalog_item_id, title?, description?, answers: {key: value} } → ticket
  order:       (cid, data) => client.post(`/customers/${cid}/service-requests`, data),
}

// Approvals (service requests and changes). Ticket fields: approval_status
// (''|pending|approved|rejected), approval_group_id, approvals_required, approval_round.
export const approvalsApi = {
  mine:     ()                       => client.get('/approvals'),   // pending rounds I can decide on
  decide:   (ticketId, decision, comment) => client.post(`/approvals/${ticketId}`, { decision, comment }), // reject needs a comment
  forTicket: (cid, tid)              => client.get(`/customers/${cid}/tickets/${tid}/approvals`),
  request:  (cid, tid, groupId, required) => client.post(`/customers/${cid}/tickets/${tid}/approvals/request`, { group_id: groupId, approvals_required: required }),
}

// Change enablement. Change fields live on the ticket and are saved through
// ticketsApi.update: change_type (standard|normal|emergency), change_status
// (draft|awaiting_approval|approved|scheduled|implementing|review|completed|failed|cancelled),
// risk (low|medium|high), planned_start, planned_end (RFC 3339 or null),
// implementation_plan, backout_plan, test_plan, review_notes,
// change_outcome (successful|partially_successful|failed).
// Transition errors come back as 400 with a `code`: change_incomplete,
// no_cab_group, approval_required, planned_window_required, change_freeze,
// outcome_required, invalid_change_transition, change_not_draft.
export const changesApi = {
  calendar:     (params)   => client.get('/changes', { params }),   // { from, to, unscheduled } → { changes, freezes }
  check:        (cid, tid) => client.get(`/customers/${cid}/tickets/${tid}/change/check`), // { freezes, conflicts }
  freezes:      (params)   => client.get('/change-freezes', { params }), // { all }
  createFreeze: (data)     => client.post('/admin/change-freezes', data), // { name, start_at, end_at, customer_id }
  updateFreeze: (id, data) => client.put(`/admin/change-freezes/${id}`, data),
  deleteFreeze: (id)       => client.delete(`/admin/change-freezes/${id}`),
}

// CMDB (agents only). CI: { id, name, type_id, customer_id (null = internal),
// status: planned|active|maintenance|retired, environment:
// production|acceptance|test|development|'', criticality: low|medium|high|'',
// owner_id, group_id, description, attributes: {k: v}, type, customer, owner, group }
// Relation types, read "source <type> target": depends_on, runs_on, part_of, connected_to.
export const cmdbApi = {
  types:          ()          => client.get('/cmdb/types'),
  createType:     (data)      => client.post('/admin/ci-types', data),
  updateType:     (id, data)  => client.put(`/admin/ci-types/${id}`, data),
  deleteType:     (id)        => client.delete(`/admin/ci-types/${id}`),
  list:           (params)    => client.get('/cmdb/cis', { params }), // { q, type_id, customer_id ('internal' or id), status, include_retired }
  get:            (id)        => client.get(`/cmdb/cis/${id}`),       // { ci, relations: [{id, type, direction, ci}], tickets }
  create:         (data)      => client.post('/cmdb/cis', data),
  update:         (id, data)  => client.put(`/cmdb/cis/${id}`, data),
  remove:         (id)        => client.delete(`/cmdb/cis/${id}`),
  impact:         (id)        => client.get(`/cmdb/cis/${id}/impact`), // { impacted: [{ci, depth, via, open_tickets}], hidden, open_tickets }
  addRelation:    (id, targetId, type) => client.post(`/cmdb/cis/${id}/relations`, { target_id: targetId, type }),
  removeRelation: (relId)     => client.delete(`/cmdb/relations/${relId}`),
  ticketCIs:      (cid, tid)  => client.get(`/customers/${cid}/tickets/${tid}/cis`),
  linkCI:         (cid, tid, ciId) => client.post(`/customers/${cid}/tickets/${tid}/cis`, { ci_id: ciId }),
  unlinkCI:       (cid, tid, ciId) => client.delete(`/customers/${cid}/tickets/${tid}/cis/${ciId}`),
}
