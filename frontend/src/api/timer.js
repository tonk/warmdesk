import client from './client'

// Time-tracking timer: one running timer per user, booked as time entries on stop.
export const timerApi = {
  get: () => client.get('/timer'),
  targets: () => client.get('/timer/targets'),
  start: (data) => client.post('/timer/start', data),
  stop: (data) => client.post('/timer/stop', data),
  cancel: () => client.delete('/timer'),
}
