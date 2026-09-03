import type { User } from '@/types'

export function hasRoles(user: User | null, required: string[] | undefined) {
  if (!required || required.length === 0) return true
  if (!user) return false
  const userRoles = user.roles || []
  return required.every((r) => userRoles.includes(r))
}

export function isAdmin(user: User | null) {
  return hasRoles(user, ['admin'])
}
