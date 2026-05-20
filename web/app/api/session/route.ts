import { NextResponse } from "next/server";
import { bearer, backendFetch, backendJSON, currentAccessToken } from "../../../lib/server-api";

export async function GET() {
  const token = await currentAccessToken();
  if (!token) {
    return NextResponse.json({ authenticated: false }, { status: 401 });
  }

  const [meResponse, subscriptionResponse] = await Promise.all([
    backendFetch("/api/v1/me", { headers: bearer(token) }),
    backendFetch("/api/v1/me/subscription", { headers: bearer(token) }),
  ]);

  if (!meResponse.ok) {
    const data = await backendJSON(meResponse);
    return NextResponse.json(data, { status: meResponse.status });
  }
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
