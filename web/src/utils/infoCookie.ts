// 10 years, matches the previous hand-rolled Max-Age for the dismissed-info
// cookie. Long enough to outlast any realistic "I already read this" interval
// without ever becoming a runaway store entry.
const maxAgeSeconds = 60 * 60 * 24 * 365 * 10

export interface CookieJar {
  read: (cookie?: string) => string
  write: (value: string) => void
}

function makeCookie(name: string): CookieJar {
  return {
    read: (cookie: string = document.cookie): string => {
      const prefix = `${name}=`
      for (const part of cookie.split(';')) {
        const trimmed = part.trim()
        if (trimmed.startsWith(prefix)) {
          return decodeURIComponent(trimmed.slice(prefix.length))
        }
      }

      return ''
    },
    write: (value: string): void => {
      document.cookie = `${name}=${encodeURIComponent(value)}; Max-Age=${maxAgeSeconds}; Path=/; SameSite=Lax`
    }
  }
}

export const infoDismissedCookie = makeCookie('meshmap-lite.info.dismissed_source_hash')
export const updatesDismissedCookie = makeCookie('meshmap-lite.updates.dismissed_published_at')
