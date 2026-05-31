"use client";

import {
  Activity,
  AlertTriangle,
  Bell,
  Check,
  ChevronRight,
  Cpu,
  DatabaseBackup,
  Gauge,
  Globe2,
  HardDrive,
  History,
  MonitorCog,
  Power,
  Radio,
  RefreshCw,
  RotateCcw,
  Search,
  Server,
  Settings,
  ShieldAlert,
  ShieldCheck,
  Terminal,
  TicketPercent,
  UploadCloud,
  UserRound,
  Users,
  WalletCards,
  X,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import type React from "react";
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  type AdminAuditLog,
  type AdminNotification,
  type AdminOverview,
  type AdminServer,
  type LegacyAdminDownload,
  type LegacyAdminPayment,
  type LegacyAdminPromoCode,
  type LegacyAdminUser,
  applyServerFilters,
  hasUnacknowledgedCriticalIncident,
  mergeNotificationEvent,
  preserveSelectedServer,
  type ServerFilters,
} from "@/lib/admin";
import { SparkAreaChart, UtilizationBars } from "./Charts";

type AdminStats = {
  users_series?: Array<{ label: string; value: number }>;
  revenue_series?: Array<{ label: string; value: number }>;
};

type ServerFormState = {
  mode: "add" | "edit";
  serverID?: number;
  name: string;
  host: string;
  endpoint: string;
  region: string;
  country_code: string;
  max_peers: number;
  awg_port: number;
  is_vip_only: boolean;
  agent_url: string;
  agent_node_id: string;
  ssh_password?: string;
  public_key?: string;
};

type ConfirmDialogState = {
  title: string;
  message: string;
  actionLabel: string;
  danger?: boolean;
  onConfirm: () => Promise<unknown> | unknown;
};

type DigestDialogState = {
  server: AdminServer;
  image_digest: string;
};

type BootstrapDialogState = {
  server: AdminServer;
  agent_image_digest: string;
  xray_image_digest: string;
  management_port: number;
  local_port: number;
  node_id: string;
  server_name: string;
  reality_dest: string;
  force: boolean;
};

const navItems = [
  { id: "overview", label: "Обзор", hint: "Состояние продукта", icon: Gauge },
  { id: "servers", label: "VPS", hint: "Флот и действия", icon: Server },
  { id: "incidents", label: "Инциденты", hint: "SLA и алерты", icon: Bell },
  { id: "users", label: "Пользователи", hint: "Аудитория и ключи", icon: Users },
  { id: "payments", label: "Платежи", hint: "Выручка и риски", icon: WalletCards },
  { id: "promo", label: "Промокоды", hint: "Скидки и лимиты", icon: TicketPercent },
  { id: "downloads", label: "Приложения", hint: "Файлы клиентов", icon: UploadCloud },
  { id: "agent", label: "Агент", hint: "Digest, Docker, snapshot", icon: MonitorCog },
  { id: "settings", label: "Настройки", hint: "AWG/Xray и аудит", icon: Settings },
] as const;

type AdminSection = (typeof navItems)[number]["id"];

const drawerTabs = ["Health", "Agent", "Snapshot", "Config", "Actions", "Audit"] as const;
type DrawerTab = (typeof drawerTabs)[number];

