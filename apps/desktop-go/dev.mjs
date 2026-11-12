import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const wails = process.env.WAILS3_BIN || join(homedir(), 'go', 'bin', 'wails3')

if (!existsSync(wails)) {
  console.error(
    '未找到 wails3。请先安装：go install github.com/wailsapp/wails/v3/cmd/wails3@latest',
  )
  process.exit(1)
}

const child = spawn(wails, ['dev', '-config', './build/config.yml', '-port', '34115'], {
  cwd: here,
  stdio: 'inherit',
  env: {
    ...process.env,
    PATH: `${dirname(wails)}:${process.env.PATH ?? ''}`,
    GOPROXY: process.env.GOPROXY || 'https://goproxy.cn,direct',
  },
})

child.on('exit', (code) => process.exit(code ?? 0))
for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => child.kill(signal))
}
