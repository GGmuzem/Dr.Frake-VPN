import { NextResponse } from "next/server";
import { authorizedFetch, backendFetch, backendJSON, bearer } from "@/lib/server-api";

export async function POST(_request: Request, context: { params: Promise<{ code: string }> }) {
  const params = await context.params;
  const response = await authorizedFetch((token) =>
    backendFetch(`/api/v1/me/routing-profiles/system/${encodeURIComponent(params.code)}/copy`, {
      method: "POST",
      headers: bearer(token),
      body: JSON.stringify({}),
    }),
  );
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
