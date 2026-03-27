import client from './client'

export const systemApi = {
  getSettings: () => client.get('/system/settings')
}