export function AdminPanel({ adminEmail, initialOverview }: { adminEmail: string; initialOverview: AdminOverview }) {
  const [overview, setOverview] = useState(initialOverview);
  const [notifications, setNotifications] = useState(initialOverview.notifications);
  const [activeSection, setActiveSection] = useState<AdminSection>("overview");
  const [filters, setFilters] = useState<ServerFilters>({ query: "", status: "all", region: "", vipOnly: "all" });
  const [selectedServerID, setSelectedServerID] = useState<number | null>(initialOverview.servers[0]?.id ?? null);
  const [drawerTab, setDrawerTab] = useState<DrawerTab>("Health");
  const [stats, setStats] = useState<AdminStats>({});
  const [auditLogs, setAuditLogs] = useState<AdminAuditLog[]>([]);
  const [users, setUsers] = useState<LegacyAdminUser[]>([]);
  const [payments, setPayments] = useState<LegacyAdminPayment[]>([]);
  const [promoCodes, setPromoCodes] = useState<LegacyAdminPromoCode[]>([]);
  const [downloads, setDownloads] = useState<LegacyAdminDownload[]>([]);
  const [legacyLoading, setLegacyLoading] = useState<string>("");
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [toast, setToast] = useState<string>("");
  const [serverModal, setServerModal] = useState<ServerFormState | null>(null);
  const [confirmDialog, setConfirmDialog] = useState<ConfirmDialogState | null>(null);
  const [digestDialog, setDigestDialog] = useState<DigestDialogState | null>(null);
  const [bootstrapDialog, setBootstrapDialog] = useState<BootstrapDialogState | null>(null);
  const reduceMotion = useReducedMotion();

  const servers = overview.servers;
  const visibleServers = useMemo(() => applyServerFilters(servers, filters), [servers, filters]);
  const selectedServer = servers.find((server) => server.id === selectedServerID) ?? visibleServers[0] ?? null;
  const criticalBanner = hasUnacknowledgedCriticalIncident(notifications);
  const openNotifications = notifications.filter((notification) => notification.status === "open");
  const staleServers = servers.filter((server) => server.agent_heartbeat_stale);
  const dockerUnavailable = servers.filter((server) => server.agent_mode === "agent" && server.agent_docker_available === false);
  const fleetHealth = computeFleetHealth(servers, openNotifications);
  const regions = useMemo(
    () => [...new Set(servers.map((server) => server.region).filter(Boolean) as string[])].sort(),
    [servers],
  );

  useEffect(() => {
    setSelectedServerID((current) => preserveSelectedServer(current, visibleServers));
  }, [visibleServers]);

  useEffect(() => {
    if (activeSection === "agent") setDrawerTab("Agent");
    if (activeSection === "settings") setDrawerTab("Config");
    if (activeSection === "servers") setDrawerTab("Health");
    if (activeSection === "users") void loadLegacyResource("users");
    if (activeSection === "payments") void loadLegacyResource("payments");
    if (activeSection === "promo") void loadLegacyResource("promo-codes");
    if (activeSection === "downloads") void loadLegacyResource("downloads");
  }, [activeSection]);

  useEffect(() => {
    void refreshStats();
    const events = new EventSource("/api/admin/events");
    events.addEventListener("notifications", (event) => {
      try {
        const data = JSON.parse((event as MessageEvent).data) as { notifications: AdminNotification[] };
        setNotifications((current) => {
          const merged = mergeNotificationEvent(current, data.notifications ?? []);
          if (hasUnacknowledgedCriticalIncident(data.notifications ?? [])) {
            setToast("Новый критический инцидент");
          }
          return merged;
        });
      } catch {
        setToast("Не удалось обработать поток событий");
      }
    });
    events.onerror = () => {
      events.close();
    };
    const interval = window.setInterval(() => {
      void refreshOverview();
      void refreshStats();
    }, 15000);
    return () => {
      events.close();
      window.clearInterval(interval);
    };
  }, []);

  useEffect(() => {
    if (drawerTab === "Audit" && selectedServer) {
      void refreshAudit(selectedServer.id);
    }
  }, [drawerTab, selectedServer?.id]);

  async function refreshOverview() {
    try {
      const response = await fetch("/api/admin/overview", { cache: "no-store" });
      if (!response.ok) throw new Error("Overview недоступен");
      const data = (await response.json()) as AdminOverview;
      setOverview(data);
      setNotifications(data.notifications);
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Не удалось обновить обзор");
    }
  }

  async function refreshStats() {
    try {
      const response = await fetch("/api/admin/stats?days=7", { cache: "no-store" });
      if (response.ok) setStats((await response.json()) as AdminStats);
    } catch {
      setStats({});
    }
  }

  async function refreshAudit(serverID: number) {
    try {
      const response = await fetch(`/api/admin/audit?entity=vpn_server&entity_id=${serverID}`, { cache: "no-store" });
      if (response.ok) {
        const data = (await response.json()) as { audit_logs: AdminAuditLog[] };
        setAuditLogs(data.audit_logs ?? []);
      }
    } catch {
      setAuditLogs([]);
    }
  }

  async function loadLegacyResource(resource: "users" | "payments" | "promo-codes" | "downloads") {
    setLegacyLoading(resource);
    try {
      const response = await fetch(`/api/admin/${resource}`, { cache: "no-store" });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error ?? `Не удалось загрузить ${resource}`);
      if (resource === "users") setUsers(data.users ?? []);
      if (resource === "payments") setPayments(data.payments ?? []);
      if (resource === "promo-codes") setPromoCodes(data.promo_codes ?? []);
      if (resource === "downloads") setDownloads(data.downloads ?? []);
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Ошибка загрузки данных");
    } finally {
      setLegacyLoading("");
    }
  }

  async function exportCSV(entity: "users" | "servers" | "payments" | "promo-codes") {
    try {
      const response = await fetch(`/api/admin/export/${entity}`, { cache: "no-store" });
      if (!response.ok) throw new Error("Не удалось скачать CSV");
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `fblink-${entity}.csv`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Ошибка экспорта");
    }
  }

  async function uploadDownload(platform: string, file: File) {
    setBusyAction(`download-${platform}`);
    try {
      const body = new FormData();
      body.set("file", file);
      const response = await fetch(`/api/admin/downloads/${encodeURIComponent(platform)}`, {
        method: "POST",
        body,
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error ?? "Не удалось загрузить файл");
      setToast("Файл приложения загружен");
      await loadLegacyResource("downloads");
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Ошибка загрузки файла");
    } finally {
      setBusyAction(null);
    }
  }

  function openAddServerModal() {
    setServerModal({
      mode: "add",
      name: "",
      host: "",
      endpoint: "",
      region: "",
      country_code: "",
      max_peers: 100,
      awg_port: 51820,
      is_vip_only: false,
      agent_url: "",
      agent_node_id: "",
      ssh_password: "",
      public_key: "",
    });
  }

  function openEditServerModal(server: AdminServer) {
    setServerModal({
      mode: "edit",
      serverID: server.id,
      name: server.name,
      host: server.host ?? "",
      endpoint: server.endpoint ?? "",
      region: server.region ?? "",
      country_code: server.country_code ?? "",
      max_peers: server.max_peers ?? 100,
      awg_port: server.awg_port ?? 51820,
      is_vip_only: Boolean(server.is_vip_only),
      agent_url: server.agent_url ?? "",
      agent_node_id: server.agent_node_id ?? "",
      ssh_password: "", // Not returned by API
      public_key: "",   // Usually not needed during edit unless changed
    });
  }

  async function submitServerModal() {
    if (!serverModal) return;
    if (!serverModal.name.trim()) return setToast("Название сервера обязательно");
    if (serverModal.mode === "add" && !serverModal.host.trim()) return setToast("Host/IP обязателен для нового сервера");
    const payload = {
      name: serverModal.name.trim(),
      host: serverModal.host.trim(),
      endpoint: serverModal.endpoint.trim(),
      region: serverModal.region.trim(),
      country_code: serverModal.country_code.trim(),
      max_peers: Number(serverModal.max_peers) || 100,
      awg_port: Number(serverModal.awg_port) || 51820,
      is_vip_only: serverModal.is_vip_only,
      agent_url: serverModal.agent_url.trim(),
      agent_node_id: serverModal.agent_node_id.trim(),
      ssh_password: serverModal.ssh_password?.trim() || undefined,
      public_key: serverModal.public_key?.trim() || undefined,
    };
    const ok = await postAction(
      serverModal.mode === "add" ? "server-add" : `server-edit-${serverModal.serverID}`,
      serverModal.mode === "add" ? "/api/admin/servers" : `/api/admin/servers/${serverModal.serverID}`,
      payload,
      serverModal.mode === "add" ? "POST" : "PUT",
    );
    if (ok) {
      setServerModal(null);
      await refreshOverview();
    }
  }

  async function submitDigestDialog() {
    if (!digestDialog) return;
    const digest = digestDialog.image_digest.trim();
    if (!digest.includes("@sha256:")) return setToast("Нужен immutable digest вида image@sha256:...");
    const ok = await postAction(`update-${digestDialog.server.id}`, `/api/admin/servers/${digestDialog.server.id}/agent/update`, { image_digest: digest });
    if (ok) setDigestDialog(null);
  }

  async function submitBootstrapDialog() {
    if (!bootstrapDialog) return;
    if (!bootstrapDialog.agent_image_digest.includes("@sha256:") || !bootstrapDialog.xray_image_digest.includes("@sha256:")) {
      return setToast("Agent и Xray digest должны быть immutable image@sha256:...");
    }
    const ok = await postAction(`bootstrap-${bootstrapDialog.server.id}`, `/api/admin/servers/${bootstrapDialog.server.id}/agent/bootstrap`, {
      agent_image_digest: bootstrapDialog.agent_image_digest.trim(),
      xray_image_digest: bootstrapDialog.xray_image_digest.trim(),
      management_port: Number(bootstrapDialog.management_port) || undefined,
      local_port: Number(bootstrapDialog.local_port) || undefined,
      node_id: bootstrapDialog.node_id.trim(),
      server_name: bootstrapDialog.server_name.trim(),
      reality_dest: bootstrapDialog.reality_dest.trim(),
      force: bootstrapDialog.force,
    });
    if (ok) setBootstrapDialog(null);
  }

  function openBootstrapDialog(server: AdminServer) {
    setBootstrapDialog({
      server,
      agent_image_digest: "",
      xray_image_digest: "",
      management_port: 39000 + server.id,
      local_port: 19000 + server.id,
      node_id: server.agent_node_id || `server-${server.id}`,
      server_name: server.config_summary?.xray?.server_name || "www.microsoft.com",
      reality_dest: `${server.config_summary?.xray?.server_name || "www.microsoft.com"}:443`,
      force: false,
    });
  }

  async function confirmAction(dialog: ConfirmDialogState) {
    setConfirmDialog(null);
    await dialog.onConfirm();
  }

  async function toggleServer(server: AdminServer) {
    const ok = await postAction(`toggle-${server.id}`, `/api/admin/servers/${server.id}/toggle`);
    if (ok) await refreshOverview();
  }

  async function deleteServer(server: AdminServer) {
    setConfirmDialog({
      title: "Удалить сервер",
      message: `Удалить ${server.name} и связанные ключи? Это действие нельзя быстро откатить.`,
      actionLabel: "Удалить",
      danger: true,
      onConfirm: async () => {
        const ok = await postAction(`server-delete-${server.id}`, `/api/admin/servers/${server.id}`, undefined, "DELETE");
        if (ok) await refreshOverview();
      },
    });
  }

  async function runPiHoleSync() {
    const ok = await postAction("pihole-sync", "/api/admin/servers/pihole-sync", { force: true });
    if (ok) await refreshOverview();
  }

  async function postAction(label: string, url: string, body?: unknown, method = "POST") {
    setBusyAction(label);
    try {
      const response = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: method === "GET" ? undefined : body ? JSON.stringify(body) : "{}",
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error ?? "Команда не выполнена");
      setToast("Команда выполнена");
      await refreshOverview();
      if (selectedServer) await refreshAudit(selectedServer.id);
      return true;
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Ошибка команды");
      return false;
    } finally {
      setBusyAction(null);
    }
  }

  async function refreshServerHealth(serverID: number) {
    setBusyAction(`health-${serverID}`);
    try {
      const response = await fetch(`/api/admin/servers/${serverID}/health`, { cache: "no-store" });
      if (!response.ok) throw new Error("Health недоступен");
      const data = (await response.json()) as { server: AdminServer; notifications: AdminNotification[] };
      setOverview((current) => ({
        ...current,
        servers: current.servers.map((server) => (server.id === serverID ? { ...server, ...data.server } : server)),
      }));
      setNotifications((current) => mergeNotificationEvent(current, data.notifications ?? []));
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Ошибка health refresh");
    } finally {
      setBusyAction(null);
    }
  }

  async function ackNotification(id: number) {
    await postAction(`ack-${id}`, `/api/admin/notifications/${id}/ack`);
  }

  async function muteNotification(id: number) {
    await postAction(`mute-${id}`, `/api/admin/notifications/${id}/mute`, { minutes: 30 });
  }

  const kpis = overview.kpis;
  const userPoints = stats.users_series?.length ? stats.users_series : [{ label: "сейчас", value: kpis.total_users }];
  const revenuePoints = stats.revenue_series?.length ? stats.revenue_series : [{ label: "месяц", value: Math.round(kpis.monthly_revenue) }];

  return (
    <div className="min-h-screen bg-[#050608] text-zinc-100">
      <a href="#admin-main" className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-lg focus:bg-amber-300 focus:px-3 focus:py-2 focus:text-black">
        Перейти к содержимому
      </a>
      {toast ? (
        <div role="status" aria-live="polite" className="fixed right-4 top-4 z-50 rounded-lg border border-amber-300/35 bg-zinc-950 px-4 py-3 text-sm shadow-2xl">
          {toast}
          <button className="ml-3 text-zinc-400 hover:text-white" onClick={() => setToast("")} aria-label="Закрыть уведомление">
            <X size={14} />
          </button>
        </div>
      ) : null}

      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r border-white/10 bg-black/70 p-4 backdrop-blur xl:block">
        <div className="mb-6 rounded-lg border border-amber-300/20 bg-amber-300/[0.06] p-4">
          <div className="flex items-center gap-3">
            <img src="/brand-icon.png" alt="FBLink VPN" className="h-9 w-9" />
            <div>
              <div className="text-lg font-bold">FBLink VPN</div>
              <div className="text-xs text-amber-100/65">Admin cockpit</div>
            </div>
          </div>
        </div>
        <nav aria-label="Админ навигация" className="space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = activeSection === item.id;
            return (
              <button
                key={item.label}
                type="button"
                aria-current={active ? "page" : undefined}
                onClick={() => setActiveSection(item.id)}
                className={`group relative flex min-h-12 w-full cursor-pointer items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm transition ${active ? "bg-amber-300 text-black shadow-[0_10px_32px_rgba(250,204,21,0.22)]" : "text-zinc-400 hover:bg-white/5 hover:text-white"}`}
              >
                <Icon size={16} />
                <span className="min-w-0">
                  <span className="block font-semibold leading-4">{item.label}</span>
                  <span className={`block truncate text-[11px] leading-4 ${active ? "text-black/60" : "text-zinc-600 group-hover:text-zinc-400"}`}>{item.hint}</span>
                </span>
              </button>
            );
          })}
        </nav>
      </aside>

      <main id="admin-main" className="xl:pl-64">
        <header className="sticky top-0 z-30 border-b border-white/10 bg-[#050608]/92 px-4 py-3 backdrop-blur">
          <div className="flex flex-wrap items-center gap-3">
            <div className="relative min-w-[220px] flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" size={16} />
              <input
                value={filters.query}
                onChange={(event) => setFilters((current) => ({ ...current, query: event.target.value }))}
                className="h-10 w-full rounded-lg border border-white/10 bg-white/[0.035] pl-9 pr-3 text-sm outline-none transition focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20"
                placeholder="Поиск VPS, endpoint, регион"
              />
            </div>
            <select className="h-10 rounded-lg border border-white/10 bg-zinc-950 px-3 text-sm text-zinc-200" aria-label="Окружение">
              <option>Production</option>
            </select>
            <button className="relative rounded-lg border border-white/10 bg-white/[0.035] p-2.5" aria-label="Уведомления">
              <Bell size={16} />
              {notifications.length ? <span className="absolute -right-1 -top-1 rounded-full bg-red-500 px-1.5 text-[10px] font-bold">{notifications.length}</span> : null}
            </button>
            <div className="flex items-center gap-2 rounded-lg border border-white/10 bg-white/[0.035] px-3 py-2 text-sm text-zinc-300">
              <UserRound size={16} />
              <span className="max-w-[180px] truncate">{adminEmail}</span>
            </div>
          </div>
        </header>

        <div className="grid gap-4 p-4 2xl:grid-cols-[minmax(0,1fr)_360px]">
          <section className="space-y-4">
            {criticalBanner ? (
              <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-red-400/40 bg-red-500/10 p-4">
                <div className="flex items-center gap-3">
                  <AlertTriangle className="text-red-300" size={22} />
                  <div>
                    <div className="font-semibold text-red-100">1 VPS недоступен или heartbeat устарел</div>
                    <div className="text-sm text-red-100/70">Проверьте agent status, последний snapshot и сетевую доступность.</div>
                  </div>
                </div>
                <Button size="sm" variant="secondary" onClick={() => notifications[0] && ackNotification(notifications[0].id)}>
                  <Check size={14} /> Принять
                </Button>
              </div>
            ) : null}

            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
              <Kpi icon={ShieldCheck} label="Uptime fleet" value={`${kpis.active_servers}/${kpis.total_servers}`} hint="активные VPS" />
              <Kpi icon={Server} label="VPS" value={String(kpis.total_servers)} hint={`${kpis.open_incidents} инцидентов`} />
              <Kpi icon={Users} label="Пользователи" value={kpis.total_users.toLocaleString("ru-RU")} hint={`${kpis.active_subscriptions} активных`} />
              <Kpi icon={Activity} label="Ключи" value={kpis.active_vpn_keys.toLocaleString("ru-RU")} hint="AWG + VLESS" />
              <Kpi icon={WalletCards} label="MRR" value={`${Math.round(kpis.monthly_revenue).toLocaleString("ru-RU")} ₽`} hint="текущий месяц" />
            </div>

            <AnimatePresence mode="wait">
              <motion.div
                key={activeSection}
                initial={reduceMotion ? false : { opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={reduceMotion ? undefined : { opacity: 0, y: -8 }}
                transition={{ duration: 0.18, ease: "easeOut" }}
              >
                <SectionFocus
                  section={activeSection}
                  fleetHealth={fleetHealth}
                  servers={servers}
                  selectedServer={selectedServer}
                  staleServers={staleServers}
                  dockerUnavailable={dockerUnavailable}
                  notifications={notifications}
                  userPoints={userPoints}
                  revenuePoints={revenuePoints}
                  users={users}
                  payments={payments}
                  promoCodes={promoCodes}
                  downloads={downloads}
                  legacyLoading={legacyLoading}
                  busyAction={busyAction}
                  onSelectSection={setActiveSection}
                  onSelectServer={setSelectedServerID}
                  onRefreshHealth={refreshServerHealth}
                  onAction={postAction}
                  onLoadLegacy={loadLegacyResource}
                  onExport={exportCSV}
                  onUploadDownload={uploadDownload}
                  onConfirm={setConfirmDialog}
                />
              </motion.div>
            </AnimatePresence>

            <div className="rounded-lg border border-white/10 bg-zinc-950/70">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/10 p-4">
                <div>
                  <h1 className="text-xl font-bold">VPS health</h1>
                  <p className="text-sm text-zinc-500">Agent-first контроль, heartbeat, нагрузка, протоколы и быстрые действия.</p>
                </div>
                <div className="flex flex-wrap gap-2">
                  <FilterSelect label="Статус" value={filters.status} onChange={(status) => setFilters((current) => ({ ...current, status: status as ServerFilters["status"] }))} options={[["all", "Все"], ["active", "Активные"], ["inactive", "Отключены"], ["stale", "Stale"]]} />
                  <FilterSelect label="Регион" value={filters.region} onChange={(region) => setFilters((current) => ({ ...current, region }))} options={[["", "Все регионы"], ...regions.map((region) => [region, region] as [string, string])]} />
                  <FilterSelect label="VIP" value={filters.vipOnly} onChange={(vipOnly) => setFilters((current) => ({ ...current, vipOnly: vipOnly as ServerFilters["vipOnly"] }))} options={[["all", "Все"], ["vip", "VIP"], ["standard", "Standard"]]} />
                  <Button size="sm" variant="secondary" onClick={() => void refreshOverview()}>
                    <RefreshCw size={14} /> Обновить
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => void exportCSV("servers")}>CSV</Button>
                  <Button size="sm" variant="secondary" onClick={() => void runPiHoleSync()} disabled={busyAction === "pihole-sync"}>Pi-hole Sync</Button>
                  <Button size="sm" onClick={openAddServerModal}>Добавить сервер</Button>
                </div>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[980px] text-left text-sm">
                  <thead className="border-b border-white/10 text-xs uppercase text-zinc-500">
                    <tr>
                      <th className="px-4 py-3 font-medium">Сервер</th>
                      <th className="px-4 py-3 font-medium">Agent</th>
                      <th className="px-4 py-3 font-medium">Клиенты</th>
                      <th className="px-4 py-3 font-medium">Нагрузка</th>
                      <th className="px-4 py-3 font-medium">Heartbeat</th>
                      <th className="px-4 py-3 font-medium">Статус</th>
                      <th className="px-4 py-3 font-medium">Действия</th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleServers.map((server) => (
                      <tr key={server.id} className={`border-b border-white/[0.06] hover:bg-white/[0.03] ${selectedServer?.id === server.id ? "bg-amber-300/[0.04]" : ""}`}>
                        <td className="px-4 py-3">
                          <button className="text-left" onClick={() => setSelectedServerID(server.id)}>
                            <span className="block font-semibold text-zinc-100">{server.name}</span>
                            <span className="block text-xs text-zinc-500">{server.region || "—"} · {server.endpoint || server.host}</span>
                          </button>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-zinc-400">{server.agent_mode || "manual"}</td>
                        <td className="px-4 py-3 text-zinc-300">{server.active_awg ?? 0} AWG / {server.active_vless ?? 0} VLESS</td>
                        <td className="px-4 py-3"><MiniMeter value={server.utilization ?? 0} /></td>
                        <td className="px-4 py-3 text-xs text-zinc-400">{formatRelative(server.agent_last_heartbeat_at)}</td>
                        <td className="px-4 py-3"><StatusBadge server={server} /></td>
                        <td className="px-4 py-3">
                          <div className="flex gap-2">
                            <IconButton label="Health" onClick={() => refreshServerHealth(server.id)} icon={RefreshCw} active={busyAction === `health-${server.id}`} />
                            <IconButton label="Snapshot" onClick={() => postAction(`snapshot-${server.id}`, `/api/admin/servers/${server.id}/agent/snapshot`)} icon={DatabaseBackup} active={busyAction === `snapshot-${server.id}`} />
                            <IconButton label="Edit" onClick={() => openEditServerModal(server)} icon={Settings} active={busyAction === `server-edit-${server.id}`} />
                            <IconButton label="Toggle active" onClick={() => setConfirmDialog({
                              title: server.active ? "Отключить сервер" : "Включить сервер",
                              message: `${server.active ? "Отключить" : "Включить"} ${server.name}?`,
                              actionLabel: server.active ? "Отключить" : "Включить",
                              onConfirm: () => toggleServer(server),
                            })} icon={Power} active={busyAction === `toggle-${server.id}`} />
                            <IconButton label="Details" onClick={() => setSelectedServerID(server.id)} icon={ChevronRight} />
                            <IconButton label="Delete" onClick={() => deleteServer(server)} icon={X} active={busyAction === `server-delete-${server.id}`} />
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            <div className="grid gap-4 xl:grid-cols-3">
              <SparkAreaChart title="Новые пользователи" points={userPoints} color="#facc15" />
              <SparkAreaChart title="Выручка" points={revenuePoints} color="#22c55e" />
              <UtilizationBars items={servers} />
            </div>
          </section>

          <aside className="space-y-4">
            <NotificationInbox notifications={notifications} onAck={ackNotification} onMute={muteNotification} />
            {selectedServer ? (
              <ServerDrawer
                server={selectedServer}
                tab={drawerTab}
                setTab={setDrawerTab}
                auditLogs={auditLogs}
                busyAction={busyAction}
                onAction={postAction}
                onRefreshHealth={refreshServerHealth}
                onOpenDigest={setDigestDialog}
                onOpenBootstrap={openBootstrapDialog}
                onConfirm={setConfirmDialog}
              />
            ) : null}
          </aside>
        </div>
      </main>
      <ServerEditModal state={serverModal} busy={busyAction === "server-add" || Boolean(serverModal?.serverID && busyAction === `server-edit-${serverModal.serverID}`)} onChange={setServerModal} onClose={() => setServerModal(null)} onSubmit={submitServerModal} />
      <DigestModal state={digestDialog} busy={Boolean(digestDialog && busyAction === `update-${digestDialog.server.id}`)} onChange={setDigestDialog} onClose={() => setDigestDialog(null)} onSubmit={submitDigestDialog} />
      <BootstrapModal state={bootstrapDialog} busy={Boolean(bootstrapDialog && busyAction === `bootstrap-${bootstrapDialog.server.id}`)} onChange={setBootstrapDialog} onClose={() => setBootstrapDialog(null)} onSubmit={submitBootstrapDialog} />
      <ConfirmModal state={confirmDialog} busy={Boolean(busyAction)} onClose={() => setConfirmDialog(null)} onConfirm={confirmAction} />
    </div>
  );
}

function ModalShell({ title, children, footer, onClose }: { title: string; children: React.ReactNode; footer: React.ReactNode; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-[80] grid place-items-center bg-black/70 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-label={title}>
      <motion.div initial={{ opacity: 0, y: 12, scale: 0.98 }} animate={{ opacity: 1, y: 0, scale: 1 }} className="max-h-[92vh] w-full max-w-2xl overflow-hidden rounded-lg border border-white/10 bg-zinc-950 shadow-2xl">
        <div className="flex items-center justify-between border-b border-white/10 px-5 py-4">
          <h2 className="text-lg font-bold">{title}</h2>
          <button className="rounded-lg p-2 text-zinc-400 transition hover:bg-white/10 hover:text-white" onClick={onClose} aria-label="Закрыть">
            <X size={16} />
          </button>
        </div>
        <div className="max-h-[68vh] overflow-auto p-5">{children}</div>
        <div className="flex flex-wrap justify-end gap-2 border-t border-white/10 px-5 py-4">{footer}</div>
      </motion.div>
    </div>
  );
}

function ServerEditModal({ state, busy, onChange, onClose, onSubmit }: { state: ServerFormState | null; busy: boolean; onChange: (state: ServerFormState | null) => void; onClose: () => void; onSubmit: () => Promise<void> }) {
  if (!state) return null;
  const patch = (updates: Partial<ServerFormState>) => onChange({ ...state, ...updates });
  return (
    <ModalShell
      title={state.mode === "add" ? "Добавить VPS" : "Редактировать VPS"}
      onClose={onClose}
      footer={<><Button variant="ghost" onClick={onClose}>Отмена</Button><Button onClick={onSubmit} disabled={busy}>{busy ? "Сохраняю..." : "Сохранить"}</Button></>}
    >
      <div className="grid gap-3 md:grid-cols-2">
        <ModalInput label="Название" value={state.name} onChange={(name) => patch({ name })} />
        <ModalInput label="Host/IP backend" value={state.host} onChange={(host) => patch({ host })} disabled={state.mode === "edit"} />
        <ModalInput label="Endpoint клиента" value={state.endpoint} onChange={(endpoint) => patch({ endpoint })} />
        <ModalInput label="Регион" value={state.region} onChange={(region) => patch({ region })} />
        <ModalInput label="Country code" value={state.country_code} onChange={(country_code) => patch({ country_code })} />
        <ModalInput label="Лимит peers" type="number" value={String(state.max_peers)} onChange={(max_peers) => patch({ max_peers: Number(max_peers) })} />
        <ModalInput label="AWG port" type="number" value={String(state.awg_port)} onChange={(awg_port) => patch({ awg_port: Number(awg_port) })} />
        <ModalInput label="Agent URL" value={state.agent_url} onChange={(agent_url) => patch({ agent_url })} />
        <ModalInput label="Agent node id" value={state.agent_node_id} onChange={(agent_node_id) => patch({ agent_node_id })} />
        <ModalInput label="SSH Password" type="password" value={state.ssh_password || ""} onChange={(ssh_password) => patch({ ssh_password })} placeholder="Для получения public key или деплоя" />
        <ModalInput label="AWG Public Key" value={state.public_key || ""} onChange={(public_key) => patch({ public_key })} placeholder="Если нет SSH" />
        <label className="flex min-h-11 items-center gap-2 rounded-lg border border-white/10 bg-black/30 px-3 text-sm text-zinc-300">
          <input type="checkbox" checked={state.is_vip_only} onChange={(event) => patch({ is_vip_only: event.target.checked })} />
          VIP-only сервер
        </label>
      </div>
    </ModalShell>
  );
}

function DigestModal({ state, busy, onChange, onClose, onSubmit }: { state: DigestDialogState | null; busy: boolean; onChange: (state: DigestDialogState | null) => void; onClose: () => void; onSubmit: () => Promise<void> }) {
  if (!state) return null;
  return (
    <ModalShell
      title={`Update agent: ${state.server.name}`}
      onClose={onClose}
      footer={<><Button variant="ghost" onClick={onClose}>Отмена</Button><Button onClick={onSubmit} disabled={busy}>{busy ? "Запускаю..." : "Обновить"}</Button></>}
    >
      <p className="mb-3 text-sm text-zinc-400">Укажите immutable digest образа agent. Тег `latest` не принимается backend-ом.</p>
      <ModalInput label="Agent image digest" value={state.image_digest} onChange={(image_digest) => onChange({ ...state, image_digest })} placeholder="registry/fblink-agent@sha256:..." />
    </ModalShell>
  );
}

function BootstrapModal({ state, busy, onChange, onClose, onSubmit }: { state: BootstrapDialogState | null; busy: boolean; onChange: (state: BootstrapDialogState | null) => void; onClose: () => void; onSubmit: () => Promise<void> }) {
  if (!state) return null;
  const patch = (updates: Partial<BootstrapDialogState>) => onChange({ ...state, ...updates });
  return (
    <ModalShell
      title={`Bootstrap agent: ${state.server.name}`}
      onClose={onClose}
      footer={<><Button variant="ghost" onClick={onClose}>Отмена</Button><Button onClick={onSubmit} disabled={busy}>{busy ? "Bootstrap..." : "Запустить bootstrap"}</Button></>}
    >
      <div className="mb-4 rounded-lg border border-amber-300/20 bg-amber-300/[0.06] p-3 text-sm text-amber-50/80">
        Нужны два immutable digest: agent и management Xray. Backend также требует сохранённый SSH password у сервера и валидный `AGENT_SIGNING_PRIVATE_KEY`.
      </div>
      <div className="grid gap-3">
        <ModalInput label="Agent image digest" value={state.agent_image_digest} onChange={(agent_image_digest) => patch({ agent_image_digest })} placeholder="registry/fblink-agent@sha256:..." />
        <ModalInput label="Xray image digest" value={state.xray_image_digest} onChange={(xray_image_digest) => patch({ xray_image_digest })} placeholder="teddysun/xray@sha256:..." />
        <div className="grid gap-3 md:grid-cols-2">
          <ModalInput label="Management port" type="number" value={String(state.management_port)} onChange={(value) => patch({ management_port: Number(value) })} />
          <ModalInput label="Local tunnel port" type="number" value={String(state.local_port)} onChange={(value) => patch({ local_port: Number(value) })} />
          <ModalInput label="Node ID" value={state.node_id} onChange={(node_id) => patch({ node_id })} />
          <ModalInput label="Reality server name" value={state.server_name} onChange={(server_name) => patch({ server_name, reality_dest: state.reality_dest || `${server_name}:443` })} />
          <ModalInput label="Reality dest" value={state.reality_dest} onChange={(reality_dest) => patch({ reality_dest })} />
          <label className="flex min-h-11 items-center gap-2 rounded-lg border border-white/10 bg-black/30 px-3 text-sm text-zinc-300">
            <input type="checkbox" checked={state.force} onChange={(event) => patch({ force: event.target.checked })} />
            Force reinstall
          </label>
        </div>
      </div>
    </ModalShell>
  );
}

function ConfirmModal({ state, busy, onClose, onConfirm }: { state: ConfirmDialogState | null; busy: boolean; onClose: () => void; onConfirm: (state: ConfirmDialogState) => Promise<void> }) {
  if (!state) return null;
  return (
    <ModalShell
      title={state.title}
      onClose={onClose}
      footer={<><Button variant="ghost" onClick={onClose}>Отмена</Button><Button variant={state.danger ? "destructive" : "default"} onClick={() => onConfirm(state)} disabled={busy}>{state.actionLabel}</Button></>}
    >
      <p className="text-sm leading-6 text-zinc-300">{state.message}</p>
    </ModalShell>
  );
}

function ModalInput({ label, value, onChange, type = "text", placeholder, disabled = false }: { label: string; value: string; onChange: (value: string) => void; type?: string; placeholder?: string; disabled?: boolean }) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold text-zinc-500">{label}</span>
      <input
        value={value}
        type={type}
        disabled={disabled}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="h-10 w-full rounded-lg border border-white/10 bg-black/35 px-3 text-sm text-zinc-100 outline-none transition focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20 disabled:opacity-60"
      />
    </label>
  );
}

