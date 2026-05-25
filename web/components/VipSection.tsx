"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import {
  AlertTriangle,
  Check,
  Edit2,
  ExternalLink,
  Layers,
  Plus,
  RefreshCw,
  Route,
  Server,
  ShieldAlert,
  Trash2,
  X,
  Zap,
} from "lucide-react";
import {
  buildHappRoutingProfile,
  countRoutingRules,
  normalizeRuleText,
  routingSummary,
  type RoutingAction,
  type RoutingProfile,
} from "@/lib/routing-profiles";

type Session = {
  authenticated: boolean;
  user: { email: string };
  subscription: {
    plan: string;
    status: string;
    expires_at: string;
    auto_renew: boolean;
    vip_ad_block_enabled?: boolean;
  };
};

type ToastState = {
  tone: "success" | "error" | "info";
  title: string;
  message?: string;
};

type EditorMode = "create" | "edit";

type EditorState = {
  mode: EditorMode;
  id: number | null;
  name: string;
  action: RoutingAction;
  enabled: boolean;
  domains: string;
  suffixes: string;
  cidrs: string;
};

const emptyEditor: EditorState = {
  mode: "create",
  id: null,
  name: "",
  action: "proxy",
  enabled: true,
  domains: "",
  suffixes: "",
  cidrs: "",
};

const panelMotion = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -8 },
};

function encodeBase64Unicode(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}

function isVIPPlan(plan: string): boolean {
  return plan === "vip" || plan === "vip_3m";
}

function actionLabel(action: RoutingAction): string {
  return action === "proxy" ? "Через VPN" : "Без VPN";
}

function actionTone(action: RoutingAction): string {
  return action === "proxy" ? "proxy" : "direct";
}

function pluralRu(count: number, one: string, few: string, many: string): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return few;
  return many;
}

function ruleCountLabel(count: number, one: string, few: string, many: string): string {
  return `${count} ${pluralRu(count, one, few, many)}`;
}

function activeProfilesLabel(count: number): string {
  return `${ruleCountLabel(count, "активный профиль", "активных профиля", "активных профилей")} · FBLink VPN`;
}

function prettifyTechnicalText(value: string): string {
  return value.replace(/\.xn--p1ai/g, ".рф").replace(/\bxn--p1ai\b/g, "рф");
}

function displayProfileName(profile: RoutingProfile): string {
  if (profile.code === "ru_direct" || profile.name === "RU без VPN") {
    return "Россия без VPN";
  }
  return prettifyTechnicalText(profile.name);
}

function displayProfileDescription(profile: RoutingProfile): string {
  if (profile.code === "ru_direct") {
    return "Российские домены (.ru, .рф) и локальные ресурсы идут напрямую.";
  }
  return prettifyTechnicalText(profile.description || routingSummary(profile));
}

function profilePayload(profile: RoutingProfile, enabled = profile.enabled) {
  return {
    name: profile.name,
    action: profile.action,
    enabled,
    domains: profile.domains || [],
    domain_suffixes: profile.domain_suffixes || [],
    cidrs: profile.cidrs || [],
  };
}

