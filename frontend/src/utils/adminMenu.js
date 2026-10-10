// The admin panel's menu: groups of sections, each reachable at
// /admin/<group path>/<item path>. `key` is the section's internal name
// (panel id tab-panel-<key>, help context admin.<key>). Shared by
// AdminView.vue (menu and data loading) and App.vue (breadcrumbs).
export const ADMIN_MENU = [
  { key: 'people', path: 'people', labelKey: 'adminNav.group_people', items: [
    { key: 'users', path: 'users', labelKey: 'admin.users' },
    { key: 'groups', path: 'groups', labelKey: 'groups.title' },
  ] },
  { key: 'customers', path: 'customers', labelKey: 'adminNav.group_customers', items: [
    { key: 'customers', path: 'customers', labelKey: 'customer.customers' },
    { key: 'invoice-templates', path: 'invoice-templates', labelKey: 'admin.invoice_templates' },
  ] },
  { key: 'work', path: 'work', labelKey: 'adminNav.group_work', items: [
    { key: 'projects', path: 'projects', labelKey: 'admin.projects', hideInTimetracking: true, hideInTicketsystem: true },
    { key: 'time-tracking', path: 'time-tracking', labelKey: 'admin.time_tracking', hideInTicketsystem: true },
  ] },
  { key: 'helpdesk', path: 'helpdesk', labelKey: 'adminNav.group_helpdesk', hideInTimetracking: true, items: [
    { key: 'helpdesk', path: 'ticket-settings', labelKey: 'adminNav.ticket_settings' },
    { key: 'sla', path: 'sla-policies', labelKey: 'sla.title' },
    { key: 'business-calendars', path: 'business-hours', labelKey: 'helpdeskAdmin.business_hours' },
    { key: 'macros', path: 'macros', labelKey: 'macro.title' },
    { key: 'ticket-checklists', path: 'checklists', labelKey: 'ticketChecklist.title' },
    { key: 'service-catalog', path: 'service-catalogue', labelKey: 'catalogAdmin.title' },
  ] },
  { key: 'change', path: 'change', labelKey: 'adminNav.group_change', hideInTimetracking: true, items: [
    { key: 'change-management', path: 'change-management', labelKey: 'changeAdmin.title' },
    { key: 'cmdb-types', path: 'cmdb-types', labelKey: 'cmdbAdmin.title' },
  ] },
  { key: 'system', path: 'system', labelKey: 'adminNav.group_system', items: [
    { key: 'settings', path: 'settings', labelKey: 'admin.settings' },
    { key: 'news', path: 'news', labelKey: 'admin.news_tab' },
    { key: 'backup', path: 'backup', labelKey: 'admin.backup_tab' },
  ] },
]

// visibleAdminMenu drops the groups and items hidden in the given app mode
// ('' | 'timetracking' | 'ticketsystem').
export function visibleAdminMenu(appMode) {
  const hidden = x => (appMode === 'timetracking' && x.hideInTimetracking) || (appMode === 'ticketsystem' && x.hideInTicketsystem)
  return ADMIN_MENU
    .filter(g => !hidden(g))
    .map(g => ({ ...g, items: g.items.filter(it => !hidden(it)) }))
    .filter(g => g.items.length)
}

// findAdminSection returns { group, item } for a group/item path pair, or null.
export function findAdminSection(menu, groupPath, itemPath) {
  const group = menu.find(g => g.path === groupPath)
  const item = group?.items.find(it => it.path === itemPath)
  return item ? { group, item } : null
}
