import Image from "next/image";
import { Check, Download, Lock, ShieldCheck, Zap } from "lucide-react";
import { Topbar } from "../components/Topbar";
import { formatRub, loadSiteConfig } from "../lib/site-config";

export default async function HomePage() {
  const config = await loadSiteConfig();

  return (
    <main className="page-shell">
      <Topbar />
      <section className="container hero">
        <div className="hero-copy">
          <h1>Безопасный доступ. Полная свобода.</h1>
          <p>
            FBLink VPN защищает соединение, помогает обходить сетевые ограничения и
            остается простым: подписка, приложения и iOS через Happ в одном кабинете.
          </p>
          <div className="hero-actions">
            <a className="button button-primary" href="#plans">
              <Lock size={18} /> Купить подписку
            </a>
            <a className="button button-secondary" href="/dashboard">
              <Download size={18} /> Скачать приложение
            </a>
          </div>
        </div>
        <div className="hero-card">
          <Image src="/brand-icon.png" width={520} height={520} alt="FBLink VPN logo" priority />
          <div className="hero-card-row">
            <span>Защищенное подключение</span>
            <span className="status-dot" aria-label="active" />
          </div>
        </div>
      </section>

      <section id="plans" className="container section">
        <div className="section-head">
          <h2>Выберите тариф</h2>
          <p>Только два понятных плана: Premium для ежедневного доступа и VIP для приоритетной сети.</p>
        </div>
        <div className="plans-grid">
          {config.plans.map((plan) => (
            <article className={`plan-card ${plan.code === "vip" ? "vip" : ""}`} key={plan.code}>
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
            </article>
          ))}
        </div>
      </section>

      <section className="container section">
        <div className="grid-two">
          <div className="panel">
            <h3>Все приложения рядом</h3>
            <p className="muted">
              Android, Windows, macOS и Linux скачиваются напрямую. На iOS используем Happ и личную
              подписочную ссылку с вашими серверами.
            </p>
          </div>
          <div className="panel">
            <h3>Поддержка без лишнего шума</h3>
            <p className="muted">
              В кабинете будут только быстрые контакты: email и Telegram. Без перегруженной тикетной
              системы в первом релизе.
            </p>
          </div>
        </div>
      </section>
    </main>
  );
}
