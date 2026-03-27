/**
 * Returns the best available avatar URL for a user object.
 * Priority: explicit avatar_url → Gravatar URL supplied by backend → null
 */
export function avatarUrl(user) {
  if (!user) return null
  return user.avatar_url || user.gravatar_url || null
}
