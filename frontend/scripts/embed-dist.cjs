#!/usr/bin/env node
/* eslint-disable no-console */

const fs = require('node:fs')
const path = require('node:path')

const frontendRoot = path.resolve(__dirname, '..')
const sourceDir = path.join(frontendRoot, 'dist')
const targetDir = path.resolve(frontendRoot, '..', 'backend', 'internal', 'frontend', 'dist')

function removeDirContents(dirPath) {
  if (!fs.existsSync(dirPath)) {
    return
  }

  for (const entry of fs.readdirSync(dirPath)) {
    fs.rmSync(path.join(dirPath, entry), { recursive: true, force: true })
  }
}

function copyDirContents(source, target) {
  const entries = fs.readdirSync(source, { withFileTypes: true })

  for (const entry of entries) {
    const srcPath = path.join(source, entry.name)
    const dstPath = path.join(target, entry.name)

    if (entry.isDirectory()) {
      fs.mkdirSync(dstPath, { recursive: true })
      copyDirContents(srcPath, dstPath)
      continue
    }

    fs.copyFileSync(srcPath, dstPath)
  }
}

if (!fs.existsSync(sourceDir)) {
  console.error(`[embed-dist] 未找到前端构建产物: ${sourceDir}`)
  console.error('[embed-dist] 请先执行: pnpm run build-only')
  process.exit(1)
}

fs.mkdirSync(targetDir, { recursive: true })
removeDirContents(targetDir)
copyDirContents(sourceDir, targetDir)

console.log(`[embed-dist] 已嵌入前端产物 -> ${targetDir}`)
