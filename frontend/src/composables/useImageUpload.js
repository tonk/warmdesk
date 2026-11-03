import { attachmentsApi } from '@/api/attachments'
import { useUIStore } from '@/stores/ui'

/**
 * Shared handler for "choose an image" <input type="file"> elements
 * (avatars, customer/company logos). Uploads the first selected file and hands
 * the resulting server URL to `onUrl`; failures are shown as a toast.
 *
 * The input is reset only after the upload finishes, so the File stays valid
 * for the whole request and re-selecting the same file still fires `change`.
 */
export function useImageUpload() {
  const ui = useUIStore()

  async function uploadFromInput(e, onUrl, errorMessage = 'Failed to upload image') {
    const input = e.target
    const file = input.files?.[0]
    if (!file) return
    try {
      const { data } = await attachmentsApi.uploadImage(file)
      onUrl(data.url)
    } catch (err) {
      ui.error(err.response?.data?.error || errorMessage)
    } finally {
      input.value = ''
    }
  }

  return { uploadFromInput }
}
