import { NextResponse } from "next/server";
import { authorizedFetch, backendFetch, backendJSON, bearer } from "../../../lib/server-api";

export async function GET() {
  const response = await authorizedFetch((token) =>
    backendFetch("/api/v1/me/routing-profiles", {
      method: "GET",
      headers: bearer(token),
    }),
  );
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}

export async function POST(request: Request) {
  const body = await request.json();
  const response = await authorizedFetch((token) =>
    backendFetch("/api/v1/me/routing-profiles", {
      method: "POST",
      headers: { ...bearer(token), "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  );
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
