import { NextResponse } from "next/server";
import { authorizedFetch, backendFetch, backendJSON, bearer } from "../../../../lib/server-api";

export async function PATCH(req: Request) {
  const body = await req.json();

  const response = await authorizedFetch((token) =>
    backendFetch("/api/v1/me/subscription/auto-renew", {
      method: "PATCH",
      headers: bearer(token),
      body: JSON.stringify(body),
    }),
  );

  if (!response.ok) {
    const data = await backendJSON(response);
    return NextResponse.json(data, { status: response.status });
  }

  const data = await response.json();
  return NextResponse.json(data);
}
