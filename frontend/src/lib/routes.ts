export function appPath(path: string) {
  const base = process.env.NEXT_PUBLIC_BASE_PATH || ''
  if (!path) return base || '/'
  if (path.startsWith('/')) return `${base}${path}`
  return `${base}/${path}`
}
