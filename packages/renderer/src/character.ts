import { buildCharacterPack } from '@puck/avatar-runtime'
import type { Manifest, Mapping } from '@puck/avatar-runtime'
import manifestJson from '../../../characters/puck/manifest.json'
import mappingJson from '../../../characters/puck/mapping.json'
import persona from '../../../characters/puck/persona.json'

const manifest = manifestJson as Manifest
const mapping = mappingJson as Mapping

// 角色包里的 .riv 文件会被 Vite 打包成带 hash 的资源；没有文件时返回空对象。
const riveAssets = import.meta.glob('../../../characters/puck/assets/rive/*.riv', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>

const riveEntries = Object.entries(riveAssets)
const rivePath = manifest.assets?.rive
const riveEntry =
  (rivePath ? riveEntries.find(([path]) => path.endsWith(rivePath)) : undefined) ?? riveEntries[0]

export const characterManifest = manifest
export const characterPack = buildCharacterPack(manifest, mapping, {
  riveAssetUrl: riveEntry?.[1],
})
export { persona }
