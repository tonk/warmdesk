import client, { fetchBinary } from './client'

// ITIL helpdesk configuration, problem management and reporting.
export const helpdeskApi = {
  // Readable by every helpdesk user: { priority_matrix: {"impact:urgency": priority},
  // resolved_autoclose_days, require_resolution_code, major_incident_group_id }
  getSettings:          ()      => client.get('/helpdesk/settings'),
  updateSettings:       (data)  => client.put('/admin/helpdesk/settings', data),

  // Active categories / resolution codes, for ticket forms.
  listCategories:       ()      => client.get('/ticket-categories'),
  listResolutionCodes:  ()      => client.get('/ticket-resolution-codes'),

  // Admin CRUD (lists include inactive entries).
  adminListCategories:  ()         => client.get('/admin/ticket-categories'),
  createCategory:       (data)     => client.post('/admin/ticket-categories', data),
  updateCategory:       (id, data) => client.put(`/admin/ticket-categories/${id}`, data),
  deleteCategory:       (id)       => client.delete(`/admin/ticket-categories/${id}`),
  adminListResolutionCodes: ()         => client.get('/admin/ticket-resolution-codes'),
  createResolutionCode:     (data)     => client.post('/admin/ticket-resolution-codes', data),
  updateResolutionCode:     (id, data) => client.put(`/admin/ticket-resolution-codes/${id}`, data),
  deleteResolutionCode:     (id)       => client.delete(`/admin/ticket-resolution-codes/${id}`),

  // Business calendars: { name, timezone, work_days: "1,2,3,4,5" (0 = Sunday),
  // start_time: "09:00", end_time: "17:00", holidays: [{date: "YYYY-MM-DD", name}] }
  listCalendars:        ()         => client.get('/admin/business-calendars'),
  createCalendar:       (data)     => client.post('/admin/business-calendars', data),
  updateCalendar:       (id, data) => client.put(`/admin/business-calendars/${id}`, data),
  deleteCalendar:       (id)       => client.delete(`/admin/business-calendars/${id}`),
  importHolidays:       (id, year, locale) => client.post(`/admin/business-calendars/${id}/import-holidays`, { year, locale }),

  // Problem records across accessible customers; params: { known_error, q, include_resolved }
  listProblems:         (params) => client.get('/problems', { params }),
  resolveLinked:        (cid, id, data) => client.post(`/customers/${cid}/tickets/${id}/resolve-linked`, data),

  // KPI report; params: { from, to, customer_id, group_id, assigned_to_id, type, category_id, priority }
  ticketReport:         (params) => client.get('/helpdesk/reports/tickets', { params }),
  ticketReportPdf:      (params) => fetchBinary('/helpdesk/reports/tickets/pdf', params),
  ticketReportXlsx:     (params) => fetchBinary('/helpdesk/reports/tickets/xlsx', params),
}
