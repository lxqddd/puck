import { buildCharacterPack } from '@puck/avatar-runtime'
import manifest from '../../../characters/puck/manifest.json'
import mapping from '../../../characters/puck/mapping.json'
import persona from '../../../characters/puck/persona.json'

export const characterManifest = manifest
export const characterPack = buildCharacterPack(manifest, mapping)
export { persona }
