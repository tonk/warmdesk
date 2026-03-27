import client from './client'

export const authApi = {
  register: (data) => client.post('/auth/register', data),
  login: (data) => client.post('/auth/login', data),
  refresh: (token) => client.post('/auth/refresh', { refresh_token: token }),
  me: () => client.get('/auth/me'),
  updateMe: (data) => client.put('/auth/me', data),
  changePassword: (data) => client.put('/auth/me/password', data),
  listApiKeys: () => client.get('/auth/api-keys'),
  createApiKey: (name) => client.post('/auth/api-keys', { name }),
  deleteApiKey: (id) => client.delete(`/auth/api-keys/${id}`)
}
