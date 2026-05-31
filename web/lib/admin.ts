export type AdminSeverity = "info" | "warning" | "critical";
export type AdminNotificationStatus = "open" | "resolved";

export type AdminNotification = {
  id: number;
  fingerprint?: string;
  severity: AdminSeverity;
  status: AdminNotificationStatus;
  title: string;
  message: string;
  server_id?: number | null;
  server?: {
    id: number;
    name: string;
    region?: string;
    endpoint?: string;
    country_code?: string;
  } | null;
  metadata_json?: string;
  last_seen_at: string;
  acknowledged_at?: string | null;
  acknowledged_by_id?: number | null;
  muted_until?: string | null;
  resolved_at?: string | null;
  created_at: string;
};

export type AdminServer = {
  id: number;
  name: string;
  host?: string;
  endpoint?: string;
  region?: string;
  country_code?: string;
  active: boolean;
  is_vip_only?: boolean;
  max_peers?: number;
  awg_port?: number;
  agent_url?: string;
  active_keys?: number;
  active_awg?: number;
  active_vless?: number;
  utilization?: number;
  agent_mode?: string;
  agent_node_id?: string;
  agent_last_heartbeat_at?: string | null;
  agent_heartbeat_stale?: boolean;
  agent_docker_available?: boolean;
  agent_uptime_seconds?: number;
  agent_last_health_status?: string;
  agent_last_version?: string;
  agent_last_commit?: string;
  agent_active_digest?: string;
  agent_previous_digest?: string;
  agent_last_update_status?: string;
  agent_last_update_error?: string;
  agent_last_snapshot_hash?: string;
  agent_last_snapshot_at?: string | null;
  agent_last_snapshot_status?: string;
  agent_bootstrap_status?: string;
  agent_bootstrap_error?: string;
  agent_bootstrap_at?: string | null;
  pihole_enabled?: boolean;
  pihole_last_sync_at?: string | null;
  pihole_last_sync_error?: string;
  config_summary?: {
    awg?: {
      listen_port?: number;
      interface?: string;
      container?: string;
      jc?: string;
      jmin?: string;
      jmax?: string;
      s1?: string;
      s2?: string;
      s3?: string;
      s4?: string;
      h1?: string;
      h2?: string;
      h3?: string;
      h4?: string;
    };
    xray?: {
      address?: string;
      port?: number;
      server_name?: string;
      client_id?: string;
      public_key?: string;
      short_id?: string;
      fingerprint?: string;
      mldsa65_verify?: string;
      network?: string;
      security?: string;
      flow?: string;
      spider_x?: string;
      grpc_service_name?: string;
      grpc_authority?: string;
      grpc_multi_mode?: boolean;
      x_http_path?: string;
      x_http_host?: string;
      x_http_mode?: string;
      x_http_padding?: string;
      x_http_post_size?: number;
      hysteria_enabled?: boolean;
      hysteria_port?: number;
      hysteria_password?: string;
      hysteria_sni?: string;
      hysteria_insecure?: boolean;
      hysteria_obfs_password?: string;
      hysteria_masquerade_url?: string;
      container_name?: string;
    };
  };
};

export type AdminOverview = {
  kpis: {
    total_users: number;
    active_subscriptions: number;
    active_vpn_keys: number;
    total_servers: number;
    active_servers: number;
    open_incidents: number;
    monthly_revenue: number;
  };
  notifications: AdminNotification[];
  servers: AdminServer[];
};

export type AdminAuditLog = {
  id: number;
  actor_user_id?: number | null;
  action: string;
  entity: string;
  entity_id: string;
  result: string;
  message: string;
  ip: string;
  created_at: string;
};

export type LegacyAdminUser = {
  id: number;
  email: string;
  role: "admin" | "user";
  created_at: string;
  subscription?: {
    plan?: string;
    status?: string;
    expires_at?: string | null;
  };
};

export type LegacyAdminPayment = {
  id: number;
  user_email?: string;
  amount: number;
  original_amount?: number;
  discount_amount?: number;
  promo_code?: string;
  plan: string;
  status: string;
  created_at: string;
  confirmed_at?: string | null;
};

export type LegacyAdminPromoCode = {
  id: number;
  code: string;
  description?: string;
  discount_percent: number;
  max_uses?: number;
  used_count?: number;
  active: boolean;
  applicable_plans?: string;
  once_per_user?: boolean;
  expires_at?: string | null;
  created_at?: string;
};

export type LegacyAdminDownload = {
  platform: string;
  current_file?: string;
  file_name?: string;
  public_url?: string;
  url?: string;
  allowed_extensions?: string[];
  enabled?: boolean;
  size_bytes?: number;
  uploaded_at?: string | null;
};

export type ServerFilters = {
  query: string;
  status: "all" | "active" | "inactive" | "stale";
  region: string;
  vipOnly: "all" | "vip" | "standard";
};

export function hasUnacknowledgedCriticalIncident(notifications: AdminNotification[]): boolean {
  const now = Date.now();
  return notifications.some((notification) => {
    const mutedUntil = notification.muted_until ? Date.parse(notification.muted_until) : 0;
    return (
      notification.severity === "critical" &&
      notification.status === "open" &&
      !notification.acknowledged_at &&
      (!mutedUntil || mutedUntil <= now)
    );
  });
}

export function mergeNotificationEvent(
  current: AdminNotification[],
  incoming: AdminNotification[],
): AdminNotification[] {
  const byID = new Map<number, AdminNotification>();
  for (const notification of current) byID.set(notification.id, notification);
  for (const notification of incoming) byID.set(notification.id, notification);
  return [...byID.values()].sort(
    (a, b) => Date.parse(b.created_at || b.last_seen_at) - Date.parse(a.created_at || a.last_seen_at),
  );
}

export function applyServerFilters(servers: AdminServer[], filters: ServerFilters): AdminServer[] {
  const query = filters.query.trim().toLowerCase();
  return servers.filter((server) => {
    if (query) {
      const haystack = [server.name, server.region, server.endpoint, server.host, server.country_code]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      if (!haystack.includes(query)) return false;
    }
    if (filters.status === "active" && !server.active) return false;
    if (filters.status === "inactive" && server.active) return false;
    if (filters.status === "stale" && !server.agent_heartbeat_stale) return false;
    if (filters.region && server.region !== filters.region) return false;
    if (filters.vipOnly === "vip" && !server.is_vip_only) return false;
    if (filters.vipOnly === "standard" && server.is_vip_only) return false;
    return true;
  });
}

export function preserveSelectedServer(selectedID: number | null, visibleServers: AdminServer[]): number | null {
  if (visibleServers.length === 0) return null;
  if (selectedID && visibleServers.some((server) => server.id === selectedID)) return selectedID;
  return visibleServers[0].id;
}
