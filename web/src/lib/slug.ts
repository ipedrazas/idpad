/**
 * The client's copy of the API's tag slug rule (api/internal/model/tag.go).
 * It exists so the editor can tell whether a typed name is a duplicate of a
 * chip already showing, without a round trip. The server remains authoritative:
 * what it returns is what gets rendered.
 */
export function slugify(name: string): string {
  return name
    .normalize('NFD')
    // Strip the combining marks NFD split off, so accents fold to the base letter.
    .replace(/\p{Mn}/gu, '')
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, '-')
    .replace(/^-+|-+$/g, '')
}
