import * as motion from "motion/react-client";
import { MailQuestion, MessageCircle, MessagesSquare, Send, ShieldCheck } from "lucide-react";
import { Brand } from "../../components/Brand";
import { loadSiteConfig, supportChannels } from "../../lib/site-config";

const channelIcons = {
  telegram: Send,
  whatsapp: MessageCircle,
  max: MessagesSquare,
} as const;

export default async function PolicyPage() {
  const config = await loadSiteConfig();
  const support = supportChannels(config);

  return (
    <main className="page-shell">
      <section className="container section compact-section">
        <Brand />
        <motion.div
          animate={{ opacity: 1, y: 0 }}
          className="panel policy-panel"
          initial={{ opacity: 0, y: 16 }}
          transition={{ duration: 0.35, ease: "easeOut" }}
        >
          <span className="eyebrow">
            <ShieldCheck size={14} /> Legal
          </span>
          <h1>Политика конфиденциальности FBLink VPN</h1>
          <p className="muted">
            Мы обрабатываем только данные, необходимые для аккаунта, оплаты, поддержки и работы VPN-сервиса. Трафик, DNS-запросы
            и содержимое соединений не логируются.
          </p>

          <div className="policy-grid">
            <article>
              <h2>Какие данные используются</h2>
              <p>
                Email аккаунта, статус подписки, сведения об оплате от платежного провайдера, технические данные приложения и сообщения,
                которые пользователь сам отправляет в поддержку.
              </p>
            </article>
            <article>
              <h2>Зачем это нужно</h2>
              <p>Чтобы создать аккаунт, выдать доступ, восстановить подписку, отправить чек, обработать обращение и защитить сервис от злоупотреблений.</p>
            </article>
            <article>
              <h2>Поддержка</h2>
              <p>
                Основная почта техподдержки: <a href={`mailto:${config.support.email}`}>{config.support.email}</a>.
              </p>
              <div className="support-ribbon policy-support-ribbon">
                {support.map((channel) => {
                  const Icon = channelIcons[channel.id];
                  return (
                    <a className={`support-chip support-chip-${channel.id}`} href={channel.href} key={channel.id} rel="noreferrer" target="_blank">
                      <Icon size={16} />
                      <span>{channel.label}</span>
                    </a>
                  );
                })}
                <a className="support-chip" href={`mailto:${config.support.email}`}>
                  <MailQuestion size={16} />
                  <span>Email</span>
                </a>
              </div>
            </article>
            <article>
              <h2>Запросы пользователя</h2>
              <p>По вопросам доступа, удаления или уточнения данных напишите на почту поддержки. В обращении можно указать support tag из приложения.</p>
            </article>
          </div>
        </motion.div>
      </section>
    </main>
  );
}
