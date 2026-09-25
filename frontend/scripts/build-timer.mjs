// Builds the warmdesk-timer CLI (backend/cmd/timer) for the platform Tauri is
// bundling, so the Linux .deb and .rpm can ship it as /usr/bin/warmdesk-timer
// (see bundle.linux.{deb,rpm}.files in src-tauri/tauri.conf.json).
//
// Runs as part of beforeBuildCommand. Tauri passes the bundle target in
// TAURI_ENV_PLATFORM / TAURI_ENV_ARCH; without them (run by hand) the host is
// used. On macOS and Windows nothing is built: those installers don't
// include the timer, users download it from the release page instead.
import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const tauriDir = join(here, '..', 'src-tauri')
const backendDir = join(here, '..', '..', 'backend')

const platform = process.env.TAURI_ENV_PLATFORM || process.platform
if (platform !== 'linux') {
  console.log(`build-timer: skipped for ${platform} (only the Linux packages include warmdesk-timer)`)
  process.exit(0)
}

const goArch = {
  x86_64: 'amd64', x64: 'amd64', amd64: 'amd64',
  aarch64: 'arm64', arm64: 'arm64',
}[process.env.TAURI_ENV_ARCH || process.arch]
if (!goArch) {
  console.error(`build-timer: unsupported architecture ${process.env.TAURI_ENV_ARCH || process.arch}`)
  process.exit(1)
}

const { version } = JSON.parse(readFileSync(join(tauriDir, 'tauri.conf.json'), 'utf8'))
const out = join(tauriDir, 'bin', 'warmdesk-timer')
mkdirSync(dirname(out), { recursive: true })

try {
  execFileSync('go', ['build', '-ldflags', `-s -w -X main.version=v${version}`, '-o', out, './cmd/timer'], {
    cwd: backendDir,
    stdio: 'inherit',
    env: { ...process.env, CGO_ENABLED: '0', GOOS: 'linux', GOARCH: goArch },
  })
} catch (err) {
  console.error('build-timer: building warmdesk-timer failed' +
    (err.code === 'ENOENT' ? ' — Go is not installed (https://go.dev/dl)' : ''))
  process.exit(1)
}
console.log(`build-timer: built ${out} (linux/${goArch}, v${version})`)
