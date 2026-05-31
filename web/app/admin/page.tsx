import { redirect } from "next/navigation";
import { AdminPanel } from "@/components/admin/AdminPanel";
import type { AdminOverview } from "@/lib/admin";
import { authorizedFetch, backendFetch, bearer } from "@/lib/server-api";

type MeResponse = {
  id: number;
  email: string;
  role: "user" | "admin";
};

export const dynamic = "force-dynamic";

const emptyOverview: AdminOverview = {
  kpis: {
    total_users: 0,
    active_subscriptions: 0,
    active_vpn_keys: 0,
    total_servers: 0,
    active_servers: 0,
    open_incidents: 0,
    monthly_revenue: 0,
  },
  notifications: [],
  servers: [],
};

export default async function AdminPage() {
  let meResponse: Response;
  try {
    meResponse = await authorizedFetch((token) =>
      backendFetch("/api/v1/me", { headers: bearer(token) }),
    );
  } catch (error) {
    console.error("[admin-page] failed to load current admin session", error);
    redirect("/auth");
  }

  if (meResponse.status === 401) redirect("/auth");
  if (!meResponse.ok) redirect("/dashboard");

  let me: MeResponse;
  try {
    me = (await meResponse.json()) as MeResponse;
  } catch (error) {
    console.error("[admin-page] invalid /api/v1/me response", error);
    redirect("/auth");
  }
  if (me.role !== "admin") redirect("/dashboard");

  let overview = emptyOverview;
  try {
    const overviewResponse = await authorizedFetch((token) =>
      backendFetch("/api/v1/admin/overview", { headers: bearer(token) }),
    );
    overview = overviewResponse.ok
      ? ((await overviewResponse.json()) as AdminOverview)
      : emptyOverview;
  } catch (error) {
    console.error("[admin-page] failed to load admin overview", error);
  }

  return <AdminPanel adminEmail={me.email} initialOverview={overview} />;
}