function Kpi({ icon: Icon, label, value, hint }: { icon: LucideIcon; label: string; value: string; hint: string }) {
  return (
    <div className="rounded-lg border border-white/10 bg-white/[0.035] p-4">
      <Icon size={18} className="mb-4 text-amber-200" />
      <div className="text-xs uppercase tracking-wide text-zinc-500">{label}</div>
      <div className="mt-1 text-2xl font-bold text-zinc-50">{value}</div>
      <div className="mt-1 text-xs text-zinc-500">{hint}</div>
    </div>
  );
}

function computeFleetHealth(servers: AdminServer[], notifications: AdminNotification[]) {
  if (servers.length === 0) return 0;
  const stalePenalty = servers.filter((server) => server.agent_heartbeat_stale).length * 22;
  const inactivePenalty = servers.filter((server) => !server.active).length * 12;
  const dockerPenalty = servers.filter((server) => server.agent_mode === "agent" && server.agent_docker_available === false).length * 14;
  const incidentPenalty = notifications.filter((notification) => notification.status === "open").length * 8;
  return Math.max(0, Math.min(100, 100 - stalePenalty - inactivePenalty - dockerPenalty - incidentPenalty));
}

function SectionFocus({
  section,
  fleetHealth,
  servers,
  selectedServer,
  staleServers,
  dockerUnavailable,
  notifications,
  userPoints,
  revenuePoints,
  users,
  payments,
  promoCodes,
  downloads,
  legacyLoading,
  busyAction,
  onSelectSection,
  onSelectServer,
  onRefreshHealth,
  onAction,
  onLoadLegacy,
  onExport,
  onUploadDownload,
  onConfirm,
}: {
  section: AdminSection;
  fleetHealth: number;
  servers: AdminServer[];
  selectedServer: AdminServer | null;
  staleServers: AdminServer[];
  dockerUnavailable: AdminServer[];
  notifications: AdminNotification[];
  userPoints: Array<{ label: string; value: number }>;
  revenuePoints: Array<{ label: string; value: number }>;
  users: LegacyAdminUser[];
  payments: LegacyAdminPayment[];
  promoCodes: LegacyAdminPromoCode[];
  downloads: LegacyAdminDownload[];
  legacyLoading: string;
  busyAction: string | null;
  onSelectSection: (section: AdminSection) => void;
  onSelectServer: (id: number) => void;
  onRefreshHealth: (serverID: number) => Promise<void>;
  onAction: (label: string, url: string, body?: unknown, method?: string) => Promise<boolean>;
  onLoadLegacy: (resource: "users" | "payments" | "promo-codes" | "downloads") => Promise<void>;
  onExport: (entity: "users" | "servers" | "payments" | "promo-codes") => Promise<void>;
  onUploadDownload: (platform: string, file: File) => Promise<void>;
  onConfirm: (state: ConfirmDialogState) => void;
}) {
  if (section === "incidents") {
    return (
      <div className="grid gap-4 xl:grid-cols-[1.2fr_0.8fr]">
        <PanelHeader
          icon={ShieldAlert}
          title="Инциденты и уведомления"
          text="Открытые алерты, stale heartbeat и быстрый переход к проблемной VPS."
        />
        <div className="rounded-lg border border-white/10 bg-white/[0.035] p-4">
          <div className="mb-3 text-sm font-semibold text-zinc-100">Очередь реакции</div>
          <div className="space-y-2">
            {notifications.slice(0, 5).map((notification) => (
              <button
                key={notification.id}
                type="button"
                onClick={() => notification.server_id && onSelectServer(notification.server_id)}
                className="flex min-h-12 w-full cursor-pointer items-center justify-between gap-3 rounded-lg border border-white/10 bg-black/20 px-3 text-left transition hover:border-amber-300/35 hover:bg-amber-300/[0.06]"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-semibold">{notification.title}</span>
                  <span className="block truncate text-xs text-zinc-500">{notification.server?.name ?? "Fleet"} · {notification.status}</span>
                </span>
                <span className={notification.severity === "critical" ? "text-red-300" : "text-amber-200"}>{notification.severity}</span>
              </button>
            ))}
            {notifications.length === 0 ? <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-zinc-500">Инцидентов нет.</div> : null}
          </div>
        </div>
      </div>
    );
  }

  if (section === "users") {
    return (
      <div className="space-y-4">
        <div className="grid gap-4 xl:grid-cols-3">
          <StatusCard icon={Users} label="Аудитория" value={String(users.length || userPoints.at(-1)?.value || 0)} detail="Пользователи из legacy API" />
          <StatusCard icon={ShieldCheck} label="Активные ключи" value={String(servers.reduce((sum, server) => sum + (server.active_keys ?? 0), 0))} detail="Распределены по VPS" />
          <PanelHeader icon={Globe2} title="Управление пользователями" text="Выдача тарифов, отзыв подписки и ключей, смена роли, удаление и CSV экспорт как в старой панели." />
        </div>
        <UsersManager users={users} loading={legacyLoading === "users"} onLoad={() => onLoadLegacy("users")} onExport={() => onExport("users")} onAction={onAction} onConfirm={onConfirm} />
      </div>
    );
  }

  if (section === "payments") {
    return (
      <div className="space-y-4">
        <div className="grid gap-4 xl:grid-cols-[1fr_1fr_0.8fr]">
          <SparkAreaChart title="Выручка по дням" points={revenuePoints} color="#22c55e" />
          <SparkAreaChart title="Рост пользователей" points={userPoints} color="#facc15" />
          <PanelHeader icon={WalletCards} title="Финансовый контроль" text="История платежей, фильтры, ручное подтверждение pending-платежей и экспорт CSV." />
        </div>
        <PaymentsManager payments={payments} loading={legacyLoading === "payments"} onLoad={() => onLoadLegacy("payments")} onExport={() => onExport("payments")} onAction={onAction} onConfirm={onConfirm} />
      </div>
    );
  }

  if (section === "promo") {
    return (
      <PromoCodesManager
        promoCodes={promoCodes}
        loading={legacyLoading === "promo-codes"}
        busyAction={busyAction}
        onLoad={() => onLoadLegacy("promo-codes")}
        onExport={() => onExport("promo-codes")}
        onAction={onAction}
        onConfirm={onConfirm}
      />
    );
  }

  if (section === "downloads") {
    return (
      <DownloadsManager
        downloads={downloads}
        loading={legacyLoading === "downloads"}
        busyAction={busyAction}
        onLoad={() => onLoadLegacy("downloads")}
        onUpload={onUploadDownload}
      />
    );
  }

  if (section === "agent") {
    return (
      <div className="grid gap-4 xl:grid-cols-4">
        <StatusCard icon={Radio} label="Heartbeat stale" value={String(staleServers.length)} detail="Требуют проверки агента" danger={staleServers.length > 0} />
        <StatusCard icon={HardDrive} label="Docker unavailable" value={String(dockerUnavailable.length)} detail="Agent не видит Docker" danger={dockerUnavailable.length > 0} />
        <StatusCard icon={Terminal} label="Digest tracked" value={String(servers.filter((server) => server.agent_active_digest).length)} detail="Immutable update ready" />
        <StatusCard icon={Cpu} label="Snapshot status" value={String(servers.filter((server) => server.agent_last_snapshot_hash).length)} detail="Есть актуальный hash" />
      </div>
    );
  }

  if (section === "settings") {
    return (
      <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
        <PanelHeader
          icon={Settings}
          title="Настройки конфигов"
          text={selectedServer ? `Выбран сервер ${selectedServer.name}. В правой панели открыт таб Config для импорта AWG/Xray.` : "Выберите VPS в таблице, чтобы импортировать AWG/Xray."}
        />
        <div className="rounded-lg border border-white/10 bg-white/[0.035] p-4">
          <div className="mb-3 flex items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold">Health before config</div>
              <div className="text-xs text-zinc-500">Перед импортом проверьте heartbeat и snapshot.</div>
            </div>
            {selectedServer ? (
              <Button size="sm" variant="secondary" onClick={() => onRefreshHealth(selectedServer.id)} disabled={busyAction === `health-${selectedServer.id}`}>
                <RefreshCw size={14} /> Health
              </Button>
            ) : null}
          </div>
          <MiniMeter value={selectedServer?.utilization ?? 0} />
        </div>
      </div>
    );
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[1fr_1fr_1fr]">
      <div className="rounded-lg border border-amber-300/25 bg-[radial-gradient(circle_at_top_left,rgba(250,204,21,0.16),transparent_42%),rgba(255,255,255,0.035)] p-5">
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="text-xs uppercase tracking-wide text-amber-200/70">Fleet health</div>
            <div className="mt-2 text-4xl font-black text-zinc-50">{fleetHealth}%</div>
            <p className="mt-2 text-sm text-zinc-400">Сводный индекс по heartbeat, Docker, активности VPS и открытым инцидентам.</p>
          </div>
          <Gauge className="text-amber-200" size={28} />
        </div>
        <div className="mt-5"><MiniMeter value={fleetHealth} /></div>
      </div>
      <StatusCard icon={ShieldAlert} label="Инциденты" value={String(notifications.filter((item) => item.status === "open").length)} detail="Открытые уведомления" danger={notifications.some((item) => item.severity === "critical" && item.status === "open")} onClick={() => onSelectSection("incidents")} />
      <StatusCard icon={Server} label="VPS под контролем" value={String(servers.length)} detail={`${staleServers.length} stale heartbeat`} danger={staleServers.length > 0} onClick={() => onSelectSection("servers")} />
    </div>
  );
}

function PanelHeader({ icon: Icon, title, text }: { icon: LucideIcon; title: string; text: string }) {
  return (
    <div className="rounded-lg border border-white/10 bg-white/[0.035] p-5">
      <Icon className="mb-4 text-amber-200" size={22} />
      <h2 className="text-lg font-bold">{title}</h2>
      <p className="mt-2 text-sm leading-6 text-zinc-400">{text}</p>
    </div>
  );
}

function StatusCard({ icon: Icon, label, value, detail, danger = false, onClick }: { icon: LucideIcon; label: string; value: string; detail: string; danger?: boolean; onClick?: () => void }) {
  const Comp = onClick ? "button" : "div";
  return (
    <Comp
      type={onClick ? "button" : undefined}
      onClick={onClick}
      className={`rounded-lg border p-5 text-left transition ${danger ? "border-red-400/35 bg-red-500/10" : "border-white/10 bg-white/[0.035]"} ${onClick ? "min-h-32 w-full cursor-pointer hover:border-amber-300/35 hover:bg-amber-300/[0.06]" : ""}`}
    >
      <Icon className={danger ? "mb-4 text-red-200" : "mb-4 text-amber-200"} size={22} />
      <div className="text-xs uppercase tracking-wide text-zinc-500">{label}</div>
      <div className="mt-1 text-3xl font-black text-zinc-50">{value}</div>
      <div className="mt-1 text-sm text-zinc-500">{detail}</div>
    </Comp>
  );
}

function UsersManager({ users, loading, onLoad, onExport, onAction, onConfirm }: { users: LegacyAdminUser[]; loading: boolean; onLoad: () => Promise<void>; onExport: () => Promise<void>; onAction: (label: string, url: string, body?: unknown, method?: string) => Promise<boolean>; onConfirm: (state: ConfirmDialogState) => void }) {
  const [query, setQuery] = useState("");
  const [role, setRole] = useState("");
  const [status, setStatus] = useState("");
  const visible = users.filter((user) => {
    const subscription = user.subscription ?? {};
    if (query && !user.email.toLowerCase().includes(query.toLowerCase())) return false;
    if (role && user.role !== role) return false;
    if (status && (subscription.status ?? "none") !== status && !(status === "none" && !subscription.status)) return false;
    return true;
  });

  async function userAction(action: string, user: LegacyAdminUser, plan?: string) {
    const labels: Record<string, string> = {
      upgrade: `Выдать тариф ${planLabel(plan)} пользователю ${user.email}?`,
      revokeSubscription: `Снять подписку и отозвать ключи у ${user.email}?`,
      revokeKeys: `Отозвать все VPN-ключи ${user.email}?`,
      role: `Сменить роль ${user.email} на ${user.role === "admin" ? "user" : "admin"}?`,
      delete: `Удалить пользователя ${user.email} и связанные данные?`,
    };
    onConfirm({
      title: "Действие с пользователем",
      message: labels[action],
      actionLabel: action === "delete" ? "Удалить" : "Выполнить",
      danger: action === "delete" || action === "revokeSubscription" || action === "revokeKeys",
      onConfirm: async () => {
        if (action === "upgrade") await onAction(`user-upgrade-${user.id}`, `/api/admin/users/${user.id}/upgrade`, { plan });
        if (action === "revokeSubscription") await onAction(`user-revoke-sub-${user.id}`, `/api/admin/users/${user.id}/subscription/revoke`, { revoke_keys: true });
        if (action === "revokeKeys") await onAction(`user-revoke-keys-${user.id}`, `/api/admin/users/${user.id}/revoke`);
        if (action === "role") await onAction(`user-role-${user.id}`, `/api/admin/users/${user.id}/set-role`, { role: user.role === "admin" ? "user" : "admin" });
        if (action === "delete") await onAction(`user-delete-${user.id}`, `/api/admin/users/${user.id}`, undefined, "DELETE");
        await onLoad();
      },
    });
  }

  return (
    <section className="rounded-lg border border-white/10 bg-zinc-950/70">
      <LegacyToolbar title="Пользователи" loading={loading} onLoad={onLoad} onExport={onExport}>
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск email" className="legacy-input" />
        <select value={role} onChange={(event) => setRole(event.target.value)} className="legacy-select"><option value="">Все роли</option><option value="admin">Администратор</option><option value="user">Пользователь</option></select>
        <select value={status} onChange={(event) => setStatus(event.target.value)} className="legacy-select"><option value="">Все статусы</option><option value="active">Активные</option><option value="expired">Истекшие</option><option value="cancelled">Отменённые</option><option value="none">Без подписки</option></select>
      </LegacyToolbar>
      <LegacyTable empty="Пользователи не загружены">
        <thead><tr><LegacyTh>ID</LegacyTh><LegacyTh>Email</LegacyTh><LegacyTh>Роль</LegacyTh><LegacyTh>Тариф</LegacyTh><LegacyTh>Статус</LegacyTh><LegacyTh>Истекает</LegacyTh><LegacyTh>Действия</LegacyTh></tr></thead>
        <tbody>{visible.map((user) => (
          <tr key={user.id}>
            <LegacyTd>{user.id}</LegacyTd>
            <LegacyTd>{user.email}</LegacyTd>
            <LegacyTd><BadgeText value={user.role} /></LegacyTd>
            <LegacyTd>{planLabel(user.subscription?.plan)}</LegacyTd>
            <LegacyTd>{user.subscription?.status ?? "none"}</LegacyTd>
            <LegacyTd>{formatDate(user.subscription?.expires_at)}</LegacyTd>
            <LegacyTd>
              <div className="flex flex-wrap gap-2">
                <Button size="sm" variant="secondary" onClick={() => userAction("upgrade", user, "basic")}>Premium</Button>
                <Button size="sm" variant="secondary" onClick={() => userAction("upgrade", user, "vip")}>VIP</Button>
                <Button size="sm" variant="ghost" onClick={() => userAction("revokeSubscription", user)}>Снять</Button>
                <Button size="sm" variant="ghost" onClick={() => userAction("revokeKeys", user)}>Ключи</Button>
                <Button size="sm" variant="ghost" onClick={() => userAction("role", user)}>Роль</Button>
                <Button size="sm" variant="destructive" onClick={() => userAction("delete", user)}>Удалить</Button>
              </div>
            </LegacyTd>
          </tr>
        ))}
        {visible.length === 0 ? <tr><LegacyTd colSpan={7}>Пользователи не найдены</LegacyTd></tr> : null}</tbody>
      </LegacyTable>
    </section>
  );
}

function PaymentsManager({ payments, loading, onLoad, onExport, onAction, onConfirm }: { payments: LegacyAdminPayment[]; loading: boolean; onLoad: () => Promise<void>; onExport: () => Promise<void>; onAction: (label: string, url: string, body?: unknown, method?: string) => Promise<boolean>; onConfirm: (state: ConfirmDialogState) => void }) {
  const [query, setQuery] = useState("");
  const [plan, setPlan] = useState("");
  const [status, setStatus] = useState("");
  const visible = payments.filter((payment) => {
    if (query && !(payment.user_email ?? "").toLowerCase().includes(query.toLowerCase())) return false;
    if (plan && payment.plan !== plan) return false;
    if (status && payment.status !== status) return false;
    return true;
  });
  async function approve(payment: LegacyAdminPayment) {
    onConfirm({
      title: "Подтвердить платёж",
      message: `Подтвердить платёж #${payment.id} вручную и выдать подписку?`,
      actionLabel: "Подтвердить",
      onConfirm: async () => {
        await onAction(`payment-approve-${payment.id}`, `/api/admin/payments/${payment.id}/approve`);
        await onLoad();
      },
    });
  }
  return (
    <section className="rounded-lg border border-white/10 bg-zinc-950/70">
      <LegacyToolbar title="Платежи" loading={loading} onLoad={onLoad} onExport={onExport}>
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Поиск email" className="legacy-input" />
        <select value={plan} onChange={(event) => setPlan(event.target.value)} className="legacy-select"><option value="">Все планы</option><option value="trial">Trial</option><option value="basic">Premium</option><option value="vip">VIP</option><option value="free">Free</option></select>
        <select value={status} onChange={(event) => setStatus(event.target.value)} className="legacy-select"><option value="">Все статусы</option><option value="pending">Ожидают</option><option value="succeeded">Успешные</option><option value="failed">Неуспешные</option></select>
      </LegacyToolbar>
      <LegacyTable empty="Платежи не загружены">
        <thead><tr><LegacyTh>ID</LegacyTh><LegacyTh>Email</LegacyTh><LegacyTh>Сумма</LegacyTh><LegacyTh>Скидка</LegacyTh><LegacyTh>Промо</LegacyTh><LegacyTh>План</LegacyTh><LegacyTh>Статус</LegacyTh><LegacyTh>Создан</LegacyTh><LegacyTh>Действия</LegacyTh></tr></thead>
        <tbody>{visible.map((payment) => (
          <tr key={payment.id}>
            <LegacyTd>{payment.id}</LegacyTd>
            <LegacyTd>{payment.user_email ?? "—"}</LegacyTd>
            <LegacyTd>{formatMoney(payment.amount)}</LegacyTd>
            <LegacyTd>{formatMoney(payment.discount_amount ?? 0)}</LegacyTd>
            <LegacyTd>{payment.promo_code || "—"}</LegacyTd>
            <LegacyTd>{planLabel(payment.plan)}</LegacyTd>
            <LegacyTd><BadgeText value={payment.status} /></LegacyTd>
            <LegacyTd>{formatDate(payment.created_at)}</LegacyTd>
            <LegacyTd>{payment.status === "pending" ? <Button size="sm" onClick={() => approve(payment)}>Подтвердить</Button> : null}</LegacyTd>
          </tr>
        ))}
        {visible.length === 0 ? <tr><LegacyTd colSpan={9}>Платежи не найдены</LegacyTd></tr> : null}</tbody>
      </LegacyTable>
    </section>
  );
}

function PromoCodesManager({ promoCodes, loading, busyAction, onLoad, onExport, onAction, onConfirm }: { promoCodes: LegacyAdminPromoCode[]; loading: boolean; busyAction: string | null; onLoad: () => Promise<void>; onExport: () => Promise<void>; onAction: (label: string, url: string, body?: unknown, method?: string) => Promise<boolean>; onConfirm: (state: ConfirmDialogState) => void }) {
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("");
  const [editing, setEditing] = useState<LegacyAdminPromoCode | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [draft, setDraft] = useState({ code: "", description: "", discount_percent: 10, max_uses: 0, applicable_plans: "all", once_per_user: true, active: true, expires_at: "" });
  const visible = promoCodes.filter((promo) => {
    const haystack = `${promo.code} ${promo.description ?? ""}`.toLowerCase();
    if (query && !haystack.includes(query.toLowerCase())) return false;
    if (status === "active" && !promo.active) return false;
    if (status === "inactive" && promo.active) return false;
    return true;
  });
  function open(promo?: LegacyAdminPromoCode) {
    setEditing(promo ?? null);
    setFormOpen(true);
    setDraft({
      code: promo?.code ?? "",
      description: promo?.description ?? "",
      discount_percent: promo?.discount_percent ?? 10,
      max_uses: promo?.max_uses ?? 0,
      applicable_plans: promo?.applicable_plans ?? "all",
      once_per_user: promo?.once_per_user ?? true,
      active: promo?.active ?? true,
      expires_at: toDatetimeLocal(promo?.expires_at),
    });
  }
  async function save() {
    if (!draft.code.trim() || draft.discount_percent < 1 || draft.discount_percent > 100) return;
    const payload = { ...draft, expires_at: draft.expires_at ? new Date(draft.expires_at).toISOString() : null };
    const ok = await onAction(`promo-save-${editing?.id ?? "new"}`, editing ? `/api/admin/promo-codes/${editing.id}` : "/api/admin/promo-codes", payload, editing ? "PUT" : "POST");
    if (ok) {
      setEditing(null);
      setFormOpen(false);
      await onLoad();
    }
  }
  async function remove(promo: LegacyAdminPromoCode) {
    onConfirm({
      title: "Отключить промокод",
      message: `Отключить промокод ${promo.code}? Уже созданные платежи сохранят историю скидки.`,
      actionLabel: "Отключить",
      danger: true,
      onConfirm: async () => {
        await onAction(`promo-delete-${promo.id}`, `/api/admin/promo-codes/${promo.id}`, undefined, "DELETE");
        await onLoad();
      },
    });
  }
  return (
    <section className="rounded-lg border border-white/10 bg-zinc-950/70">
      <LegacyToolbar title="Промокоды" loading={loading} onLoad={onLoad} onExport={onExport}>
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Код или описание" className="legacy-input" />
        <select value={status} onChange={(event) => setStatus(event.target.value)} className="legacy-select"><option value="">Все статусы</option><option value="active">Активные</option><option value="inactive">Отключённые</option></select>
        <Button size="sm" onClick={() => open()}>Добавить</Button>
      </LegacyToolbar>
      {formOpen ? (
        <div className="grid gap-2 border-b border-white/10 p-4 md:grid-cols-4">
          <input className="legacy-input" placeholder="Код" value={draft.code} onChange={(e) => setDraft((d) => ({ ...d, code: e.target.value.toUpperCase() }))} />
          <input className="legacy-input" placeholder="Описание" value={draft.description} onChange={(e) => setDraft((d) => ({ ...d, description: e.target.value }))} />
          <input className="legacy-input" type="number" min={1} max={100} value={draft.discount_percent} onChange={(e) => setDraft((d) => ({ ...d, discount_percent: Number(e.target.value) }))} />
          <input className="legacy-input" type="number" min={0} value={draft.max_uses} onChange={(e) => setDraft((d) => ({ ...d, max_uses: Number(e.target.value) }))} />
          <input className="legacy-input" placeholder="Тарифы: all/basic/vip" value={draft.applicable_plans} onChange={(e) => setDraft((d) => ({ ...d, applicable_plans: e.target.value }))} />
          <input className="legacy-input" type="datetime-local" value={draft.expires_at} onChange={(e) => setDraft((d) => ({ ...d, expires_at: e.target.value }))} />
          <label className="flex items-center gap-2 text-sm text-zinc-300"><input type="checkbox" checked={draft.once_per_user} onChange={(e) => setDraft((d) => ({ ...d, once_per_user: e.target.checked }))} /> 1 раз на пользователя</label>
          <div className="flex gap-2"><Button size="sm" onClick={save} disabled={busyAction?.startsWith("promo-save")}>Сохранить</Button><Button size="sm" variant="ghost" onClick={() => { setEditing(null); setFormOpen(false); setDraft((d) => ({ ...d, code: "" })); }}>Отмена</Button></div>
        </div>
      ) : null}
      <LegacyTable empty="Промокоды не загружены">
        <thead><tr><LegacyTh>Код</LegacyTh><LegacyTh>Скидка</LegacyTh><LegacyTh>Использовано</LegacyTh><LegacyTh>Тарифы</LegacyTh><LegacyTh>Истекает</LegacyTh><LegacyTh>Статус</LegacyTh><LegacyTh>Действия</LegacyTh></tr></thead>
        <tbody>{visible.map((promo) => (
          <tr key={promo.id}>
            <LegacyTd>{promo.code}</LegacyTd>
            <LegacyTd>{promo.discount_percent}%</LegacyTd>
            <LegacyTd>{promo.used_count ?? 0}/{promo.max_uses || "∞"}</LegacyTd>
            <LegacyTd>{promo.applicable_plans || "all"}</LegacyTd>
            <LegacyTd>{formatDate(promo.expires_at)}</LegacyTd>
            <LegacyTd><BadgeText value={promo.active ? "active" : "inactive"} /></LegacyTd>
            <LegacyTd><div className="flex gap-2"><Button size="sm" variant="secondary" onClick={() => open(promo)}>Ред.</Button><Button size="sm" variant="destructive" onClick={() => remove(promo)}>Откл.</Button></div></LegacyTd>
          </tr>
        ))}
        {visible.length === 0 ? <tr><LegacyTd colSpan={7}>Промокоды не найдены</LegacyTd></tr> : null}</tbody>
      </LegacyTable>
    </section>
  );
}

function DownloadsManager({ downloads, loading, busyAction, onLoad, onUpload }: { downloads: LegacyAdminDownload[]; loading: boolean; busyAction: string | null; onLoad: () => Promise<void>; onUpload: (platform: string, file: File) => Promise<void> }) {
  const [files, setFiles] = useState<Record<string, File | null>>({});
  return (
    <section className="rounded-lg border border-white/10 bg-zinc-950/70">
      <LegacyToolbar title="Приложения" loading={loading} onLoad={onLoad} />
      <LegacyTable empty="Файлы приложений не загружены">
        <thead><tr><LegacyTh>Платформа</LegacyTh><LegacyTh>Текущий файл</LegacyTh><LegacyTh>Разрешено</LegacyTh><LegacyTh>Публичная ссылка</LegacyTh><LegacyTh>Загрузка</LegacyTh></tr></thead>
        <tbody>{downloads.map((item) => {
          const file = files[item.platform] ?? null;
          return (
            <tr key={item.platform}>
              <LegacyTd>{item.platform}</LegacyTd>
              <LegacyTd>{item.current_file || item.file_name || "—"}<div className="text-xs text-zinc-600">{fmtBytes(item.size_bytes)}</div></LegacyTd>
              <LegacyTd>{(item.allowed_extensions ?? []).join(", ") || "—"}</LegacyTd>
              <LegacyTd>{item.public_url || item.url ? <a className="text-amber-200 hover:underline" href={item.public_url || item.url} target="_blank">Открыть</a> : "—"}</LegacyTd>
              <LegacyTd>
                <div className="flex min-w-[260px] flex-wrap gap-2">
                  <input type="file" className="max-w-[170px] text-xs text-zinc-400" onChange={(event) => setFiles((current) => ({ ...current, [item.platform]: event.target.files?.[0] ?? null }))} />
                  <Button size="sm" disabled={!file || busyAction === `download-${item.platform}`} onClick={() => file && onUpload(item.platform, file)}>Загрузить</Button>
                </div>
              </LegacyTd>
            </tr>
          );
        })}
        {downloads.length === 0 ? <tr><LegacyTd colSpan={5}>Файлы приложений не найдены</LegacyTd></tr> : null}</tbody>
      </LegacyTable>
    </section>
  );
}

function LegacyToolbar({ title, loading, onLoad, onExport, children }: { title: string; loading: boolean; onLoad: () => Promise<void>; onExport?: () => Promise<void>; children?: React.ReactNode }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/10 p-4">
      <div>
        <h2 className="font-semibold">{title}</h2>
        <p className="text-xs text-zinc-500">Функции перенесены из legacy admin/index.html</p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        {children}
        <Button size="sm" variant="secondary" onClick={onLoad} disabled={loading}><RefreshCw size={14} /> {loading ? "Загрузка" : "Обновить"}</Button>
        {onExport ? <Button size="sm" onClick={onExport}>CSV</Button> : null}
      </div>
    </div>
  );
}

