import { Events } from '@wailsio/runtime'
import { EVENTS } from '@puck/contract'
import type {
  ChatDeltaEvent,
  ChatDoneEvent,
  ChatErrorEvent,
  ChatStateEvent,
  Conversation,
  MascotScaleEvent,
  ProviderConfig,
  ProviderTestResult,
  SendResult,
  Settings,
  StoredMessage,
} from '@puck/contract'
import { ChatService, MascotService, SettingsService } from './bindings/puck/internal/services'
import type { Position } from './bindings/puck/internal/services'

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
  sendChat: (text: string, conversationId: string, providerId: string): Promise<SendResult> =>
    ChatService.Send(text, conversationId, providerId),
  listConversations: async (): Promise<Conversation[]> => (await ChatService.ListConversations()) ?? [],
  getMessages: async (conversationId: string): Promise<StoredMessage[]> =>
    ((await ChatService.GetMessages(conversationId)) ?? []) as StoredMessage[],
  deleteConversation: (conversationId: string) => ChatService.DeleteConversation(conversationId),

  getSettings: async (): Promise<Settings> => {
    const settings = await SettingsService.Get()
    const providers = (settings.providers ?? [])
      .filter((provider) => provider !== null)
      .map((provider) => ({ ...provider })) as ProviderConfig[]
    return {
      providers,
      activeProviderId: settings.activeProviderId,
      mascotScale: settings.mascotScale,
    }
  },
  saveProvider: (provider: ProviderConfig, apiKey: string): Promise<ProviderConfig> =>
    SettingsService.SaveProvider(provider, apiKey),
  testProvider: (provider: ProviderConfig, apiKey: string): Promise<ProviderTestResult> =>
    SettingsService.TestProvider(provider, apiKey),
  listModels: async (provider: ProviderConfig, apiKey: string): Promise<string[]> =>
    (await SettingsService.ListModels(provider, apiKey)) ?? [],
  deleteProvider: (id: string) => SettingsService.DeleteProvider(id),
  setActiveProvider: (id: string) => SettingsService.SetActiveProvider(id),

  onChatDelta: (callback: (event: ChatDeltaEvent) => void) => subscribe(EVENTS.chatDelta, callback),
  onChatDone: (callback: (event: ChatDoneEvent) => void) => subscribe(EVENTS.chatDone, callback),
  onChatError: (callback: (event: ChatErrorEvent) => void) => subscribe(EVENTS.chatError, callback),
  onChatState: (callback: (event: ChatStateEvent) => void) => subscribe(EVENTS.chatState, callback),
  onMascotScale: (callback: (event: MascotScaleEvent) => void) =>
    subscribe(EVENTS.mascotScale, callback),
}
