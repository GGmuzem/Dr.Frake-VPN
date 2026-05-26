import { NextResponse } from "next/server";
import { authorizedFetch, backendFetch, backendJSON, bearer } from "../../../../lib/server-api";

export async function PUT(request: Request, context: { params: Promise<{ id: string }> }) {
  const params = await context.params;
  const body = await request.json();
  const response = await authorizedFetch((token) =>
    backendFetch(`/api/v1/me/routing-profiles/${params.id}`, {
      method: "PUT",
      headers: { ...bearer(token), "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  );
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}

export async function DELETE(request: Request, context: { params: Promise<{ id: string }> }) {
  const params = await context.params;
  const response = await authorizedFetch((token) =>
    backendFetch(`/api/v1/me/routing-profiles/${params.id}`, {
      method: "DELETE",
      headers: bearer(token),
    }),
  );
  const data = await backendJSON(response);
  return NextResponse.json(data, { status: response.status });
}
