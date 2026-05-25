import { NextResponse } from "next/server";
import { authorizedFetch, backendFetch, backendJSON, bearer } from "../../../../lib/server-api";

export async function PATCH(request: Request) {
  const body = await request.json();
  const response = await authorizedFetch((token) =>
    backendFetch("/api/v1/me/subscription/ad-block", {
      method: "PATCH",
      headers: { ...bearer(token), "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  );
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
