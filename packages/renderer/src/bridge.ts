import { Events } from '@wailsio/runtime'
import { EVENTS } from '@puck/contract'
import type {
  ChatDeltaEvent,
  ChatDoneEvent,
  ChatErrorEvent,
  ChatStateEvent,
  MascotScaleEvent,
} from '@puck/contract'
import { ChatService, MascotService, SettingsService } from './bindings/puck/internal/services'
import type { Position } from './bindings/puck/internal/services'
import type { Settings } from './bindings/puck/internal/store/models'

function subscribe<T>(name: string, callback: (data: T) => void): () => void {
  return Events.On(name, (event) => callback(event.data as T))
}

export const bridge = {
  openChat: () => MascotService.OpenChat(),
  openSettings: () => MascotService.OpenSettings(),
  setHitRegion: (
    region: { x: number; y: number; width: number; height: number },
    scale: number,
  ) => MascotService.SetHitRegion(region.x, region.y, region.width, region.height, scale),
  setClickThrough: (enabled: boolean) => MascotService.SetClickThrough(enabled),
  setDragging: (dragging: boolean) => MascotService.SetDragging(dragging),
  getPosition: (): Promise<Position> => MascotService.GetPosition(),
  setPosition: (x: number, y: number) => MascotService.SetPosition(x, y),
  getScale: (): Promise<number> => MascotService.GetScale(),
  scaleBy: (delta: number): Promise<number> => MascotService.ScaleBy(delta),

  setPersona: (prompt: string) => ChatService.SetPersona(prompt),
  sendChat: (text: string): Promise<string> => ChatService.Send(text),

  getSettings: (): Promise<Settings> => SettingsService.Get(),
  saveSettings: (settings: Settings, apiKey: string) => SettingsService.Save(settings, apiKey),

  onChatDelta: (callback: (event: ChatDeltaEvent) => void) => subscribe(EVENTS.chatDelta, callback),
  onChatDone: (callback: (event: ChatDoneEvent) => void) => subscribe(EVENTS.chatDone, callback),
  onChatError: (callback: (event: ChatErrorEvent) => void) => subscribe(EVENTS.chatError, callback),
  onChatState: (callback: (event: ChatStateEvent) => void) => subscribe(EVENTS.chatState, callback),
  onMascotScale: (callback: (event: MascotScaleEvent) => void) =>
    subscribe(EVENTS.mascotScale, callback),
}
