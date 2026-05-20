"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { motion, useReducedMotion } from "motion/react";
import {
  CheckCircle2,
  CreditCard,
  Download,
  ExternalLink,
  Home,
  Laptop,
  LogOut,
  Mail,
  MonitorDown,
  Send,
  ShieldCheck,
  Smartphone,
} from "lucide-react";
import { Brand } from "./Brand";
import { defaultSiteConfig, formatRub } from "../lib/site-config";
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
  subscription_url: string;
  happ_url: string;
};

const platformIcons = {
  android: Smartphone,
  windows: MonitorDown,
  macos: Laptop,
  linux: Download,
  happ: Smartphone,
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

export function Dashboard() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const reduceMotion = useReducedMotion();
  const [session, setSession] = useState<Session | null>(null);
  const [config, setConfig] = useState<SiteConfig>(defaultSiteConfig);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loadingPayment, setLoadingPayment] = useState<PlanId | "">("");
  const [happLink, setHappLink] = useState("");

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
  const expiresAt = useMemo(() => {
    if (!session?.subscription.expires_at) return "неизвестно";
    return new Intl.DateTimeFormat("ru-RU", { dateStyle: "long" }).format(new Date(session.subscription.expires_at));
  }, [session?.subscription.expires_at]);

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
      if (!response.ok) throw new Error(data.error ?? "Не удалось создать ссылку для Happ");
      setHappLink(data.subscription_url);
      window.location.href = data.happ_url || data.subscription_url;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка Happ-ссылки");
    }
  }

  async function logout() {
    await fetch("/api/auth/logout", { method: "POST" });
    router.replace("/");
  }

  if (!session) {
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

  return (
    <main className="page-shell">
      <header className="topbar">
        <div className="container topbar-inner">
          <Brand />
          <div className="nav-actions">
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
          <Brand />
          <nav aria-label="Кабинет">
            <a className="active" href="#home">
              <Home size={17} /> Главная
            </a>
            <a href="#subscription">
              <CreditCard size={17} /> Подписка
            </a>
            <a href="#downloads">
              <Download size={17} /> Скачивание
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
          <motion.section className="panel subscription-panel" id="home" variants={enter}>
            <div className="subscription-state">
              <span className="success-icon">
                <CheckCircle2 size={26} />
              </span>
              <div>
                <h2>{session.subscription.status === "active" ? "Подписка активна" : "Подписка не активна"}</h2>
                <p className="muted">{session.user.email}</p>
              </div>
            </div>
            <div>
              <p className="muted">Действует до</p>
              <strong>{expiresAt}</strong>
            </div>
          </motion.section>

          {(message || error || selectedPlan) && (
            <motion.div className={`notice ${error ? "error" : ""}`} variants={enter}>
              {error || message || `Выбран тариф ${selectedPlan}. Завершите оплату ниже.`}
            </motion.div>
          )}

          <motion.section className="grid-two" id="subscription" variants={enterGroup}>
            {config.plans.map((plan) => (
              <motion.article
                className={`plan-card ${plan.code === "vip" ? "vip" : ""}`}
                key={plan.code}
                variants={enter}
                whileHover={reduceMotion ? undefined : { y: -2 }}
              >
                <div className="plan-title">
                  <h3>{plan.title}</h3>
                  <ShieldCheck size={21} color="#EAB308" />
                </div>
                <p className="muted">{plan.description}</p>
                <div className="periods">
                  {plan.periods.map((period) => (
                    <motion.button
                      className="period"
                      disabled={loadingPayment !== ""}
                      key={period.id}
                      onClick={() => createPayment(period.id)}
                      type="button"
                      whileTap={reduceMotion ? undefined : { scale: 0.985 }}
                    >
                      <strong>{period.label}</strong>
                      <div className="plan-price">{formatRub(period.amount)}</div>
                      <span className="muted">{loadingPayment === period.id ? "Создаем платеж..." : "Оплатить"}</span>
                    </motion.button>
                  ))}
                </div>
              </motion.article>
            ))}
          </motion.section>

          <motion.section className="panel" id="downloads" variants={enter}>
            <h3>Скачать приложение</h3>
            <p className="muted">Выберите платформу. Для iOS используйте Happ и личную подписку FBLink VPN.</p>
            <div className="downloads">
              {(["android", "windows", "macos", "linux", "happ"] as const).map((platform) => {
                const Icon = platformIcons[platform];
                const label = platform === "happ" ? "iOS Happ" : platform === "macos" ? "macOS" : platform;
                return (
                  <motion.div
                    className="download-card"
                    key={platform}
                    whileHover={reduceMotion ? undefined : { y: -2 }}
                    whileTap={reduceMotion ? undefined : { scale: 0.99 }}
                  >
                    <Icon size={24} />
                    <strong>{label}</strong>
                    {platform === "happ" ? (
                      <button className="button button-primary" onClick={openHapp} type="button">
                        Открыть в Happ
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
            {happLink && (
              <motion.p
                animate={{ opacity: 1, y: 0 }}
                className="muted happ-manual-link"
                initial={reduceMotion ? false : { opacity: 0, y: 8 }}
              >
                Ссылка для ручного добавления в Happ: {happLink}
              </motion.p>
            )}
          </motion.section>

          <motion.section className="grid-two" variants={enterGroup}>
            <motion.div className="panel support-panel" variants={enter}>
              <h3>Поддержка</h3>
              <p className="muted">Напишите нам удобным способом.</p>
              <div className="hero-actions">
                <a className="button button-secondary" href={`mailto:${config.support.email}`}>
                  <Mail size={17} /> Email
                </a>
                <a className="button button-secondary" href={config.support.telegram} target="_blank" rel="noreferrer">
                  <Send size={17} /> Telegram
                </a>
              </div>
            </motion.div>
            <motion.div className="panel support-panel" variants={enter}>
              <h3>iOS через Happ</h3>
              <p className="muted">
                Для Premium и VIP сайт выдает VLESS/Xray подписку. Остальные платформы используют приложения FBLink.
              </p>
              <a className="button button-secondary" href={config.downloads.happ} target="_blank" rel="noreferrer">
                <ExternalLink size={17} /> Найти Happ
              </a>
            </motion.div>
          </motion.section>
        </motion.section>
      </div>
    </main>
  );
}
