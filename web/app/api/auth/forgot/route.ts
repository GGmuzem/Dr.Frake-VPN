import { NextResponse } from "next/server";
import { backendFetch, backendJSON } from "../../../../lib/server-api";

export async function POST(request: Request) {
  const body = await request.json();
  const response = await backendFetch("/api/v1/auth/forgot-password", {
    method: "POST",
    body: JSON.stringify(body),
  });
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
