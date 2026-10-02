import { describe, it, expect } from 'vitest'
import { isExternalUrl } from './externalLinks'

describe('isExternalUrl', () => {
  const origin = 'tauri://localhost'

  it('treats links to other hosts as external', () => {
    expect(isExternalUrl('https://git.example.com/org/repo/issues/42', origin)).toBe(true)
    expect(isExternalUrl('http://warmdesk.example.com/api/v1/attachments/1', origin)).toBe(true)
  })

  it('treats mailto and tel links as external', () => {
    expect(isExternalUrl('mailto:someone@example.com', origin)).toBe(true)
    expect(isExternalUrl('tel:+31201234567', origin)).toBe(true)
  })

  it('keeps in-app links inside the app', () => {
    expect(isExternalUrl('/projects/abc', 'https://tauri.localhost')).toBe(false)
    expect(isExternalUrl('https://tauri.localhost/time-tracking', 'https://tauri.localhost')).toBe(false)
    expect(isExternalUrl('#section', origin)).toBe(false)
  })

  it('ignores other schemes', () => {
    expect(isExternalUrl('javascript:void(0)', origin)).toBe(false)
    expect(isExternalUrl('blob:tauri://localhost/123', origin)).toBe(false)
  })
})
