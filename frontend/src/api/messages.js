import client from './client'

export const messagesApi = {
  listConversations: () => client.get('/direct-messages/conversations'),
  listUsers: () => client.get('/users'),
  listMessages: (userId) => client.get(`/direct-messages/${userId}`),
  sendMessage: (userId, data) => client.post(`/direct-messages/${userId}`, data),
  deleteMessage: (userId, msgId) => client.delete(`/direct-messages/${userId}/${msgId}`)
}
