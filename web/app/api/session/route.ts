import { NextResponse } from "next/server";
import { authorizedFetch, backendFetch, backendJSON, bearer } from "../../../lib/server-api";

export async function GET() {
  const meResponse = await authorizedFetch((token) =>
    backendFetch("/api/v1/me", { headers: bearer(token) }),
  );
  if (meResponse.status === 401) {
    return NextResponse.json({ authenticated: false }, { status: 401 });
  }
  if (!meResponse.ok) {
    const data = await backendJSON(meResponse);
    return NextResponse.json(data, { status: meResponse.status });
  }

  const subscriptionResponse = await authorizedFetch((token) =>
    backendFetch("/api/v1/me/subscription", { headers: bearer(token) }),
  );
  if (!subscriptionResponse.ok) {
    const data = await backendJSON(subscriptionResponse);
    return NextResponse.json(data, { status: subscriptionResponse.status });
  }

  return NextResponse.json({
    authenticated: true,
    user: await meResponse.json(),
    subscription: await subscriptionResponse.json(),
  });
}
