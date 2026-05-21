import { NextResponse } from "next/server";
import { backendFetch, backendJSON, BackendTokens, setAuthCookies } from "../../../../lib/server-api";

export async function POST(request: Request) {
  const body = await request.json();
  const response = await backendFetch("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(body),
  });
  const data = await backendJSON<BackendTokens>(response);
  if (!response.ok || !("access_token" in data)) {
    return NextResponse.json(data, { status: response.status });
  }
  await setAuthCookies(data);
  return NextResponse.json({ ok: true });
}
