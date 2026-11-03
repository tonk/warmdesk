import { describe, it, expect, beforeEach } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

// The fetch proxy lives inline in index.html because it must run before the
// bundle evaluates (Axios captures window.fetch at module load). Extract that
// script and run it against a fake tauri-plugin-http so we can test it.
const html = readFileSync(resolve(process.cwd(), 'index.html'), 'utf8')
const proxySource = html.match(/<script>([\s\S]*?warmDeskFetch[\s\S]*?)<\/script>/)[1]

function installProxy() {
  const calls = []
  window.fetch = async () => new Response('native')
  window.__tauriFetch = async (url, init) => {
    calls.push({ url, init })
    return new Response('tauri')
  }
  // eslint-disable-next-line no-new-func
  new Function(proxySource)()
  return calls
}

describe('index.html fetch proxy', () => {
  let calls
  beforeEach(() => { calls = installProxy() })

  it('passes a binary Request body to tauri-plugin-http byte-for-byte', async () => {
    // PNG signature + bytes that are invalid UTF-8 (0x89, 0xFF, 0xC4 ...)
    const bytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0xff, 0xc4, 0x00, 0x80])
    const req = new Request('http://example.test/api/v1/upload/image', {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream' },
      body: bytes,
    })
    await window.fetch(req)

    expect(calls).toHaveLength(1)
    const body = calls[0].init.body
    expect(typeof body).not.toBe('string')
    expect(Array.from(new Uint8Array(body))).toEqual(Array.from(bytes))
  })

  it('keeps routing text bodies and plain (url, init) calls through tauri', async () => {
    await window.fetch('http://example.test/x', { method: 'POST', body: 'hi' })
    expect(calls[0].init.body).toBe('hi')
  })

  it('leaves Tauri-internal ipc.localhost requests on the native fetch', async () => {
    const res = await window.fetch('http://ipc.localhost/cmd')
    expect(await res.text()).toBe('native')
    expect(calls).toHaveLength(0)
  })
})
