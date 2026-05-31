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
  Radio,
  RefreshCw,
  RotateCcw,
  Search,
  Server,
  Settings,
  ShieldAlert,
  ShieldCheck,
  Terminal,
  UserRound,
  Users,
  WalletCards,
  X,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  type AdminAuditLog,
  type AdminNotification,
  type AdminOverview,
  type AdminServer,
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

const navItems = [
  { id: "overview", label: "Обзор", hint: "Состояние продукта", icon: Gauge },
  { id: "servers", label: "VPS", hint: "Флот и действия", icon: Server },
  { id: "incidents", label: "Инциденты", hint: "SLA и алерты", icon: Bell },
  { id: "users", label: "Пользователи", hint: "Аудитория и ключи", icon: Users },
  { id: "payments", label: "Платежи", hint: "Выручка и риски", icon: WalletCards },
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
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [toast, setToast] = useState<string>("");
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
    } catch (error) {
      setToast(error instanceof Error ? error.message : "Ошибка команды");
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
                  onSelectSection={setActiveSection}
                  onSelectServer={setSelectedServerID}
                  onRefreshHealth={refreshServerHealth}
                  busyAction={busyAction}
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
                            <IconButton label="Details" onClick={() => setSelectedServerID(server.id)} icon={ChevronRight} />
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
              />
            ) : null}
          </aside>
        </div>
      </main>
    </div>
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
  onSelectSection,
  onSelectServer,
  onRefreshHealth,
  busyAction,
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
  onSelectSection: (section: AdminSection) => void;
  onSelectServer: (id: number) => void;
  onRefreshHealth: (serverID: number) => Promise<void>;
  busyAction: string | null;
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
      <div className="grid gap-4 xl:grid-cols-3">
        <StatusCard icon={Users} label="Аудитория" value={String(userPoints.at(-1)?.value ?? 0)} detail="Регистрации за выбранный период" />
        <StatusCard icon={ShieldCheck} label="Активные ключи" value={String(servers.reduce((sum, server) => sum + (server.active_keys ?? 0), 0))} detail="Распределены по VPS" />
        <PanelHeader icon={Globe2} title="Срез по регионам" text={`${new Set(servers.map((server) => server.region).filter(Boolean)).size || 0} регионов, VIP-only маршрутизация видна в таблице ниже.`} />
      </div>
    );
  }

  if (section === "payments") {
    return (
      <div className="grid gap-4 xl:grid-cols-[1fr_1fr_0.8fr]">
        <SparkAreaChart title="Выручка по дням" points={revenuePoints} color="#22c55e" />
        <SparkAreaChart title="Рост пользователей" points={userPoints} color="#facc15" />
        <PanelHeader icon={WalletCards} title="Финансовый контроль" text="Платежные риски выводятся рядом с инцидентами, чтобы не терять связь между выручкой и доступностью сети." />
      </div>
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

function ServerDrawer({ server, tab, setTab, auditLogs, busyAction, onAction, onRefreshHealth }: { server: AdminServer; tab: DrawerTab; setTab: (tab: DrawerTab) => void; auditLogs: AdminAuditLog[]; busyAction: string | null; onAction: (label: string, url: string, body?: unknown, method?: string) => Promise<void>; onRefreshHealth: (serverID: number) => Promise<void> }) {
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
          <ConfigImportPanel server={server} busyAction={busyAction} onAction={onAction} />
        ) : null}
        {tab === "Actions" ? (
          <div className="space-y-3">
            <Button className="w-full justify-start" variant="secondary" onClick={() => onAction(`status-${server.id}`, `/api/admin/servers/${server.id}/agent/status`, undefined, "GET")} disabled={busyAction === `status-${server.id}`}><RefreshCw size={14} /> Agent status</Button>
            <Button className="w-full justify-start" variant="secondary" onClick={() => {
              const digest = window.prompt("Immutable image digest");
              if (digest) void onAction(`update-${server.id}`, `/api/admin/servers/${server.id}/agent/update`, { image_digest: digest });
            }}><Zap size={14} /> Update by digest</Button>
            <Button className="w-full justify-start" variant="secondary" onClick={() => {
              if (window.confirm("Rollback agent image?")) void onAction(`rollback-${server.id}`, `/api/admin/servers/${server.id}/agent/rollback`);
            }}><RotateCcw size={14} /> Rollback</Button>
            <Button className="w-full justify-start" variant="destructive" onClick={() => {
              if (window.confirm("Toggle server active state?")) void onAction(`toggle-${server.id}`, `/api/admin/servers/${server.id}/toggle`);
            }}><AlertTriangle size={14} /> Toggle active</Button>
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

function ConfigImportPanel({ server, busyAction, onAction }: { server: AdminServer; busyAction: string | null; onAction: (label: string, url: string, body?: unknown, method?: string) => Promise<void> }) {
  const [awgConfig, setAwgConfig] = useState(() => seedAWGConfig(server));
  const [xrayConfig, setXrayConfig] = useState("");
  const [xrayPublicKey, setXrayPublicKey] = useState(server.config_summary?.xray?.public_key ?? "");
  const [xrayShortID, setXrayShortID] = useState(server.config_summary?.xray?.short_id ?? "");
  const [xrayClientID, setXrayClientID] = useState("");
  const busy = busyAction === `config-import-${server.id}`;

  useEffect(() => {
    setAwgConfig(seedAWGConfig(server));
    setXrayPublicKey(server.config_summary?.xray?.public_key ?? "");
    setXrayShortID(server.config_summary?.xray?.short_id ?? "");
    setXrayClientID("");
    setXrayConfig("");
  }, [server.id]);

  return (
    <div className="space-y-3">
      <div className="rounded-lg border border-amber-300/20 bg-amber-300/[0.05] p-3 text-xs leading-5 text-amber-50/80">
        Импорт сохраняет шаблон в базе и не пишет напрямую в файлы VPS. Для применения на ноде используйте snapshot/agent действия отдельно.
      </div>
      <label className="block">
        <span className="mb-1 block text-xs font-semibold text-zinc-400">AWG config</span>
        <textarea
          value={awgConfig}
          onChange={(event) => setAwgConfig(event.target.value)}
          className="min-h-36 w-full resize-y rounded-lg border border-white/10 bg-black/35 p-3 font-mono text-xs text-zinc-100 outline-none focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20"
          spellCheck={false}
        />
      </label>
      <label className="block">
        <span className="mb-1 block text-xs font-semibold text-zinc-400">Xray server.json</span>
        <textarea
          value={xrayConfig}
          onChange={(event) => setXrayConfig(event.target.value)}
          placeholder='{"inbounds":[{"protocol":"vless","port":443,"streamSettings":{"network":"xhttp","security":"reality"}}]}'
          className="min-h-40 w-full resize-y rounded-lg border border-white/10 bg-black/35 p-3 font-mono text-xs text-zinc-100 outline-none focus:border-amber-300/60 focus:ring-2 focus:ring-amber-300/20"
          spellCheck={false}
        />
      </label>
      <div className="grid gap-2 sm:grid-cols-3">
        <ConfigInput label="Reality public key" value={xrayPublicKey} onChange={setXrayPublicKey} />
        <ConfigInput label="Short ID" value={xrayShortID} onChange={setXrayShortID} />
        <ConfigInput label="Template client ID" value={xrayClientID} onChange={setXrayClientID} />
      </div>
      <Button
        size="sm"
        className="w-full"
        disabled={busy || (!awgConfig.trim() && !xrayConfig.trim())}
        onClick={() =>
          onAction(`config-import-${server.id}`, `/api/admin/servers/${server.id}/configs/import`, {
            awg_config: awgConfig,
            xray_config_json: xrayConfig,
            xray_public_key: xrayPublicKey,
            xray_short_id: xrayShortID,
            xray_client_id: xrayClientID,
          })
        }
      >
        <DatabaseBackup size={14} /> {busy ? "Импорт..." : "Импортировать конфиги"}
      </Button>
    </div>
  );
}

function ConfigInput({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold text-zinc-500">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
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
