import Image from "next/image";
import * as motion from "motion/react-client";
import { Check, Download, Lock, ShieldCheck, Sparkles, Zap } from "lucide-react";
import { Topbar } from "../components/Topbar";
import { formatRub, loadSiteConfig } from "../lib/site-config";

const reveal = {
  hidden: { opacity: 0, y: 18 },
  show: { opacity: 1, y: 0 },
};

const stagger = {
  hidden: {},
  show: {
    transition: {
      staggerChildren: 0.08,
    },
  },
};

export default async function HomePage() {
  const config = await loadSiteConfig();

  return (
    <main className="page-shell">
      <Topbar />
      <section className="container hero">
        <motion.div
          animate="show"
          className="hero-copy"
          initial="hidden"
          transition={{ duration: 0.42, ease: "easeOut" }}
          variants={stagger}
        >
          <motion.h1 variants={reveal}>FBLink VPN. Подписка и приложения в одном кабинете.</motion.h1>
          <motion.p variants={reveal}>
            Быстрый доступ к VPN без лишних экранов: Premium или VIP, оплата, продление,
            прямые загрузки и iOS через Happ.
          </motion.p>
          <motion.div className="hero-actions" variants={reveal}>
            <a className="button button-primary" href="#plans">
              <Lock size={18} /> Купить подписку
            </a>
            <a className="button button-secondary" href="/dashboard">
              <Download size={18} /> Скачать приложение
            </a>
          </motion.div>
          <motion.div className="hero-proof" variants={reveal}>
            <span>Premium / VIP</span>
            <span>1 или 3 месяца</span>
            <span>Happ для iOS</span>
          </motion.div>
        </motion.div>
        <motion.div
          animate={{ opacity: 1, y: 0 }}
          className="hero-device"
          initial={{ opacity: 0, y: 24 }}
          transition={{ duration: 0.55, delay: 0.08, ease: "easeOut" }}
        >
          <div className="signal-ring" aria-hidden="true" />
          <div className="brand-orb">
            <Image src="/brand-icon.png" width={360} height={360} alt="FBLink VPN logo" priority />
          </div>
          <div className="connection-card">
            <div>
              <span className="eyebrow">Статус</span>
              <strong>Защищено</strong>
            </div>
            <span className="status-dot" aria-label="active" />
          </div>
          <div className="connection-grid">
            <div>
              <span>Протокол</span>
              <strong>Xray / AWG</strong>
            </div>
            <div>
              <span>iOS</span>
              <strong>Happ link</strong>
            </div>
          </div>
        </motion.div>
      </section>

      <motion.section
        className="container section"
        id="plans"
        initial="hidden"
        transition={{ duration: 0.38, ease: "easeOut" }}
        variants={stagger}
        viewport={{ once: true, amount: 0.22 }}
        whileInView="show"
      >
        <div className="section-head">
          <motion.h2 variants={reveal}>Только два тарифа</motion.h2>
          <p>Только два понятных плана: Premium для ежедневного доступа и VIP для приоритетной сети.</p>
        </div>
        <div className="plans-grid">
          {config.plans.map((plan) => (
            <motion.article className={`plan-card ${plan.code === "vip" ? "vip" : ""}`} key={plan.code} variants={reveal}>
              <div className="plan-title">
                <h3>{plan.title}</h3>
                {plan.code === "vip" ? <Zap size={22} color="#EAB308" /> : <ShieldCheck size={22} color="#EAB308" />}
              </div>
              <p className="muted">{plan.description}</p>
              <div className="periods">
                {plan.periods.map((period) => (
                  <div className="period" key={period.id}>
                    <strong>{period.label}</strong>
                    <div className="plan-price">{formatRub(period.amount)}</div>
                  </div>
                ))}
              </div>
              <ul className="feature-list">
                {plan.features.map((feature) => (
                  <li key={feature}>
                    <Check size={16} /> {feature}
                  </li>
                ))}
              </ul>
              <a className="button button-primary" href={`/auth?plan=${plan.periods[0].id}`}>
                Купить подписку
              </a>
            </motion.article>
          ))}
        </div>
      </motion.section>

      <motion.section
        className="container section compact-section"
        initial="hidden"
        transition={{ duration: 0.38, ease: "easeOut" }}
        variants={stagger}
        viewport={{ once: true, amount: 0.22 }}
        whileInView="show"
      >
        <div className="grid-two">
          <motion.div className="panel info-panel" variants={reveal}>
            <Sparkles size={22} />
            <h3>Все приложения рядом</h3>
            <p className="muted">
              Android, Windows, macOS и Linux скачиваются напрямую. На iOS используем Happ и личную
              подписочную ссылку с вашими серверами.
            </p>
          </motion.div>
          <motion.div className="panel info-panel" variants={reveal}>
            <ShieldCheck size={22} />
            <h3>Поддержка без лишнего шума</h3>
            <p className="muted">
              В кабинете будут только быстрые контакты: email и Telegram. Без перегруженной тикетной
              системы в первом релизе.
            </p>
          </motion.div>
        </div>
      </motion.section>
    </main>
  );
}
