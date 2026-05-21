import { NextResponse } from "next/server";
import { authorizedFetch, backendFetch, backendJSON, bearer } from "../../../../lib/server-api";

export async function POST(request: Request) {
  const body = await request.json();
  const response = await authorizedFetch((token) =>
    backendFetch("/api/v1/payments/create", {
      method: "POST",
      headers: bearer(token),
      body: JSON.stringify(body),
    }),
  );
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
