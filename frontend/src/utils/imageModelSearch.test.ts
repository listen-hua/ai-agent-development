import { describe, expect, it } from 'vitest'
import { filterImageModels } from './imageModelSearch'
import type { ImageModel, ImageRelay } from '@/types/image-agent'

const relays = [
  { id: 'relay-comfly', relay_key: 'comfly', name: 'Comfly AI' },
  { id: 'relay-xgapi', relay_key: 'xgapi', name: 'XGAPI' },
] as ImageRelay[]

const models = [
  { id: '1', relay_id: 'relay-comfly', model_id: 'gemini-2.5-flash-image', display_name: 'Gemini Flash Image' },
  { id: '2', relay_id: 'relay-comfly', model_id: 'gpt-image-1', display_name: 'GPT Image' },
  { id: '3', relay_id: 'relay-xgapi', model_id: 'flux-kontext-pro', display_name: 'FLUX Kontext Pro' },
] as ImageModel[]

describe('filterImageModels', () => {
  it('supports case-insensitive partial matching on display name and model id', () => {
    expect(filterImageModels(models, relays, '', 'FLASH').map((item) => item.id)).toEqual(['1'])
    expect(filterImageModels(models, relays, '', 'kontext').map((item) => item.id)).toEqual(['3'])
  })

  it('matches multiple fuzzy terms regardless of their field', () => {
    expect(filterImageModels(models, relays, '', 'comfly gemini').map((item) => item.id)).toEqual(['1'])
  })

  it('combines relay filtering with fuzzy search', () => {
    expect(filterImageModels(models, relays, 'relay-comfly', 'image').map((item) => item.id)).toEqual(['1', '2'])
    expect(filterImageModels(models, relays, 'relay-xgapi', 'image')).toEqual([])
  })
})
