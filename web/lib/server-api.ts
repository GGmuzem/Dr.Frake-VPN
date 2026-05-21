import { cookies } from "next/headers";

export const accessCookieName = "fblink_access_token";
export const refreshCookieName = "fblink_refresh_token";

export type BackendTokens = {
  access_token: string;
  refresh_token: string;
};

export type BackendError = {
  error?: string;
  message?: string;
};

export function backendBaseURL(): string {
  return (process.env.BACKEND_API_URL ?? "http://backend:8081").replace(/\/$/, "");
}

export async function backendFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  return fetch(`${backendBaseURL()}${path}`, {
    ...init,
    headers,
    cache: "no-store",
  });
}

export async function currentAccessToken(): Promise<string | undefined> {
  const store = await cookies();
  return store.get(accessCookieName)?.value;
}

export async function currentRefreshToken(): Promise<string | undefined> {
  const store = await cookies();
  return store.get(refreshCookieName)?.value;
}

export async function setAuthCookies(tokens: BackendTokens): Promise<void> {
  const store = await cookies();
  const cookieOptions = {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax" as const,
    path: "/",
  };
  // Access token живёт 1 час на бэкенде, но в куки кладём 30 дней:
  // фронт-обёртка `authorizedFetch` сама вызовет /api/v1/auth/refresh при 401
  // и обновит cookie, поэтому ранний `Max-Age` только бы выкидывал юзера.
  store.set(accessCookieName, tokens.access_token, { ...cookieOptions, maxAge: 60 * 60 * 24 * 30 });
  store.set(refreshCookieName, tokens.refresh_token, { ...cookieOptions, maxAge: 60 * 60 * 24 * 30 });
}

export async function clearAuthCookies(): Promise<void> {
  const store = await cookies();
  store.delete(accessCookieName);
  store.delete(refreshCookieName);
}

export async function backendJSON<T>(response: Response): Promise<T | BackendError> {
  try {
    return (await response.json()) as T;
  } catch {
    return { error: "Неверный ответ сервера" };
  }
}

export function bearer(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` };
}

async function refreshAccessToken(): Promise<string | null> {
  const refresh = await currentRefreshToken();
  if (!refresh) return null;

  const response = await backendFetch("/api/v1/auth/refresh", {
    method: "POST",
    body: JSON.stringify({ refresh_token: refresh }),
  });
  if (!response.ok) {
    await clearAuthCookies();
    return null;
  }

  try {
    const tokens = (await response.json()) as BackendTokens;
    if (!tokens?.access_token || !tokens?.refresh_token) {
      await clearAuthCookies();
      return null;
    }
    await setAuthCookies(tokens);
    return tokens.access_token;
  } catch {
    await clearAuthCookies();
    return null;
  }
}

/**
 * Делает запрос к backend от лица текущего пользователя.
 * При 401 один раз пытается обновить access-токен через refresh и повторить запрос.
 * Если refresh-токен невалиден — куки чистятся, и наверх отдаётся последний 401-ответ.
 *
 * Принимает callback `build(token)`, который строит конкретный fetch — это нужно,
 * чтобы после refresh можно было пересоздать запрос с новым Authorization-хедером.
 */
export async function authorizedFetch(
  build: (token: string) => Promise<Response>,
): Promise<Response> {
  const access = await currentAccessToken();
  if (!access) {
    const refreshed = await refreshAccessToken();
    if (!refreshed) {
      return new Response(JSON.stringify({ error: "unauthorized" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      });
    }
    return build(refreshed);
  }

  const response = await build(access);
  if (response.status !== 401) return response;

  const refreshed = await refreshAccessToken();
  if (!refreshed) return response;

  return build(refreshed);
}
