import { authorizedFetch, backendFetch } from "@/lib/server-api";

type RouteContext = {
  params: Promise<{ path: string[] }>;
};

export const dynamic = "force-dynamic";

async function proxyAdmin(request: Request, context: RouteContext) {
  try {
    const { path } = await context.params;
    const sourceURL = new URL(request.url);
    const backendPath = `/api/v1/admin/${path.join("/")}${sourceURL.search}`;
    const body =
      request.method === "GET" || request.method === "HEAD"
        ? undefined
        : Buffer.from(await request.arrayBuffer());
    const response = await authorizedFetch((token) => {
      const headers = new Headers();
      headers.set("Authorization", `Bearer ${token}`);
      const contentType = request.headers.get("Content-Type");
      if (contentType) headers.set("Content-Type", contentType);
      return backendFetch(backendPath, {
        method: request.method,
        headers,
        body,
      });
    });

    const headers = new Headers();
    const contentType = response.headers.get("Content-Type");
    const contentDisposition = response.headers.get("Content-Disposition");
    if (contentType) headers.set("Content-Type", contentType);
    if (contentDisposition) headers.set("Content-Disposition", contentDisposition);
    return new Response(response.body, { status: response.status, headers });
  } catch (error) {
    console.error("[admin-api-proxy] backend proxy failed", error);
    return Response.json({ error: "admin_backend_unavailable" }, { status: 502 });
  }
}

export async function GET(request: Request, context: RouteContext) {
  return proxyAdmin(request, context);
}

export async function POST(request: Request, context: RouteContext) {
  return proxyAdmin(request, context);
}

export async function PUT(request: Request, context: RouteContext) {
  return proxyAdmin(request, context);
}

export async function PATCH(request: Request, context: RouteContext) {
  return proxyAdmin(request, context);
}

export async function DELETE(request: Request, context: RouteContext) {
  return proxyAdmin(request, context);
}
