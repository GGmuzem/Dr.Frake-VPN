import Image from "next/image";
import * as motion from "motion/react-client";
import {
  Apple,
  ArrowRight,
  Cpu,
  Download,
  Globe2,
  Laptop,
  Lock,
  MailQuestion,
  MonitorDown,
  Send,
  ShieldCheck,
  Smartphone,
  Sparkles,
  Wifi,
  Zap,
} from "lucide-react";
import { DitheringShader } from "@/components/ui/dithering-shader";
import Pricing04 from "@/components/ui/ruixen-pricing-04";
import { Topbar } from "../components/Topbar";
import { Faq } from "../components/Faq";
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

const METRICS = [
  { value: "120+", label: "серверов в 30+ странах" },
  { value: "0", label: "логов и трекеров" },
  { value: "1 Гбит/с", label: "пропускная способность канала" },
  { value: "5", label: "устройств на одной подписке" },
];

const FEATURES = [
  {
    icon: ShieldCheck,
    title: "Без логов",
    badge: "Privacy",
    description:
      "Не пишем журналы трафика и DNS-запросов. У нас просто нечего отдать по запросу — только email и факт подписки.",
    span: 2,
  },
  {
    icon: Zap,
    title: "Быстрое подключение",
    badge: "Performance",
    description: "AmneziaWG и Xray Reality. Средний пинг 42 мс по СНГ и Европе.",
  },
  {
    icon: Globe2,
    title: "30+ стран",
    badge: "Global",
    description: "Auto Select подбирает ближайший узел. Один тап в приложении — и трафик защищен.",
  },
  {
    icon: Cpu,
    title: "AmneziaWG + Xray",
    badge: "Tech",
    description:
      "Современные протоколы с DPI-устойчивостью. Reality поверх TLS, маскировка под Google/Cloudflare.",
  },
  {
    icon: Lock,
    title: "Kill Switch",
    badge: "Safety",
    description: "Если туннель упал — никаких утечек. Firewall блокирует трафик до восстановления соединения.",
  },
  {
    icon: Wifi,
    title: "Split Tunneling",
    badge: "Control",
    description: "Выбирайте, какие приложения и сайты идут через VPN, а какие напрямую.",
    span: 2,
  },
];

