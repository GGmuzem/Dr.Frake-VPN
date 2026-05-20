"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { motion, useReducedMotion } from "motion/react";
import {
  Apple,
  CheckCircle2,
  CreditCard,
  Crown,
  Download,
  Home,
  Laptop,
  LogOut,
  Mail,
  MonitorDown,
  Send,
  ShieldCheck,
  Smartphone,
  Sparkles,
} from "lucide-react";
import { Brand } from "./Brand";
import Pricing04 from "@/components/ui/ruixen-pricing-04";
import { defaultSiteConfig } from "../lib/site-config";
import type { PlanId, SiteConfig } from "../lib/site-config";

type Session = {
  authenticated: boolean;
  user: { email: string };
  subscription: {
    plan: PlanId | "free" | "trial";
    status: "active" | "expired" | "cancelled";
    expires_at: string;
    auto_renew: boolean;
  };
};

type HappLink = {
  happ_url: string;
};

const platformIcons = {
  android: Smartphone,
  windows: MonitorDown,
  macos: Laptop,
  linux: Download,
  happ: Apple,
};

const platformLabels: Record<keyof typeof platformIcons, string> = {
  android: "Android",
  windows: "Windows",
  macos: "macOS",
  linux: "Linux",
  happ: "iPhone",
};

const enter = {
  hidden: { opacity: 0, y: 16 },
  show: { opacity: 1, y: 0 },
};

const enterGroup = {
  hidden: {},
  show: {
    transition: {
      staggerChildren: 0.06,
    },
  },
};

const PLAN_BADGE: Record<string, { label: string; icon: typeof Crown }> = {
  vip: { label: "VIP", icon: Crown },
  vip_3m: { label: "VIP", icon: Crown },
  basic: { label: "Premium", icon: ShieldCheck },
  basic_3m: { label: "Premium", icon: ShieldCheck },
  trial: { label: "Trial", icon: Sparkles },
  free: { label: "Free", icon: Sparkles },
};

const DAYS_TOTAL = 90;
const RING_CIRCUM = 2 * Math.PI * 54;

