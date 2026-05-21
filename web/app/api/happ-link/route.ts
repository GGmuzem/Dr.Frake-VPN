import { NextResponse } from "next/server";
import { authorizedFetch, backendFetch, backendJSON, bearer } from "../../../lib/server-api";

export async function POST() {
  const response = await authorizedFetch((token) =>
    backendFetch("/api/v1/me/happ-link", {
      method: "POST",
      headers: bearer(token),
    }),
  );
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
