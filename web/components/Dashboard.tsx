"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { motion, useReducedMotion } from "motion/react";
import {
  CheckCircle2,
  CreditCard,
  Download,
  Home,
  Laptop,
  LogOut,
  Mail,
  MonitorDown,
  Send,
  Smartphone,
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
            <div className="subscription-date">
              <p className="muted">Действует до</p>
              <strong>{expiresAt}</strong>
            </div>
          </motion.section>

          {(message || error || selectedPlan) && (
            <motion.div className={`notice ${error ? "error" : ""}`} variants={enter}>
              {error || message || "Тариф выбран. Завершите оплату ниже."}
            </motion.div>
          )}

          <motion.section id="subscription" variants={enter}>
            <Pricing04
              initialPlan={selectedPlan}
              loadingPlan={loadingPayment}
              mode="payment"
              onSelect={createPayment}
              plans={config.plans}
            />
          </motion.section>

          <motion.section className="panel" id="downloads" variants={enter}>
            <h3>Скачать FBLink VPN</h3>
            <p className="muted">Выберите платформу. Основные приложения скачиваются напрямую.</p>
            <div className="downloads">
              {(["android", "windows", "macos", "linux", "happ"] as const).map((platform) => {
                const Icon = platformIcons[platform];
                const label = platform === "happ" ? "iPhone" : platform === "macos" ? "macOS" : platform;
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
              <h3>Данные аккаунта</h3>
              <p className="muted">
                Почта привязана к подписке. При смене устройства войдите в кабинет
                и скачайте нужное приложение заново.
              </p>
              <span className="account-email">{session.user.email}</span>
            </motion.div>
          </motion.section>
        </motion.section>
      </div>
    </main>
  );
}