export function VipSection({ session }: { session: Session }) {
  const reduceMotion = useReducedMotion();
  const [profiles, setProfiles] = useState<RoutingProfile[]>([]);
  const [adBlockEnabled, setAdBlockEnabled] = useState(session.subscription.vip_ad_block_enabled || false);
  const [activeTab, setActiveTab] = useState<"custom" | "system">("custom");
  const [loadingProfiles, setLoadingProfiles] = useState(false);
  const [adBlockSaving, setAdBlockSaving] = useState(false);
  const [savingProfileId, setSavingProfileId] = useState<number | "new" | null>(null);
  const [deletingProfileId, setDeletingProfileId] = useState<number | null>(null);
  const [copyingPresetCode, setCopyingPresetCode] = useState<string | null>(null);
  const [exportingHapp, setExportingHapp] = useState(false);
  const [editor, setEditor] = useState<EditorState | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<RoutingProfile | null>(null);
  const [toast, setToast] = useState<ToastState | null>(null);

  const isVip = isVIPPlan(session.subscription.plan);

  const customProfiles = useMemo(() => profiles.filter((profile) => profile.kind === "custom"), [profiles]);
  const systemProfiles = useMemo(() => profiles.filter((profile) => profile.kind === "system"), [profiles]);
  const enabledCustomProfiles = useMemo(
    () => customProfiles.filter((profile) => profile.enabled),
    [customProfiles],
  );
  const totalRules = useMemo(
    () => enabledCustomProfiles.reduce((sum, profile) => sum + countRoutingRules(profile).total, 0),
    [enabledCustomProfiles],
  );

  const fetchProfiles = useCallback(async () => {
    setLoadingProfiles(true);
    try {
      const response = await fetch("/api/routing-profiles");
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || "Не удалось загрузить VIP-правила");
      }
      setProfiles(Array.isArray(data.profiles) ? data.profiles : []);
    } catch (err) {
      setToast({
        tone: "error",
        title: "Не удалось загрузить VIP-правила",
        message: err instanceof Error ? err.message : "Повторите попытку чуть позже.",
      });
    } finally {
      setLoadingProfiles(false);
    }
  }, []);

  useEffect(() => {
    if (isVip) {
      void fetchProfiles();
    }
  }, [fetchProfiles, isVip]);

  async function toggleAdBlock() {
    setAdBlockSaving(true);
    try {
      const nextEnabled = !adBlockEnabled;
      const response = await fetch("/api/subscription/ad-block", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: nextEnabled }),
      });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || "Ошибка изменения AdBlock");
      }
      setAdBlockEnabled(nextEnabled);
      setToast({
        tone: "success",
        title: nextEnabled ? "AdBlock включен" : "AdBlock выключен",
        message: "Настройка применится при следующем обновлении конфигурации.",
      });
    } catch (err) {
      setToast({
        tone: "error",
        title: "Не удалось изменить AdBlock",
        message: err instanceof Error ? err.message : "Повторите попытку.",
      });
    } finally {
      setAdBlockSaving(false);
    }
  }

  async function toggleProfile(profile: RoutingProfile) {
    setSavingProfileId(profile.id);
    try {
      const response = await fetch(`/api/routing-profiles/${profile.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(profilePayload(profile, !profile.enabled)),
      });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || "Ошибка изменения профиля");
      }
      setProfiles((items) =>
        items.map((item) => (item.id === profile.id ? { ...item, enabled: !item.enabled } : item)),
      );
    } catch (err) {
      setToast({
        tone: "error",
        title: "Не удалось изменить профиль",
        message: err instanceof Error ? err.message : "Правила не были сохранены.",
      });
    } finally {
      setSavingProfileId(null);
    }
  }

  async function confirmDeleteProfile() {
    if (!deleteTarget) return;
    setDeletingProfileId(deleteTarget.id);
    try {
      const response = await fetch(`/api/routing-profiles/${deleteTarget.id}`, { method: "DELETE" });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || "Ошибка удаления");
      }
      setProfiles((items) => items.filter((profile) => profile.id !== deleteTarget.id));
      setToast({ tone: "success", title: "Профиль удален" });
      setDeleteTarget(null);
    } catch (err) {
      setToast({
        tone: "error",
        title: "Не удалось удалить профиль",
        message: err instanceof Error ? err.message : "Попробуйте еще раз.",
      });
    } finally {
      setDeletingProfileId(null);
    }
  }

  async function saveProfile() {
    if (!editor) return;
    const body = {
      name: editor.name.trim(),
      action: editor.action,
      enabled: editor.enabled,
      domains: normalizeRuleText(editor.domains),
      domain_suffixes: normalizeRuleText(editor.suffixes),
      cidrs: normalizeRuleText(editor.cidrs),
    };
    setSavingProfileId(editor.mode === "create" ? "new" : editor.id);

    try {
      const response = await fetch(
        editor.mode === "create" ? "/api/routing-profiles" : `/api/routing-profiles/${editor.id}`,
        {
          method: editor.mode === "create" ? "POST" : "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
      );
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || "Ошибка сохранения правила");
      }
      setEditor(null);
      setActiveTab("custom");
      await fetchProfiles();
      setToast({ tone: "success", title: editor.mode === "create" ? "Профиль создан" : "Профиль обновлен" });
    } catch (err) {
      setToast({
        tone: "error",
        title: "Не удалось сохранить профиль",
        message: err instanceof Error ? err.message : "Проверьте правила и повторите попытку.",
      });
    } finally {
      setSavingProfileId(null);
    }
  }

  async function copySystemProfile(code: string) {
    setCopyingPresetCode(code);
    try {
      const response = await fetch(`/api/routing-profiles/system/${encodeURIComponent(code)}/copy`, {
        method: "POST",
      });
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.error || "Ошибка добавления системного пресета");
      }
      await fetchProfiles();
      setActiveTab("custom");
      setToast({
        tone: "success",
        title: data.created ? "Пресет добавлен" : "Пресет уже был добавлен",
        message: "Он появился в разделе «Мои правила».",
      });
    } catch (err) {
      setToast({
        tone: "error",
        title: "Не удалось добавить пресет",
        message: err instanceof Error ? err.message : "Попробуйте еще раз.",
      });
    } finally {
      setCopyingPresetCode(null);
    }
  }

  function startCreate() {
    setEditor(emptyEditor);
  }

  function startEdit(profile: RoutingProfile) {
    setEditor({
      mode: "edit",
      id: profile.id,
      name: profile.name,
      action: profile.action || "proxy",
      enabled: profile.enabled,
      domains: (profile.domains || []).join("\n"),
      suffixes: (profile.domain_suffixes || []).join("\n"),
      cidrs: (profile.cidrs || []).join("\n"),
    });
  }

  function exportToHapp() {
    if (enabledCustomProfiles.length === 0) {
      setToast({
        tone: "info",
        title: "Нет активных правил",
        message: "Включите хотя бы один профиль перед экспортом.",
      });
      return;
    }

    setExportingHapp(true);
    try {
      const payload = buildHappRoutingProfile(enabledCustomProfiles);
      const deepLink = `happ://routing/onadd/${encodeBase64Unicode(JSON.stringify(payload))}`;
      setToast({
        tone: "success",
        title: "Откроется Happ",
        message: "Профиль маршрутизации называется FBLink VPN. Если он уже есть, обновите его.",
      });
      window.setTimeout(() => {
        window.location.href = deepLink;
        setExportingHapp(false);
      }, 280);
    } catch {
      setExportingHapp(false);
      setToast({ tone: "error", title: "Ошибка формирования ссылки" });
    }
  }

  if (!isVip) {
    return (
      <motion.div
        animate="show"
        className="panel vip-paywall glass-panel"
        initial={reduceMotion ? false : "hidden"}
        variants={panelMotion}
      >
        <div className="vip-paywall-content">
          <Zap size={48} className="gold-icon" />
          <h2>Доступно только для VIP</h2>
          <p className="muted">
            Умная маршрутизация и встроенный AdBlock DNS доступны только в тарифе VIP.
          </p>
          <a href="/dashboard#subscription" className="button button-primary gold-button">
            Обновить до VIP
          </a>
        </div>
      </motion.div>
    );
  }

  return (
    <div className="vip-dashboard-content">
      <AnimatePresence>
        {toast && (
          <motion.div
            animate={{ opacity: 1, y: 0 }}
            className={`vip-toast vip-toast-${toast.tone}`}
            exit={{ opacity: 0, y: -8 }}
            initial={{ opacity: 0, y: -8 }}
            role="status"
          >
            <div>
              <strong>{toast.title}</strong>
              {toast.message && <span>{toast.message}</span>}
            </div>
            <button aria-label="Закрыть уведомление" onClick={() => setToast(null)} type="button">
              <X size={15} />
            </button>
          </motion.div>
        )}
      </AnimatePresence>

      <motion.section
        animate="show"
        className="vip-command-panel"
        initial={reduceMotion ? false : "hidden"}
        variants={panelMotion}
      >
        <div className="vip-command-copy">
          <h1>Маршрутизация VIP</h1>
          <p>Настройте AdBlock DNS и правила трафика без ручной правки конфигураций.</p>
        </div>
        <div className="vip-status-grid">
          <StatusTile label="Подписка" value="VIP активна" tone="success" />
          <StatusTile label="AdBlock" value={adBlockEnabled ? "Включен" : "Выключен"} tone={adBlockEnabled ? "success" : "idle"} />
          <StatusTile label="Активные правила" value={String(enabledCustomProfiles.length)} tone={enabledCustomProfiles.length > 0 ? "success" : "idle"} />
          <StatusTile label="Всего правил" value={String(totalRules)} tone={totalRules > 0 ? "success" : "idle"} />
        </div>
      </motion.section>

      <motion.section
        animate="show"
        className="panel glass-panel vip-adblock-panel"
        initial={reduceMotion ? false : "hidden"}
        variants={panelMotion}
      >
        <div>
          <span className="eyebrow">
            <ShieldAlert size={12} /> DNS
          </span>
          <h2>AdBlock</h2>
          <p className="muted">Блокирует рекламу, трекеры и фишинговые домены на уровне DNS.</p>
        </div>
        <label className="vip-switch-row">
          <span>{adBlockEnabled ? "Фильтрация включена" : "Фильтрация выключена"}</span>
          <span className="switch">
            <input checked={adBlockEnabled} disabled={adBlockSaving} onChange={toggleAdBlock} type="checkbox" />
            <span className="slider" />
          </span>
        </label>
      </motion.section>

      <motion.section
        animate="show"
        className="panel glass-panel routing-panel vip-routing-panel"
        initial={reduceMotion ? false : "hidden"}
        variants={panelMotion}
      >
        <div className="vip-routing-head">
          <div>
            <span className="eyebrow">
              <Route size={12} /> Маршрутизация
            </span>
            <h2>Правила трафика</h2>
            <p className="muted">Соберите профиль FBLink VPN и отправьте правила в Happ отдельной ссылкой.</p>
          </div>
          <div className="vip-routing-actions">
            <button className="button button-secondary" disabled={loadingProfiles} onClick={fetchProfiles} type="button">
              <RefreshCw size={16} /> Обновить
            </button>
            <button
              className="button button-primary gold-button"
              disabled={exportingHapp || enabledCustomProfiles.length === 0}
              onClick={exportToHapp}
              type="button"
            >
              <ExternalLink size={16} /> Открыть в Happ
            </button>
          </div>
        </div>

        <div className="vip-segmented" role="tablist">
          <button
            aria-selected={activeTab === "custom"}
            className={activeTab === "custom" ? "active" : ""}
            onClick={() => setActiveTab("custom")}
            type="button"
          >
            <Layers size={16} /> Мои правила
          </button>
          <button
            aria-selected={activeTab === "system"}
            className={activeTab === "system" ? "active" : ""}
            onClick={() => setActiveTab("system")}
            type="button"
          >
            <Server size={16} /> Пресеты
          </button>
        </div>

        <AnimatePresence mode="wait">
          {activeTab === "custom" ? (
            <motion.div
              animate="show"
              className="routing-profiles-list"
              exit="exit"
              initial={reduceMotion ? false : "hidden"}
              key="custom"
              variants={panelMotion}
            >
              <div className="vip-list-toolbar">
                <div>
                  <strong>Мои правила</strong>
                  <span>{activeProfilesLabel(enabledCustomProfiles.length)}</span>
                </div>
                <motion.button
                  className="button button-secondary"
                  disabled={editor !== null}
                  onClick={startCreate}
                  type="button"
                  whileTap={reduceMotion ? undefined : { scale: 0.98 }}
                >
                  <Plus size={16} /> Создать
                </motion.button>
              </div>

              <AnimatePresence>
                {editor && (
                  <RoutingEditor
                    editor={editor}
                    loading={savingProfileId !== null}
                    onCancel={() => setEditor(null)}
                    onChange={setEditor}
                    onSave={saveProfile}
                    reduceMotion={!!reduceMotion}
                  />
                )}
              </AnimatePresence>

              {loadingProfiles && <LoadingRows />}

              {!loadingProfiles && customProfiles.length === 0 && !editor && (
                <EmptyState
                  actionLabel="Открыть пресеты"
                  onAction={() => setActiveTab("system")}
                  text="Добавьте готовый пресет или создайте свое правило."
                  title="Пока нет правил"
                />
              )}

              <AnimatePresence>
                {!loadingProfiles &&
                  customProfiles.map((profile) => (
                    <ProfileCard
                      key={profile.id}
                      loading={savingProfileId === profile.id || deletingProfileId === profile.id}
                      onDelete={() => setDeleteTarget(profile)}
                      onEdit={() => startEdit(profile)}
                      onToggle={() => toggleProfile(profile)}
                      profile={profile}
                      reduceMotion={!!reduceMotion}
                    />
                  ))}
              </AnimatePresence>
            </motion.div>
          ) : (
            <motion.div
              animate="show"
              className="routing-profiles-list"
              exit="exit"
              initial={reduceMotion ? false : "hidden"}
              key="system"
              variants={panelMotion}
            >
              {loadingProfiles && <LoadingRows />}
              {!loadingProfiles &&
                systemProfiles.map((profile) => (
                  <PresetCard
                    key={profile.id}
                    loading={copyingPresetCode === profile.code}
                    onCopy={() => copySystemProfile(profile.code)}
                    profile={profile}
                    reduceMotion={!!reduceMotion}
                  />
                ))}
            </motion.div>
          )}
        </AnimatePresence>
      </motion.section>

      <AnimatePresence>
        {deleteTarget && (
          <motion.div
            animate={{ opacity: 1 }}
            className="vip-modal-backdrop"
            exit={{ opacity: 0 }}
            initial={{ opacity: 0 }}
          >
            <motion.div
              animate={{ opacity: 1, scale: 1, y: 0 }}
              className="vip-modal"
              exit={{ opacity: 0, scale: 0.98, y: 8 }}
              initial={{ opacity: 0, scale: 0.98, y: 8 }}
            >
              <AlertTriangle size={24} />
              <h3>Удалить профиль?</h3>
              <p>Профиль «{displayProfileName(deleteTarget)}» будет удален из маршрутизации и экспорта в Happ.</p>
              <div className="vip-modal-actions">
                <button className="button button-ghost" onClick={() => setDeleteTarget(null)} type="button">
                  Отмена
                </button>
                <button
                  className="button vip-danger-button"
                  disabled={deletingProfileId === deleteTarget.id}
                  onClick={confirmDeleteProfile}
                  type="button"
                >
                  <Trash2 size={16} /> Удалить
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function StatusTile({ label, value, tone }: { label: string; value: string; tone: "success" | "idle" }) {
  return (
    <div className={`vip-status-tile ${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function RoutingEditor({
  editor,
  loading,
  onCancel,
  onChange,
  onSave,
  reduceMotion,
}: {
  editor: EditorState;
  loading: boolean;
  onCancel: () => void;
  onChange: (editor: EditorState) => void;
  onSave: () => void;
  reduceMotion: boolean;
}) {
  return (
    <motion.div
      animate={{ opacity: 1, height: "auto", y: 0 }}
      className="routing-editor"
      exit={{ opacity: 0, height: 0, y: -8 }}
      initial={reduceMotion ? false : { opacity: 0, height: 0, y: -8 }}
      layout
    >
      <div className="vip-editor-head">
        <div>
          <h3>{editor.mode === "create" ? "Новое правило" : "Редактировать правило"}</h3>
          <p>Разделяйте точные домены, доменные зоны и IP-сети. Так Xray и Happ корректно читают правила.</p>
        </div>
        <label className="vip-editor-enabled">
          <span>Включено</span>
          <span className="switch">
            <input
              checked={editor.enabled}
              onChange={(event) => onChange({ ...editor, enabled: event.target.checked })}
              type="checkbox"
            />
            <span className="slider" />
          </span>
        </label>
      </div>

      <div className="vip-editor-grid">
        <label>
          <span>Название</span>
          <input
            className="vip-input"
            onChange={(event) => onChange({ ...editor, name: event.target.value })}
            placeholder="Например: YouTube через VPN"
            value={editor.name}
          />
        </label>
        <label>
          <span>Действие</span>
          <select
            className="vip-input"
            onChange={(event) => onChange({ ...editor, action: event.target.value as RoutingAction })}
            value={editor.action}
          >
            <option value="proxy">Через VPN</option>
            <option value="direct">Без VPN</option>
          </select>
        </label>
      </div>

      <div className="vip-rule-fields">
        <label>
          <span>Домены</span>
          <textarea
            className="vip-input"
            onChange={(event) => onChange({ ...editor, domains: event.target.value })}
            placeholder={"youtube.com\nchatgpt.com"}
            value={editor.domains}
          />
        </label>
        <label>
          <span>Суффиксы</span>
          <textarea
            className="vip-input"
            onChange={(event) => onChange({ ...editor, suffixes: event.target.value })}
            placeholder={".googlevideo.com\n.ru"}
            value={editor.suffixes}
          />
        </label>
        <label>
          <span>IP / CIDR</span>
          <textarea
            className="vip-input"
            onChange={(event) => onChange({ ...editor, cidrs: event.target.value })}
            placeholder={"10.0.0.0/8\n192.168.0.0/16"}
            value={editor.cidrs}
          />
        </label>
      </div>

      <div className="vip-editor-actions">
        <button className="button button-ghost" disabled={loading} onClick={onCancel} type="button">
          <X size={16} /> Отмена
        </button>
        <button className="button button-primary gold-button" disabled={loading || !editor.name.trim()} onClick={onSave} type="button">
          <Check size={16} /> Сохранить
        </button>
      </div>
    </motion.div>
  );
}

function ProfileCard({
  profile,
  loading,
  onToggle,
  onEdit,
  onDelete,
  reduceMotion,
}: {
  profile: RoutingProfile;
  loading: boolean;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
  reduceMotion: boolean;
}) {
  return (
    <motion.article
      animate={{ opacity: 1, y: 0 }}
      className="routing-profile-item"
      exit={{ opacity: 0, y: -8 }}
      initial={reduceMotion ? false : { opacity: 0, y: 8 }}
      layout
      whileHover={reduceMotion ? undefined : { y: -2 }}
    >
      <div className="profile-info" data-disabled={!profile.enabled}>
        <div className="vip-profile-title-row">
          <strong>{displayProfileName(profile)}</strong>
          <span className={`vip-action-badge ${actionTone(profile.action)}`}>{actionLabel(profile.action)}</span>
        </div>
        <span>{routingSummary(profile)}</span>
      </div>
      <div className="vip-profile-actions">
        <label className="switch">
          <input checked={profile.enabled} disabled={loading} onChange={onToggle} type="checkbox" />
          <span className="slider" />
        </label>
        <button aria-label="Редактировать профиль" className="button button-ghost icon-button" disabled={loading} onClick={onEdit} type="button">
          <Edit2 size={16} />
        </button>
        <button aria-label="Удалить профиль" className="button button-ghost icon-button danger" disabled={loading} onClick={onDelete} type="button">
          <Trash2 size={16} />
        </button>
      </div>
    </motion.article>
  );
}

function PresetCard({
  profile,
  loading,
  onCopy,
  reduceMotion,
}: {
  profile: RoutingProfile;
  loading: boolean;
  onCopy: () => void;
  reduceMotion: boolean;
}) {
  const counts = countRoutingRules(profile);
  const added = profile.already_added === true;
  return (
    <motion.article
      animate={{ opacity: 1, y: 0 }}
      className={`routing-profile-item preset-card ${added ? "is-added" : ""}`}
      initial={reduceMotion ? false : { opacity: 0, y: 8 }}
      layout
      whileHover={reduceMotion || added ? undefined : { y: -2 }}
    >
      <div className="profile-info">
        <div className="vip-profile-title-row">
          <strong>{displayProfileName(profile)}</strong>
          <span className={`vip-action-badge ${actionTone(profile.action)}`}>{actionLabel(profile.action)}</span>
        </div>
        <span>{displayProfileDescription(profile)}</span>
        <div className="preset-counts">
          <span>{ruleCountLabel(counts.domains, "домен", "домена", "доменов")}</span>
          <span>{ruleCountLabel(counts.suffixes, "зона", "зоны", "зон")}</span>
          <span>{counts.cidrs} IP</span>
        </div>
      </div>
      {added ? (
        <span className="vip-added-label">
          <Check size={15} /> Добавлено
        </span>
      ) : (
        <button className="button button-secondary" disabled={loading} onClick={onCopy} type="button">
          <Plus size={16} /> {loading ? "Добавляем..." : "Добавить"}
        </button>
      )}
    </motion.article>
  );
}

function EmptyState({ title, text, actionLabel, onAction }: { title: string; text: string; actionLabel: string; onAction: () => void }) {
  return (
    <div className="vip-empty-state">
      <Layers size={24} />
      <strong>{title}</strong>
      <span>{text}</span>
      <button className="button button-secondary" onClick={onAction} type="button">
        {actionLabel}
      </button>
    </div>
  );
}

function LoadingRows() {
  return (
    <div className="vip-loading-list" aria-label="Загрузка VIP-правил">
      <span />
      <span />
      <span />
    </div>
  );
}