function LegacyTable({ children, empty }: { children: React.ReactNode; empty: string }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[920px] text-left text-sm">
        {children}
      </table>
      <div className="sr-only">{empty}</div>
    </div>
  );
}

function LegacyTh({ children }: { children: React.ReactNode }) {
  return <th className="border-b border-white/10 px-4 py-3 text-xs font-medium uppercase text-zinc-500">{children}</th>;
}

function LegacyTd({ children, colSpan }: { children: React.ReactNode; colSpan?: number }) {
  return <td colSpan={colSpan} className="border-b border-white/[0.06] px-4 py-3 align-top text-zinc-300">{children}</td>;
}

function BadgeText({ value }: { value: string }) {
  const positive = ["active", "succeeded", "admin", "ok"].includes(value);
  const danger = ["failed", "expired", "cancelled", "inactive"].includes(value);
  return <span className={`inline-flex rounded-full border px-2 py-1 text-xs ${positive ? "border-emerald-400/30 bg-emerald-500/10 text-emerald-100" : danger ? "border-red-400/30 bg-red-500/10 text-red-100" : "border-white/10 bg-white/[0.04] text-zinc-300"}`}>{value}</span>;
}

function FilterSelect<T extends string>({ label, value, onChange, options }: { label: string; value: T; onChange: (value: T) => void; options: Array<[T, string]> }) {
  return (
    <label>
      <span className="sr-only">{label}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value as T)}
        className="h-9 rounded-lg border border-white/10 bg-zinc-950 px-3 text-sm text-zinc-200 outline-none transition focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20"
      >
        {options.map(([optionValue, optionLabel]) => (
          <option key={optionValue} value={optionValue}>{optionLabel}</option>
        ))}
      </select>
    </label>
  );
}

