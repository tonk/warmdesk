import client, { fetchBinary } from './client'
import { useContractUsageStore } from '@/stores/contractUsage'

// Every save reports the weekly usage of the entry's contract; route it to
// the store, which keeps the grid counters current and warns when the
// contract's hours per week are exceeded.
function trackUsage(res) {
  useContractUsageStore().onEntrySaved(res?.data)
  return res
}

export const timeEntriesApi = {
  list: (params) => client.get('/time-entries', { params }),
  create: (data) => client.post('/time-entries', data).then(trackUsage),
  update: (id, data) => client.put(`/time-entries/${id}`, data).then(trackUsage),
  remove: (id) => client.delete(`/time-entries/${id}`).then(res => {
    useContractUsageStore().refresh()
    return res
  }),
  report: (params) => client.get('/time-entries/report', { params }),
  reportPDF: (params) => fetchBinary('/time-entries/report/pdf', params),
  reportChartPDF: (params) => fetchBinary('/time-entries/report/chart-pdf', params),
  reportXLSX: (params) => fetchBinary('/time-entries/report/xlsx', params),
  sheetXLSX: (params) => fetchBinary('/time-entries/sheet/xlsx', params),
  gridPDF: (params) => fetchBinary('/time-entries/grid/pdf', params),
  addHolidays: (data) => client.post('/time-entries/holidays', data),
  getRowOrder: (params) => client.get('/time-entries/row-order', { params }),
  setRowOrder: (keys, comments, params) => client.put('/time-entries/row-order', { keys, comments }, { params }),
  getMacroLibrary: () => client.get('/time-entries/macro-library'),
  setMacroLibrary: (library) => client.put('/time-entries/macro-library', { library }),
}
