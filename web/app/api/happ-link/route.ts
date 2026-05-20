import { NextResponse } from "next/server";
import { bearer, backendFetch, backendJSON, currentAccessToken } from "../../../lib/server-api";

export async function POST() {
  const token = await currentAccessToken();
  if (!token) {
    return NextResponse.json({ error: "Требуется вход" }, { status: 401 });
  }

  const response = await backendFetch("/api/v1/me/happ-link", {
    method: "POST",
    headers: bearer(token),
  });
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