export default async function HomePage() {
  const config = await loadSiteConfig();
  const platforms = [
    { label: "Android", icon: Smartphone, href: config.downloads.android },
    { label: "Windows", icon: MonitorDown, href: config.downloads.windows },
    { label: "macOS", icon: Laptop, href: config.downloads.macos },
    { label: "Linux", icon: Download, href: config.downloads.linux },
    { label: "iPhone", icon: Apple, href: config.downloads.happ },
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
          <motion.span className="eyebrow" variants={reveal}>
            <ShieldCheck size={14} /> AmneziaWG · Xray Reality · No logs
          </motion.span>
          <motion.h1 variants={reveal}>
            Приватный интернет <span className="gold-gradient">за минуту.</span>
          </motion.h1>
          <motion.p variants={reveal}>
            FBLink VPN — премиальный сервис без логов. Скачайте приложение, выберите подписку
            и подключайтесь одним тапом на Android, Windows, macOS, Linux и iPhone.
          </motion.p>
          <motion.div className="hero-actions" variants={reveal}>
            <a className="button button-primary" href="#plans">
              Выбрать подписку <ArrowRight size={16} />
            </a>
            <a className="button button-secondary" href="#platforms">
              <Download size={16} /> Скачать приложение
            </a>
          </motion.div>
          <motion.div className="hero-proof" variants={reveal}>
            <span>
              <span className="dot dot-green" /> 2 500+ активных пользователей
            </span>
            <span>5 устройств / подписка</span>
            <span>Возврат за 7 дней</span>
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
              <Image src="/brand-icon.png" width={56} height={56} alt="FBLink VPN logo" priority />
              <div>
                <span>FBLink VPN</span>
                <strong>Защита включена</strong>
              </div>
              <span className="status-dot" aria-label="active" />
            </div>
            <div className="app-power-button">
              <div className="app-power-ring" aria-hidden="true" />
              <ShieldCheck size={36} />
            </div>
            <div className="app-showcase-meta">
              <span>Сервер</span>
              <strong>Auto Select · NL</strong>
              <span>Пинг</span>
              <strong>42 ms</strong>
            </div>
          </div>
          <div className="platform-dock">
            {platforms.slice(0, 4).map((platform) => {
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

      <section className="container metric-strip" aria-label="Ключевые показатели">
        {METRICS.map((metric) => (
          <div className="metric" key={metric.label}>
            <strong className="gold-gradient">{metric.value}</strong>
            <span>{metric.label}</span>
          </div>
        ))}
      </section>

      <motion.section
        className="container section"
        id="features"
        initial="hidden"
        transition={{ duration: 0.38, ease: "easeOut" }}
        variants={stagger}
        viewport={{ once: true, amount: 0.18 }}
        whileInView="show"
      >
        <div className="section-head section-head-center">
          <motion.span className="eyebrow" variants={reveal}>
            Возможности
          </motion.span>
          <motion.h2 variants={reveal}>Премиум-функции по цене кофе</motion.h2>
          <motion.p variants={reveal}>
            Современные протоколы, защита от утечек и продуманный кабинет. Без баннеров,
            рекламы и сбора телеметрии.
          </motion.p>
        </div>
        <div className="bento">
          {FEATURES.map((feature) => {
            const Icon = feature.icon;
            return (
              <motion.article
                className={`bento-card ${feature.span === 2 ? "bento-card-wide" : ""}`}
                key={feature.title}
                variants={reveal}
              >
                <div className="bento-card-icon">
                  <Icon size={20} />
                </div>
                <div className="bento-card-body">
                  <span className="bento-card-badge">{feature.badge}</span>
                  <h3>{feature.title}</h3>
                  <p>{feature.description}</p>
                </div>
              </motion.article>
            );
          })}
        </div>
      </motion.section>

      <motion.section
        className="container section"
        id="plans"
        initial="hidden"
        transition={{ duration: 0.38, ease: "easeOut" }}
        variants={stagger}
        viewport={{ once: true, amount: 0.18 }}
        whileInView="show"
      >
        <div className="section-head section-head-center">
          <motion.span className="eyebrow" variants={reveal}>
            Подписка
          </motion.span>
          <motion.h2 variants={reveal}>
            Один тариф — один тап. <span className="gold-gradient">Без скрытых платежей.</span>
          </motion.h2>
          <motion.p variants={reveal}>
            Два тарифа и два срока. Выбираете план, выбираете срок, переходите к оплате.
            Возврат в течение 7 дней — без вопросов.
          </motion.p>
        </div>
        <Pricing04 plans={config.plans} />
      </motion.section>

      <motion.section
        className="container section"
        id="platforms"
        initial="hidden"
        transition={{ duration: 0.38, ease: "easeOut" }}
        variants={stagger}
        viewport={{ once: true, amount: 0.22 }}
        whileInView="show"
      >
        <div className="section-head section-head-center">
          <motion.span className="eyebrow" variants={reveal}>
            Платформы
          </motion.span>
          <motion.h2 variants={reveal}>Все ваши устройства — одной подпиской</motion.h2>
        </div>
        <div className="platform-grid">
          {platforms.map((platform) => {
            const Icon = platform.icon;
            return (
              <motion.a
                className="platform-tile"
                href={platform.href}
                key={platform.label}
                variants={reveal}
              >
                <Icon size={28} />
                <strong>{platform.label}</strong>
                <span>Скачать</span>
              </motion.a>
            );
          })}
        </div>
      </motion.section>

      <motion.section
        className="container section"
        id="faq"
        initial="hidden"
        transition={{ duration: 0.38, ease: "easeOut" }}
        variants={stagger}
        viewport={{ once: true, amount: 0.18 }}
        whileInView="show"
      >
        <div className="section-head section-head-center">
          <motion.span className="eyebrow" variants={reveal}>
            FAQ
          </motion.span>
          <motion.h2 variants={reveal}>Частые вопросы</motion.h2>
          <motion.p variants={reveal}>
            Если не нашли ответ — напишите нам в Telegram или на почту, отвечаем в течение часа.
          </motion.p>
        </div>
        <Faq />
      </motion.section>

      <motion.section
        className="container section"
        initial="hidden"
        transition={{ duration: 0.38, ease: "easeOut" }}
        variants={stagger}
        viewport={{ once: true, amount: 0.22 }}
        whileInView="show"
      >
        <motion.div className="cta-card" variants={reveal}>
          <div className="cta-card-glow" aria-hidden="true" />
          <Sparkles size={22} />
          <h2>
            Готовы начать? <span className="gold-gradient">Подписка с возвратом 7 дней.</span>
          </h2>
          <p>
            Создайте аккаунт за 30 секунд, выберите тариф и скачайте приложение.
            Если что-то пойдет не так — вернем деньги без вопросов.
          </p>
          <div className="hero-actions">
            <a className="button button-primary" href="/auth">
              Создать аккаунт <ArrowRight size={16} />
            </a>
            <a className="button button-secondary" href={`mailto:${config.support.email}`}>
              <MailQuestion size={16} /> Задать вопрос
            </a>
          </div>
        </motion.div>
      </motion.section>

      <footer className="site-footer">
        <div className="container site-footer-inner">
          <div className="site-footer-brand">
            <strong>FBLink VPN</strong>
            <p>Премиальный VPN без логов. Россия и СНГ.</p>
          </div>
          <div className="site-footer-links">
            <a href="#features">Возможности</a>
            <a href="#plans">Тарифы</a>
            <a href="#faq">FAQ</a>
            <a href={`mailto:${config.support.email}`}>
              <MailQuestion size={14} /> {config.support.email}
            </a>
            <a href={config.support.telegram} rel="noreferrer" target="_blank">
              <Send size={14} /> Telegram
            </a>
          </div>
          <span className="site-footer-copy">© {new Date().getFullYear()} FBLink VPN</span>
        </div>
      </footer>
    </main>
  );
}
