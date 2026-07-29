import type { ImageModel, ImageRelay } from '@/types/image-agent'

export function filterImageModels(
  models: ImageModel[],
  relays: ImageRelay[],
  relayId: string,
  query: string,
) {
  const relayById = new Map(relays.map((relay) => [relay.id, relay]))
  const terms = normalizeSearchText(query).split(' ').filter(Boolean)

  return models.filter((model) => {
    if (relayId && model.relay_id !== relayId) return false
    if (!terms.length) return true

    const relay = relayById.get(model.relay_id)
    const haystack = normalizeSearchText([
      model.display_name,
      model.model_id,
      relay?.name,
      relay?.relay_key,
    ].filter(Boolean).join(' '))

    return terms.every((term) => haystack.includes(term))
  })
}

function normalizeSearchText(value: string) {
  return value.trim().toLocaleLowerCase('zh-CN').replace(/\s+/g, ' ')
}
