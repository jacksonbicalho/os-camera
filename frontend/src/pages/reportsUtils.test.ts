import { describe, expect, it } from 'vitest'
import { categoryBuckets } from './reportsUtils'

describe('categoryBuckets', () => {
  it('dobra labels nas categorias (vazio→movimento, pessoa, outro→ia)', () => {
    expect(categoryBuckets({ '': 5, pessoa: 3, carro: 2 })).toEqual({
      movimento: 5, pessoa: 3, ia: 2, estados: 0,
    })
  })
  it('soma múltiplos labels da mesma categoria', () => {
    expect(categoryBuckets({ pessoa: 2, 'Pessoa com chapéu': 1 }).pessoa).toBe(3)
  })
})
