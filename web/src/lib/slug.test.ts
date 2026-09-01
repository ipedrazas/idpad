import { describe, expect, it } from 'vitest'

import { slugify } from './slug'

describe('slugify', () => {
  // These cases are the same ones api/internal/model/tag_test.go asserts. The
  // two implementations have to agree, or the editor would call a tag new that
  // the server folds onto an existing one.
  const cases: [string, string][] = [
    ['Machine Learning', 'machine-learning'],
    ['machine-learning', 'machine-learning'],
    ['machine   ---  learning', 'machine-learning'],
    ['  --machine learning--  ', 'machine-learning'],
    ['Máchine Léarning', 'machine-learning'],
    ['web 3.0', 'web-3-0'],
    ['c++/rust!', 'c-rust'],
    ['Идея', 'идея'],
    ['  --  ', ''],
    ['', ''],
  ]

  it.each(cases)('slugifies %j to %j', (input, expected) => {
    expect(slugify(input)).toBe(expected)
  })

  it('folds variant spellings onto one slug', () => {
    for (const variant of ['Machine Learning', 'machine learning', 'MACHINE-LEARNING', ' machine  learning ']) {
      expect(slugify(variant)).toBe('machine-learning')
    }
  })
})
