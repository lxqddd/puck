import { execFileSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'

function check(label, fn) {
  try {
    const value = fn()
    console.log(`ok   ${label}: ${value}`)
    return true
  } catch (error) {
    console.log(`FAIL ${label}: ${error.message}`)
    return false
  }
}

const results = [
  check('node', () => process.version),
  check('pnpm', () => execFileSync('pnpm', ['--version'], { encoding: 'utf8' }).trim()),
  check('go', () => execFileSync('go', ['version'], { encoding: 'utf8' }).trim()),
  check('wails3', () => {
    const bin = process.env.WAILS3_BIN || join(homedir(), 'go', 'bin', 'wails3')
    if (!existsSync(bin)) {
      throw new Error(
        'not found — install with: go install github.com/wailsapp/wails/v3/cmd/wails3@latest',
      )
    }
    return execFileSync(bin, ['version'], { encoding: 'utf8' }).trim()
  }),
]

if (results.some((ok) => !ok)) {
  process.exit(1)
}
