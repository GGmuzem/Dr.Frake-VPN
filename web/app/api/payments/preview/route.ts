import { NextResponse } from "next/server";
import { bearer, backendFetch, backendJSON, currentAccessToken } from "../../../../lib/server-api";

export async function POST(request: Request) {
  const token = await currentAccessToken();
  if (!token) {
    return NextResponse.json({ error: "Требуется вход" }, { status: 401 });
  }

  const body = await request.json();
  const response = await backendFetch("/api/v1/payments/preview", {
    method: "POST",
    headers: bearer(token),
    body: JSON.stringify(body),
  });
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
