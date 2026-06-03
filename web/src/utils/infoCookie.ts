export const infoDismissedCookieName = 'meshmap-lite.info.dismissed_source_hash'

const maxAgeSeconds = 60 * 60 * 24 * 365 * 10

export function readInfoDismissedSourceHash(cookie = document.cookie): string {
  const prefix = `${infoDismissedCookieName}=`
  for (const part of cookie.split(';')) {
    const trimmed = part.trim()
    if (trimmed.startsWith(prefix)) {
      return decodeURIComponent(trimmed.slice(prefix.length))
    }
  }

  return ''
}

export function writeInfoDismissedSourceHash(sourceHash: string): void {
  document.cookie = `${infoDismissedCookieName}=${encodeURIComponent(sourceHash)}; Max-Age=${maxAgeSeconds}; Path=/; SameSite=Lax`
}
