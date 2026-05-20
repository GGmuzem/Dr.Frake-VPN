import Image from "next/image";
import * as motion from "motion/react-client";
import { Download, Laptop, MonitorDown, ShieldCheck, Smartphone, Sparkles } from "lucide-react";
import { DitheringShader } from "@/components/ui/dithering-shader";
import Pricing04 from "@/components/ui/ruixen-pricing-04";
import { Topbar } from "../components/Topbar";
import { loadSiteConfig } from "../lib/site-config";

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
  const platforms = [
    { label: "Android", icon: Smartphone, href: config.downloads.android },
    { label: "Windows", icon: MonitorDown, href: config.downloads.windows },
    { label: "macOS", icon: Laptop, href: config.downloads.macos },
    { label: "Linux", icon: Download, href: config.downloads.linux },
  ];

  return (
    <main className="page-shell">
      <Topbar />
      <section className="container hero app-hero">
        <div className="hero-wave" aria-hidden="true">
          <DitheringShader
            className="hero-wave-canvas"
            colorBack="#070707"
            colorFront="#EAB308"
            height={720}
            pxSize={4}
            shape="wave"
            speed={0.34}
            type="8x8"
            width={1200}
          />
        </div>
        <motion.div
          animate="show"
          className="hero-copy"
          initial="hidden"
          transition={{ duration: 0.42, ease: "easeOut" }}
          variants={stagger}
        >
          <motion.h1 variants={reveal}>Скачайте FBLink VPN и подключайтесь за минуту.</motion.h1>
          <motion.p variants={reveal}>
            Приложения для основных платформ, понятная подписка Premium или VIP и личный
            кабинет без лишних экранов. Выбираете тариф, скачиваете приложение, включаете VPN.
          </motion.p>
          <motion.div className="hero-actions" variants={reveal}>
            <a className="button button-secondary" href="/dashboard">
              <Download size={18} /> Скачать приложение
            </a>
            <a className="button button-primary" href="#plans">
              Выбрать подписку
            </a>
          </motion.div>
          <motion.div className="hero-proof" variants={reveal}>
            <span>Premium / VIP</span>
            <span>1 или 3 месяца</span>
            <span>Android, Windows, macOS, Linux</span>
          </motion.div>
        </motion.div>

        <motion.div
          animate={{ opacity: 1, y: 0 }}
          className="app-showcase"
          initial={{ opacity: 0, y: 24 }}
          transition={{ duration: 0.55, delay: 0.08, ease: "easeOut" }}
        >
          <div className="app-showcase-screen">
            <div className="app-showcase-top">
              <Image src="/brand-icon.png" width={72} height={72} alt="FBLink VPN logo" priority />
              <div>
                <span>FBLink VPN</span>
                <strong>Защита включена</strong>
              </div>
              <span className="status-dot" aria-label="active" />
            </div>
            <div className="app-power-button">
              <ShieldCheck size={36} />
            </div>
            <div className="app-showcase-meta">
              <span>Сервер</span>
              <strong>Auto Select</strong>
              <span>Пинг</span>
              <strong>42 ms</strong>
            </div>
          </div>
          <div className="platform-dock">
            {platforms.map((platform) => {
              const Icon = platform.icon;
              return (
                <a href={platform.href} key={platform.label}>
                  <Icon size={18} />
                  <span>{platform.label}</span>
                </a>
              );
            })}
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
          <motion.h2 variants={reveal}>Выбор подписки без квеста</motion.h2>
          <p>Два тарифа и два срока. Сначала выбираете план, потом срок, затем переходите к оплате.</p>
        </div>
        <Pricing04 plans={config.plans} />
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
            <h3>Приложение на первом месте</h3>
            <p className="muted">
              На сайте не нужно изучать инструкции. Главное действие всегда рядом: скачать клиент
              и подключиться после оплаты.
            </p>
          </motion.div>
          <motion.div className="panel info-panel" variants={reveal}>
            <ShieldCheck size={22} />
            <h3>Кабинет без перегруза</h3>
            <p className="muted">
              Статус подписки, продление, скачивания и поддержка остаются на одном экране.
              Без лишних графиков и второстепенных блоков.
            </p>
          </motion.div>
        </div>
      </motion.section>
    </main>
  );
}