function MiniMeter({ value }: { value: number }) {
  const normalized = Math.max(0, Math.min(100, Math.round(value)));
  const color = normalized > 85 ? "bg-red-400" : normalized > 70 ? "bg-amber-300" : "bg-emerald-400";
  return (
    <div className="flex min-w-[120px] items-center gap-2">
      <span className="h-2 flex-1 overflow-hidden rounded-full bg-white/10">
        <span className={`block h-full rounded-full ${color}`} style={{ width: `${normalized}%` }} />
      </span>
      <span className="w-10 text-right font-mono text-xs text-zinc-400">{normalized}%</span>
    </div>
  );
}

function StatusBadge({ server }: { server: AdminServer }) {
  const stale = server.agent_heartbeat_stale;
  const className = stale
    ? "border-red-400/40 bg-red-500/10 text-red-100"
    : server.active
      ? "border-emerald-400/35 bg-emerald-500/10 text-emerald-100"
      : "border-zinc-500/40 bg-zinc-500/10 text-zinc-300";
  return <span className={`inline-flex rounded-full border px-2.5 py-1 text-xs font-semibold ${className}`}>{stale ? "Stale" : server.active ? "Online" : "Off"}</span>;
}

function IconButton({ label, icon: Icon, onClick, active = false }: { label: string; icon: LucideIcon; onClick: () => void; active?: boolean }) {
  return (
    <button title={label} aria-label={label} onClick={onClick} disabled={active} className="rounded-lg border border-white/10 bg-white/[0.035] p-2 text-zinc-300 transition hover:border-amber-300/40 hover:text-amber-100 disabled:opacity-60">
      <Icon size={15} className={active ? "animate-spin" : ""} />
    </button>
  );
}

