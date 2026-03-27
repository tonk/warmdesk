import { defineStore } from 'pinia'
import { ref } from 'vue'
import { projectsApi } from '@/api/projects'
import client from '@/api/client'

export const useSidebarStore = defineStore('sidebar', () => {
  const starredProjects = ref([])
  const allProjects = ref([])
  const allUsers = ref([])
  const chatUsers = ref([])

  async function fetchStarred() {
    try {
      const { data } = await projectsApi.listStarred()
      starredProjects.value = data || []
    } catch {}
  }

  async function fetchAllProjects() {
    try {
      const { data } = await projectsApi.list()
      allProjects.value = data || []
    } catch {}
  }

  async function fetchAllUsers() {
    try {
      const { data } = await client.get('/users')
      allUsers.value = data || []
    } catch {}
  }

  async function fetchChatUsers() {
    try {
      const { data } = await client.get('/online-users')
      chatUsers.value = data || []
    } catch {}
  }

  async function starProject(slug) {
    await projectsApi.starProject(slug)
    await fetchStarred()
  }

  async function unstarProject(slug) {
    await projectsApi.unstarProject(slug)
    starredProjects.value = starredProjects.value.filter(p => p.slug !== slug)
  }

  function isStarred(slug) {
    return starredProjects.value.some(p => p.slug === slug)
  }

  return {
    starredProjects, allProjects, allUsers, chatUsers,
    fetchStarred, fetchAllProjects, fetchAllUsers, fetchChatUsers,
    starProject, unstarProject, isStarred
  }
})
