/** Query keys, kept in one place so invalidation never drifts from the fetch. */
export const queryKeys = {
  ideas: ['ideas'] as const,
  idea: (id: string) => ['ideas', id] as const,
  comments: (ideaId: string) => ['ideas', ideaId, 'comments'] as const,
  resources: (ideaId: string) => ['ideas', ideaId, 'resources'] as const,
}
