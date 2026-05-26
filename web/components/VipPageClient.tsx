"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { motion, useReducedMotion } from "motion/react";
import { ArrowLeft, Mail, Send } from "lucide-react";
import { Brand } from "./Brand";
import { VipSection } from "./VipSection";

export function VipPageClient() {
  const router = useRouter();
  const reduceMotion = useReducedMotion();
  const [session, setSession] = useState<any>(null);
  const [config, setConfig] = useState<any>(null);

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

  if (!session || !config) {
    return (
      <main className="page-shell">
        <motion.div
          animate={{ opacity: 1, y: 0 }}
          className="container loading-state"
          initial={reduceMotion ? false : { opacity: 0, y: 10 }}
          transition={{ duration: 0.25 }}
        >
          <Brand />
          <p className="muted">Загружаем VIP панель...</p>
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
            <a className="button button-ghost" href={config.support.telegram} rel="noreferrer" target="_blank">
              <Send size={16} /> Telegram
            </a>
            <a className="button button-secondary" href={`mailto:${config.support.email}`}>
              <Mail size={16} /> Поддержка
            </a>
          </div>
        </div>
      </header>
      
      <div className="container dashboard-shell" style={{ display: 'block', paddingTop: '40px' }}>
        <div style={{ marginBottom: '32px' }}>
          <button className="button button-ghost" onClick={() => router.push('/dashboard')}>
            <ArrowLeft size={16} /> Вернуться в Дашборд
          </button>
        </div>
        
        <VipSection session={session} />
      </div>
    </main>
  );
}
