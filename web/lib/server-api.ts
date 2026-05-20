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

export async function setAuthCookies(tokens: BackendTokens): Promise<void> {
  const store = await cookies();
  const cookieOptions = {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax" as const,
    path: "/",
  };
  store.set(accessCookieName, tokens.access_token, { ...cookieOptions, maxAge: 60 * 60 * 24 });
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