export function Dashboard() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const reduceMotion = useReducedMotion();
  const [session, setSession] = useState<Session | null>(null);
  const [config, setConfig] = useState<SiteConfig>(defaultSiteConfig);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loadingPayment, setLoadingPayment] = useState<PlanId | "">("");

  useEffect(() => {
    let active = true;
    async function load() {
      const [sessionResponse, configResponse] = await Promise.all([
        fetch("/api/session"),
        fetch("/api/site-config"),
      ]);
      if (!active) return;
      if (sessionResponse.status === 401) {
        router.replace("/auth");
        return;
      }
      if (sessionResponse.ok) {
        setSession(await sessionResponse.json());
      }
      if (configResponse.ok) {
        setConfig(await configResponse.json());
      }
    }
    void load();
    return () => {
      active = false;
    };
  }, [router]);

  const selectedPlan = searchParams.get("plan") as PlanId | null;

  const subscriptionMeta = useMemo(() => {
    if (!session) return null;
    const expiresAtRaw = session.subscription.expires_at;
    const expiresDate = expiresAtRaw ? new Date(expiresAtRaw) : null;
    const formatted = expiresDate
      ? new Intl.DateTimeFormat("ru-RU", { dateStyle: "long" }).format(expiresDate)
      : "—";
    const now = Date.now();
    const ms = expiresDate ? expiresDate.getTime() - now : 0;
    const daysLeft = Math.max(0, Math.ceil(ms / (1000 * 60 * 60 * 24)));
    const progress = Math.min(1, Math.max(0, daysLeft / DAYS_TOTAL));
    const planMeta = PLAN_BADGE[session.subscription.plan] ?? PLAN_BADGE.free;
    const isActive = session.subscription.status === "active" && daysLeft > 0;
    return { formatted, daysLeft, progress, planMeta, isActive };
  }, [session]);

  async function createPayment(plan: PlanId) {
    setError("");
    setMessage("");
    setLoadingPayment(plan);
    try {
      const response = await fetch("/api/payments/create", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ plan }),
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error ?? "Не удалось создать платеж");
      if (data.confirmation_url) {
        window.location.href = data.confirmation_url;
      } else {
        setMessage("Подписка активирована.");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка оплаты");
    } finally {
      setLoadingPayment("");
    }
  }

  async function openHapp() {
    setError("");
    setMessage("");
    try {
      const response = await fetch("/api/happ-link", { method: "POST" });
      const data = (await response.json()) as HappLink & { error?: string };
      if (!response.ok) throw new Error(data.error ?? "Не удалось создать ссылку");
      if (!data.happ_url) throw new Error("Не удалось получить ссылку");
      window.location.href = data.happ_url;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка ссылки");
    }
  }

  async function logout() {
    await fetch("/api/auth/logout", { method: "POST" });
    router.replace("/");
  }

  if (!session || !subscriptionMeta) {
    return (
      <main className="page-shell">
        <motion.div
          animate={{ opacity: 1, y: 0 }}
          className="container loading-state"
          initial={reduceMotion ? false : { opacity: 0, y: 10 }}
          transition={{ duration: 0.25 }}
        >
          <Brand />
          <p className="muted">Загружаем кабинет...</p>
        </motion.div>
      </main>
    );
  }

  const { formatted, daysLeft, progress, planMeta, isActive } = subscriptionMeta;
  const PlanIcon = planMeta.icon;
  const dashOffset = RING_CIRCUM * (1 - progress);

  return (
    <main className="page-shell">
      <header className="topbar">
        <div className="container topbar-inner">
          <Brand />
          <div className="nav-actions">
            <a className="button button-ghost" href={config.support.telegram} rel="noreferrer" target="_blank">
              <Send size={16} /> Telegram
            </a>
            <a className="button button-secondary" href={`mailto:${config.support.email}`}>
              <Mail size={16} /> Поддержка
            </a>
          </div>
        </div>
      </header>
      <div className="container dashboard-shell">
        <motion.aside
          animate="show"
          className="panel sidebar"
          initial={reduceMotion ? false : "hidden"}
          transition={{ duration: 0.32, ease: "easeOut" }}
          variants={enter}
        >
          <div className="sidebar-user">
            <div className="sidebar-avatar" aria-hidden="true">
              {session.user.email.charAt(0).toUpperCase()}
            </div>
            <div className="sidebar-user-info">
              <strong>{session.user.email}</strong>
              <span>
                <PlanIcon size={12} /> {planMeta.label}
              </span>
            </div>
          </div>
          <nav aria-label="Кабинет">
            <a className="active" href="#home">
              <Home size={17} /> Главная
            </a>
            <a href="#subscription">
              <CreditCard size={17} /> Подписка
            </a>
            <a href="#downloads">
              <Download size={17} /> Скачать
            </a>
            <button onClick={logout}>
              <LogOut size={17} /> Выйти
            </button>
          </nav>
        </motion.aside>

        <motion.section
          animate="show"
          className="dashboard-main"
          initial={reduceMotion ? false : "hidden"}
          variants={enterGroup}
        >
          <motion.section className="connection-hero" id="home" variants={enter}>
            <div className="connection-hero-glow" aria-hidden="true" />
            <div className="connection-hero-ring">
              <svg viewBox="0 0 120 120" width={140} height={140} aria-hidden="true">
                <defs>
                  <linearGradient id="ring-gradient" x1="0%" x2="100%" y1="0%" y2="100%">
                    <stop offset="0%" stopColor="#FACC15" />
                    <stop offset="100%" stopColor="#EAB308" />
                  </linearGradient>
                </defs>
                <circle cx="60" cy="60" r="54" stroke="rgba(255,255,255,0.08)" strokeWidth="6" fill="none" />
                <circle
                  cx="60"
                  cy="60"
                  r="54"
                  stroke={isActive ? "url(#ring-gradient)" : "rgba(239,68,68,0.6)"}
                  strokeWidth="6"
                  fill="none"
                  strokeLinecap="round"
                  strokeDasharray={RING_CIRCUM}
                  strokeDashoffset={dashOffset}
                  transform="rotate(-90 60 60)"
                  style={{ transition: "stroke-dashoffset 800ms ease" }}
                />
              </svg>
              <div className="connection-hero-ring-center">
                <ShieldCheck size={26} />
                <strong>{isActive ? "Защита включена" : "Подписка истекла"}</strong>
              </div>
            </div>

            <div className="connection-hero-body">
              <div className="status-pill" data-active={isActive ? "true" : "false"}>
                <span className={`dot ${isActive ? "dot-green" : "dot-red"}`} />
                {isActive ? "Подписка активна" : "Требуется продление"}
              </div>
              <h2>
                {isActive ? `Осталось ${daysLeft} ${pluralizeDays(daysLeft)}` : "Подписка истекла"}
              </h2>
              <p className="muted">
                {isActive
                  ? `Действует до ${formatted}. Управляйте подпиской ниже.`
                  : "Выберите план — и подключение восстановится сразу после оплаты."}
              </p>

              <div className="connection-hero-meta">
                <div>
                  <span>План</span>
                  <strong>
                    <PlanIcon size={14} /> {planMeta.label}
                  </strong>
                </div>
                <div>
                  <span>Email</span>
                  <strong className="account-email">{session.user.email}</strong>
                </div>
                <div>
                  <span>Автопродление</span>
                  <strong>{session.subscription.auto_renew ? "Включено" : "Выключено"}</strong>
                </div>
              </div>
            </div>
          </motion.section>

          {(message || error || selectedPlan) && (
            <motion.div className={`notice ${error ? "error" : ""}`} variants={enter}>
              {error || message || "Тариф выбран. Завершите оплату ниже."}
            </motion.div>
          )}

          <motion.section className="dashboard-section" id="subscription" variants={enter}>
            <div className="dashboard-section-head">
              <span className="eyebrow">
                <CreditCard size={12} /> Подписка
              </span>
              <h2>
                {isActive ? "Продлить или сменить план" : "Активировать доступ"}
              </h2>
            </div>
            <Pricing04
              initialPlan={selectedPlan}
              loadingPlan={loadingPayment}
              mode="payment"
              onSelect={createPayment}
              plans={config.plans}
            />
          </motion.section>

          <motion.section className="panel" id="downloads" variants={enter}>
            <div className="dashboard-section-head">
              <span className="eyebrow">
                <Download size={12} /> Приложения
              </span>
              <h2>Скачать FBLink VPN</h2>
              <p className="muted">Выберите платформу. Авторизация — тем же email.</p>
            </div>
            <div className="downloads">
              {(["android", "windows", "macos", "linux", "happ"] as const).map((platform) => {
                const Icon = platformIcons[platform];
                const label = platformLabels[platform];
                return (
                  <motion.div
                    className="download-card"
                    key={platform}
                    whileHover={reduceMotion ? undefined : { y: -2 }}
                    whileTap={reduceMotion ? undefined : { scale: 0.99 }}
                  >
                    <Icon size={28} />
                    <strong>{label}</strong>
                    {platform === "happ" ? (
                      <button className="button button-primary" onClick={openHapp} type="button">
                        Открыть
                      </button>
                    ) : (
                      <a className="button button-secondary" href={config.downloads[platform]}>
                        Скачать
                      </a>
                    )}
                  </motion.div>
                );
              })}
            </div>
          </motion.section>

          <motion.section className="grid-two" variants={enterGroup}>
            <motion.div className="panel support-panel" variants={enter}>
              <div className="info-panel-icon">
                <Mail size={20} />
              </div>
              <h3>Поддержка 24/7</h3>
              <p className="muted">Отвечаем в течение часа. Email или Telegram — что удобнее.</p>
              <div className="hero-actions">
                <a className="button button-primary" href={config.support.telegram} target="_blank" rel="noreferrer">
                  <Send size={17} /> Telegram
                </a>
                <a className="button button-secondary" href={`mailto:${config.support.email}`}>
                  <Mail size={17} /> Email
                </a>
              </div>
            </motion.div>
            <motion.div className="panel support-panel" variants={enter}>
              <div className="info-panel-icon">
                <CheckCircle2 size={20} />
              </div>
              <h3>Данные аккаунта</h3>
              <p className="muted">
                Подписка привязана к email. На любом устройстве — тот же логин и пароль.
              </p>
              <span className="account-email">{session.user.email}</span>
            </motion.div>
          </motion.section>
        </motion.section>
      </div>
    </main>
  );
}

function pluralizeDays(n: number): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return "день";
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return "дня";
  return "дней";
}