function NotificationInbox({ notifications, onAck, onMute }: { notifications: AdminNotification[]; onAck: (id: number) => void; onMute: (id: number) => void }) {
  return (
    <section className="rounded-lg border border-white/10 bg-zinc-950/70">
      <div className="flex items-center justify-between border-b border-white/10 p-4">
        <h2 className="font-semibold">Уведомления</h2>
        <span className="rounded-full bg-red-500/15 px-2 py-1 text-xs text-red-100">{notifications.filter((n) => n.status === "open").length} open</span>
      </div>
      <div className="max-h-[360px] space-y-3 overflow-auto p-4">
        {notifications.length === 0 ? <p className="text-sm text-zinc-500">Инцидентов нет.</p> : null}
        {notifications.map((notification) => (
          <article key={notification.id} className="rounded-lg border border-white/10 bg-white/[0.03] p-3">
            <div className="mb-2 flex items-start justify-between gap-2">
              <div>
                <div className="text-sm font-semibold">{notification.title}</div>
                <div className="text-xs text-zinc-500">{notification.server?.name ?? "Fleet"} · {formatRelative(notification.created_at)}</div>
              </div>
              <span className={`rounded-full px-2 py-1 text-[11px] font-bold ${notification.severity === "critical" ? "bg-red-500/15 text-red-100" : "bg-amber-300/15 text-amber-100"}`}>{notification.severity}</span>
            </div>
            <p className="text-sm text-zinc-400">{notification.message}</p>
            <div className="mt-3 flex gap-2">
              <Button size="sm" variant="secondary" onClick={() => onAck(notification.id)}>Принять</Button>
              <Button size="sm" variant="ghost" onClick={() => onMute(notification.id)}>Mute 30м</Button>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}

function ServerDrawer({ server, tab, setTab, auditLogs, busyAction, onAction, onRefreshHealth, onOpenDigest, onOpenBootstrap, onConfirm }: { server: AdminServer; tab: DrawerTab; setTab: (tab: DrawerTab) => void; auditLogs: AdminAuditLog[]; busyAction: string | null; onAction: (label: string, url: string, body?: unknown, method?: string) => Promise<boolean>; onRefreshHealth: (serverID: number) => Promise<void>; onOpenDigest: (state: DigestDialogState) => void; onOpenBootstrap: (server: AdminServer) => void; onConfirm: (state: ConfirmDialogState) => void }) {
  return (
    <section className="rounded-lg border border-white/10 bg-zinc-950/70">
      <div className="border-b border-white/10 p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-lg font-bold">{server.name}</h2>
            <p className="text-xs text-zinc-500">{server.endpoint || server.host}</p>
          </div>
          <StatusBadge server={server} />
        </div>
        <div className="mt-4 grid grid-cols-3 gap-1 sm:grid-cols-6">
          {drawerTabs.map((item) => (
            <button key={item} onClick={() => setTab(item)} className={`rounded-md px-2 py-1.5 text-xs ${tab === item ? "bg-amber-300 text-black" : "bg-white/[0.035] text-zinc-400 hover:text-white"}`}>
              {item}
            </button>
          ))}
        </div>
      </div>
      <div className="p-4">
        {tab === "Health" ? (
          <div className="space-y-4">
            <HealthSignalGrid server={server} />
            <dl className="space-y-3 text-sm">
              <InfoRow label="Heartbeat" value={formatRelative(server.agent_last_heartbeat_at)} />
              <InfoRow label="Docker" value={server.agent_docker_available ? "available" : "unavailable"} />
              <InfoRow label="Uptime" value={formatDuration(server.agent_uptime_seconds ?? 0)} />
              <InfoRow label="Pi-hole" value={server.pihole_last_sync_error || (server.pihole_enabled ? "enabled" : "disabled")} />
              <Button size="sm" variant="secondary" onClick={() => onRefreshHealth(server.id)} disabled={busyAction === `health-${server.id}`}><RefreshCw size={14} /> Refresh health</Button>
            </dl>
          </div>
        ) : null}
        {tab === "Agent" ? (
          <dl className="space-y-3 text-sm">
            <InfoRow label="Version" value={server.agent_last_version || "—"} />
            <InfoRow label="Commit" value={server.agent_last_commit || "—"} />
            <InfoRow label="Active digest" value={server.agent_active_digest || "—"} mono />
            <InfoRow label="Previous digest" value={server.agent_previous_digest || "—"} mono />
            <InfoRow label="Update" value={server.agent_last_update_error || server.agent_last_update_status || "—"} />
          </dl>
        ) : null}
        {tab === "Snapshot" ? (
          <dl className="space-y-3 text-sm">
            <InfoRow label="Hash" value={server.agent_last_snapshot_hash || "—"} mono />
            <InfoRow label="Snapshot at" value={formatRelative(server.agent_last_snapshot_at)} />
            <InfoRow label="Status" value={server.agent_last_snapshot_status || "—"} />
            <Button size="sm" onClick={() => onAction(`snapshot-${server.id}`, `/api/admin/servers/${server.id}/agent/snapshot`)} disabled={busyAction === `snapshot-${server.id}`}><DatabaseBackup size={14} /> Import snapshot</Button>
          </dl>
        ) : null}
        {tab === "Config" ? (
          <ConfigDirectEditPanel server={server} busyAction={busyAction} onAction={onAction} />
        ) : null}
        {tab === "Actions" ? (
          <div className="space-y-3">
            <Button className="w-full justify-start" onClick={() => onOpenBootstrap(server)} disabled={busyAction === `bootstrap-${server.id}`}><Terminal size={14} /> Bootstrap agent</Button>
            <Button className="w-full justify-start" variant="secondary" onClick={() => onAction(`status-${server.id}`, `/api/admin/servers/${server.id}/agent/status`, undefined, "GET")} disabled={busyAction === `status-${server.id}`}><RefreshCw size={14} /> Agent status</Button>
            <Button className="w-full justify-start" variant="secondary" onClick={() => onOpenDigest({ server, image_digest: "" })}><Zap size={14} /> Update by digest</Button>
            <Button className="w-full justify-start" variant="secondary" onClick={() => onConfirm({ title: "Rollback agent", message: `Откатить agent image на ${server.name}?`, actionLabel: "Rollback", onConfirm: () => onAction(`rollback-${server.id}`, `/api/admin/servers/${server.id}/agent/rollback`) })}><RotateCcw size={14} /> Rollback</Button>
            <Button className="w-full justify-start" variant="destructive" onClick={() => onConfirm({ title: "Toggle active", message: `Переключить active state для ${server.name}?`, actionLabel: "Toggle", danger: true, onConfirm: () => onAction(`toggle-${server.id}`, `/api/admin/servers/${server.id}/toggle`) })}><AlertTriangle size={14} /> Toggle active</Button>
          </div>
        ) : null}
        {tab === "Audit" ? (
          <div className="space-y-3">
            {auditLogs.length === 0 ? <p className="text-sm text-zinc-500">Событий пока нет.</p> : null}
            {auditLogs.map((log) => (
              <div key={log.id} className="rounded-lg border border-white/10 bg-white/[0.03] p-3 text-xs">
                <div className="flex items-center gap-2 text-zinc-200"><History size={13} /> {log.action} · {log.result}</div>
                <div className="mt-1 text-zinc-500">{formatRelative(log.created_at)} {log.message ? `· ${log.message}` : ""}</div>
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </section>
  );
}

function ConfigDirectEditPanel({ server, busyAction, onAction }: { server: AdminServer; busyAction: string | null; onAction: (label: string, url: string, body?: unknown, method?: string) => Promise<boolean> }) {
  const [awgConfig, setAwgConfig] = useState(() => seedAWGConfig(server));
  const xray = server.config_summary?.xray;
  
  const [protocol, setProtocol] = useState(xray?.hysteria_enabled && (!xray?.network || xray?.network === "") ? "hysteria2" : "vless");

  const [address, setAddress] = useState(xray?.address ?? "");
  const [port, setPort] = useState(String(xray?.port ?? 443));
  const [serverName, setServerName] = useState(xray?.server_name ?? "");
  const [clientId, setClientId] = useState(xray?.client_id ?? "");
  
  const [network, setNetwork] = useState(xray?.network ?? "xhttp");
  const [security, setSecurity] = useState(xray?.security ?? "reality");
  const [flow, setFlow] = useState(xray?.flow ?? "");
  
  const [publicKey, setPublicKey] = useState(xray?.public_key ?? "");
  const [shortId, setShortId] = useState(xray?.short_id ?? "");
  const [spiderX, setSpiderX] = useState(xray?.spider_x ?? "/");
  const [fingerprint, setFingerprint] = useState(xray?.fingerprint ?? "chrome");
  const [mldsa65Verify, setMldsa65Verify] = useState(xray?.mldsa65_verify ?? "");
  
  const [xhttpPath, setXhttpPath] = useState(xray?.x_http_path ?? "");
  const [xhttpHost, setXhttpHost] = useState(xray?.x_http_host ?? "");
  const [xhttpMode, setXhttpMode] = useState(xray?.x_http_mode ?? "auto");
  const [xhttpPadding, setXhttpPadding] = useState(xray?.x_http_padding ?? "");
  const [xhttpPostSize, setXhttpPostSize] = useState(String(xray?.x_http_post_size ?? 0));
  
  const [grpcServiceName, setGrpcServiceName] = useState(xray?.grpc_service_name ?? "");
  const [grpcAuthority, setGrpcAuthority] = useState(xray?.grpc_authority ?? "");
  const [grpcMultiMode, setGrpcMultiMode] = useState(xray?.grpc_multi_mode ?? true);

  const [hysteriaPort, setHysteriaPort] = useState(String(xray?.hysteria_port ?? 443));
  const [hysteriaPassword, setHysteriaPassword] = useState(xray?.hysteria_password ?? "");
  const [hysteriaSni, setHysteriaSni] = useState(xray?.hysteria_sni ?? "");
  const [hysteriaInsecure, setHysteriaInsecure] = useState(xray?.hysteria_insecure ?? true);
  const [hysteriaObfsPassword, setHysteriaObfsPassword] = useState(xray?.hysteria_obfs_password ?? "");
  const [hysteriaMasqueradeUrl, setHysteriaMasqueradeUrl] = useState(xray?.hysteria_masquerade_url ?? "https://www.microsoft.com");

  const busy = busyAction === `config-update-${server.id}`;

  useEffect(() => {
    setAwgConfig(seedAWGConfig(server));
    const x = server.config_summary?.xray;
    setProtocol(x?.hysteria_enabled && (!x?.network || x?.network === "") ? "hysteria2" : "vless");
    setAddress(x?.address ?? "");
    setPort(String(x?.port ?? 443));
    setServerName(x?.server_name ?? "");
    setClientId(x?.client_id ?? "");
    setNetwork(x?.network ?? "xhttp");
    setSecurity(x?.security ?? "reality");
    setFlow(x?.flow ?? "");
    setPublicKey(x?.public_key ?? "");
    setShortId(x?.short_id ?? "");
    setSpiderX(x?.spider_x ?? "/");
    setFingerprint(x?.fingerprint ?? "chrome");
    setMldsa65Verify(x?.mldsa65_verify ?? "");
    setXhttpPath(x?.x_http_path ?? "");
    setXhttpHost(x?.x_http_host ?? "");
    setXhttpMode(x?.x_http_mode ?? "auto");
    setXhttpPadding(x?.x_http_padding ?? "");
    setXhttpPostSize(String(x?.x_http_post_size ?? 0));
    setGrpcServiceName(x?.grpc_service_name ?? "");
    setGrpcAuthority(x?.grpc_authority ?? "");
    setGrpcMultiMode(x?.grpc_multi_mode ?? true);
    setHysteriaPort(String(x?.hysteria_port ?? 443));
    setHysteriaPassword(x?.hysteria_password ?? "");
    setHysteriaSni(x?.hysteria_sni ?? "");
    setHysteriaInsecure(x?.hysteria_insecure ?? true);
    setHysteriaObfsPassword(x?.hysteria_obfs_password ?? "");
    setHysteriaMasqueradeUrl(x?.hysteria_masquerade_url ?? "https://www.microsoft.com");
  }, [server.id, server.config_summary]);

  return (
    <div className="space-y-4">
      <label className="block">
        <span className="mb-1 block text-xs font-semibold text-zinc-400">AWG config</span>
        <textarea
          value={awgConfig}
          onChange={(event) => setAwgConfig(event.target.value)}
          className="min-h-36 w-full resize-y rounded-lg border border-white/10 bg-black/35 p-3 font-mono text-xs text-zinc-100 outline-none focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20"
          spellCheck={false}
        />
      </label>

      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
        <div className="mb-3 text-sm font-semibold text-zinc-300">Тип подключения (Протокол)</div>
        <div className="flex gap-4">
          <label className="flex items-center space-x-2">
            <input type="radio" checked={protocol === "vless"} onChange={() => setProtocol("vless")} className="text-amber-400 focus:ring-amber-400/50 border-white/10 bg-black/35" />
            <span className="text-sm text-zinc-200">VLESS (Reality / TLS)</span>
          </label>
          <label className="flex items-center space-x-2">
            <input type="radio" checked={protocol === "hysteria2"} onChange={() => setProtocol("hysteria2")} className="text-indigo-400 focus:ring-indigo-400/50 border-white/10 bg-black/35" />
            <span className="text-sm text-zinc-200">Hysteria 2</span>
          </label>
        </div>
      </div>

      <div className="rounded-lg border border-white/10 bg-black/20 p-3">
        <div className="mb-3 text-sm font-semibold text-zinc-300">Базовые настройки сервера</div>
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          <ConfigInput label="Address (IP/Domain)" value={address} onChange={setAddress} />
          {protocol === "vless" && (
            <ConfigInput label="Server Name (SNI)" value={serverName} onChange={setServerName} />
          )}
          <ConfigInput label="Client ID (UUID)" value={clientId} onChange={setClientId} />
        </div>
      </div>

      {protocol === "vless" && (
        <>
          <div className="rounded-lg border border-white/10 bg-black/20 p-3">
            <div className="mb-3 text-sm font-semibold text-zinc-300">Транспорт и безопасность</div>
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              <label className="block">
                <span className="mb-1 block text-xs font-semibold text-zinc-500">Network</span>
                <select
                  value={network}
                  onChange={(e) => setNetwork(e.target.value)}
                  className="h-9 w-full rounded-lg border border-white/10 bg-black/35 px-2 font-mono text-xs text-zinc-100 outline-none focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20"
                >
                  <option value="tcp">tcp</option>
                  <option value="xhttp">xhttp</option>
                  <option value="grpc">grpc</option>
                  <option value="ws">ws</option>
                </select>
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-semibold text-zinc-500">Security</span>
                <select
                  value={security}
                  onChange={(e) => setSecurity(e.target.value)}
                  className="h-9 w-full rounded-lg border border-white/10 bg-black/35 px-2 font-mono text-xs text-zinc-100 outline-none focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20"
                >
                  <option value="reality">reality</option>
                  <option value="tls">tls</option>
                  <option value="none">none</option>
                </select>
              </label>
              <ConfigInput label="Port" value={port} onChange={setPort} />
              <ConfigInput label="Flow" value={flow} onChange={setFlow} placeholder="xtls-rprx-vision" />
            </div>
          </div>

          {security === "reality" && (
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="mb-3 text-sm font-semibold text-zinc-300">Reality</div>
              <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                <ConfigInput label="Public Key" value={publicKey} onChange={setPublicKey} />
                <ConfigInput label="Short ID" value={shortId} onChange={setShortId} />
                <ConfigInput label="Fingerprint" value={fingerprint} onChange={setFingerprint} />
                <ConfigInput label="Spider X" value={spiderX} onChange={setSpiderX} />
                <ConfigInput label="MLDSA65 Verify" value={mldsa65Verify} onChange={setMldsa65Verify} />
              </div>
            </div>
          )}

          {network === "xhttp" && (
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="mb-3 text-sm font-semibold text-zinc-300">XHTTP Settings</div>
              <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                <ConfigInput label="Path" value={xhttpPath} onChange={setXhttpPath} />
                <ConfigInput label="Host" value={xhttpHost} onChange={setXhttpHost} />
                <ConfigInput label="Mode" value={xhttpMode} onChange={setXhttpMode} placeholder="auto" />
                <ConfigInput label="Padding" value={xhttpPadding} onChange={setXhttpPadding} placeholder="100-1000" />
                <ConfigInput label="Post Size" value={xhttpPostSize} onChange={setXhttpPostSize} />
              </div>
            </div>
          )}

          {network === "grpc" && (
            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
              <div className="mb-3 text-sm font-semibold text-zinc-300">GRPC Settings</div>
              <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                <ConfigInput label="Service Name" value={grpcServiceName} onChange={setGrpcServiceName} />
                <ConfigInput label="Authority" value={grpcAuthority} onChange={setGrpcAuthority} />
                <label className="flex items-center space-x-2 pt-6">
                  <input type="checkbox" checked={grpcMultiMode} onChange={(e) => setGrpcMultiMode(e.target.checked)} className="rounded border-white/10 bg-black/35 text-amber-400" />
                  <span className="text-xs font-semibold text-zinc-300">Multi Mode</span>
                </label>
              </div>
            </div>
          )}
        </>
      )}

      {protocol === "hysteria2" && (
        <div className="rounded-lg border border-indigo-400/20 bg-indigo-500/5 p-3">
          <div className="mb-3 text-sm font-semibold text-indigo-300">Настройки Hysteria 2</div>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            <ConfigInput label="Port" value={hysteriaPort} onChange={setHysteriaPort} />
            <ConfigInput label="Password" value={hysteriaPassword} onChange={setHysteriaPassword} />
            <ConfigInput label="SNI" value={hysteriaSni} onChange={setHysteriaSni} />
            <ConfigInput label="Obfs Password" value={hysteriaObfsPassword} onChange={setHysteriaObfsPassword} />
            <ConfigInput label="Masquerade URL" value={hysteriaMasqueradeUrl} onChange={setHysteriaMasqueradeUrl} />
            <label className="flex items-center space-x-2 pt-6">
              <input type="checkbox" checked={hysteriaInsecure} onChange={(e) => setHysteriaInsecure(e.target.checked)} className="rounded border-white/10 bg-black/35 text-indigo-400" />
              <span className="text-xs font-semibold text-zinc-300">Allow Insecure</span>
            </label>
          </div>
        </div>
      )}

      <Button
        size="sm"
        className="w-full"
        disabled={busy}
        onClick={() => {
          onAction(`config-update-${server.id}`, `/api/admin/servers/${server.id}/vless-template`, {
            awg_config: awgConfig,
            client_id: clientId,
            address: address,
            port: parseInt(port) || 0,
            server_name: serverName,
            public_key: publicKey,
            short_id: shortId,
            fingerprint: fingerprint,
            flow: flow,
            network: protocol === "vless" ? network : "", // clear network if hysteria
            security: protocol === "vless" ? security : "",
            spider_x: spiderX,
            mldsa65_verify: mldsa65Verify,
            grpc_service_name: grpcServiceName,
            grpc_authority: grpcAuthority,
            grpc_multi_mode: grpcMultiMode,
            x_http_path: xhttpPath,
            x_http_host: xhttpHost,
            x_http_mode: xhttpMode,
            x_http_padding: xhttpPadding,
            x_http_post_size: parseInt(xhttpPostSize) || 0,
            hysteria_enabled: protocol === "hysteria2", // ONLY true if hysteria2 selected
            hysteria_port: parseInt(hysteriaPort) || 0,
            hysteria_password: hysteriaPassword,
            hysteria_sni: hysteriaSni,
            hysteria_insecure: hysteriaInsecure,
            hysteria_obfs_password: hysteriaObfsPassword,
            hysteria_masquerade_url: hysteriaMasqueradeUrl,
          }, "PUT");
        }}
      >
        <DatabaseBackup size={14} /> {busy ? "Сохранение..." : "Сохранить конфигурацию"}
      </Button>
    </div>
  );
}

function ConfigInput({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string }) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold text-zinc-500">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-9 w-full rounded-lg border border-white/10 bg-black/35 px-2 font-mono text-xs text-zinc-100 outline-none focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20"
      />
    </label>
  );
}

function HealthSignalGrid({ server }: { server: AdminServer }) {
  const signals = [
    {
      label: "Heartbeat",
      value: server.agent_heartbeat_stale ? "stale" : "fresh",
      ok: !server.agent_heartbeat_stale && Boolean(server.agent_last_heartbeat_at),
    },
    {
      label: "Docker",
      value: server.agent_docker_available ? "available" : "missing",
      ok: server.agent_docker_available === true,
    },
    {
      label: "Agent",
      value: server.agent_last_health_status || server.agent_mode || "manual",
      ok: server.agent_last_health_status === "ok" || server.agent_mode === "agent",
    },
    {
      label: "Pi-hole",
      value: server.pihole_last_sync_error ? "error" : server.pihole_enabled ? "enabled" : "off",
      ok: !server.pihole_last_sync_error,
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-2">
      {signals.map((signal) => (
        <div key={signal.label} className={`rounded-lg border p-3 ${signal.ok ? "border-emerald-400/25 bg-emerald-500/10" : "border-red-400/25 bg-red-500/10"}`}>
          <div className="text-[11px] uppercase tracking-wide text-zinc-500">{signal.label}</div>
          <div className={`mt-1 truncate text-sm font-semibold ${signal.ok ? "text-emerald-100" : "text-red-100"}`}>{signal.value}</div>
        </div>
      ))}
    </div>
  );
}

function seedAWGConfig(server: AdminServer) {
  const awg = server.config_summary?.awg;
  if (!awg) return "";
  return [
    `[Interface]`,
    `ListenPort = ${awg.listen_port ?? 51820}`,
    `Jc = ${awg.jc ?? ""}`,
    `Jmin = ${awg.jmin ?? ""}`,
    `Jmax = ${awg.jmax ?? ""}`,
    `S1 = ${awg.s1 ?? ""}`,
    `S2 = ${awg.s2 ?? ""}`,
    `S3 = ${awg.s3 ?? ""}`,
    `S4 = ${awg.s4 ?? ""}`,
    `H1 = ${awg.h1 ?? ""}`,
    `H2 = ${awg.h2 ?? ""}`,
    `H3 = ${awg.h3 ?? ""}`,
    `H4 = ${awg.h4 ?? ""}`,
  ].join("\n");
}

function InfoRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[96px_minmax(0,1fr)] gap-3">
      <dt className="text-zinc-500">{label}</dt>
      <dd className={`truncate text-zinc-200 ${mono ? "font-mono text-xs" : ""}`} title={value}>{value}</dd>
    </div>
  );
}

function planLabel(plan?: string | null) {
  return ({
    free: "Free",
    trial: "Пробный",
    basic: "Premium",
    basic_3m: "Premium 3м",
    vip: "VIP",
    vip_3m: "VIP 3м",
    none: "Нет",
  } as Record<string, string>)[String(plan || "none")] ?? String(plan || "Нет");
}

function formatMoney(value?: number | null) {
  const amount = Number(value || 0);
  return `${amount.toLocaleString("ru-RU", { minimumFractionDigits: amount % 1 ? 2 : 0, maximumFractionDigits: 2 })} ₽`;
}

function formatDate(value?: string | null) {
  if (!value) return "—";
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return "—";
  return new Date(timestamp).toLocaleString("ru-RU");
}

function toDatetimeLocal(value?: string | null) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const offset = date.getTimezoneOffset() * 60000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function fmtBytes(value?: number | null) {
  const bytes = Number(value || 0);
  if (!bytes) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let size = bytes;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  return `${size.toFixed(size >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function formatRelative(value?: string | null) {
  if (!value) return "never";
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return "—";
  const seconds = Math.max(0, Math.round((Date.now() - timestamp) / 1000));
  if (seconds < 60) return `${seconds}s назад`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}м назад`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}ч назад`;
  return new Date(timestamp).toLocaleString("ru-RU");
}

function formatDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0s";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}д ${hours}ч`;
  if (hours > 0) return `${hours}ч ${minutes}м`;
  return `${minutes}м`;
}
