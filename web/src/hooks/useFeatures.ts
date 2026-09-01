import { useQuery } from '@tanstack/react-query'

import { api } from '../api/client'
import type { Features } from '../api/types'
import { queryKeys } from './queryKeys'

/**
 * Which optional capabilities the server is configured for. It cannot change
 * without a restart, so this is cached for the session rather than refetched.
 */
export function useFeatures() {
  return useQuery<Features>({
    queryKey: queryKeys.features,
    queryFn: api.listFeatures,
    staleTime: Infinity,
  })
}
