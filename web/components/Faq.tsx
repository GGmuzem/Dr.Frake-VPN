"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";
import { motion, AnimatePresence, useReducedMotion } from "motion/react";

type FaqItem = {
  question: string;
  answer: string;
};

const ITEMS: FaqItem[] = [
  {
    question: "Хранит ли FBLink VPN логи?",
    answer:
      "Нет. Мы не пишем журналы посещенных ресурсов, IP-адресов и трафика. Хранится только email и статус подписки.",
  },
  {
    question: "На скольких устройствах можно использовать одну подписку?",
    answer:
      "Без лимита. Используйте один email и пароль на любом количестве устройств — Android, Windows, macOS, Linux и iPhone.",
  },
  {
    question: "Чем VIP отличается от Premium?",
    answer:
      "VIP добавляет приоритетные серверы, Xray Reality для регионов с глубокой DPI, AdBlock DNS и более высокую скорость на пиковой нагрузке.",
  },
  {
    question: "Что делать, если подключение медленное?",
    answer:
      "В приложении переключите сервер на другую локацию вручную — попробуйте ближайшую к вам страну. Если проблема осталась — напишите в поддержку, поможем подобрать сервер.",
  },
  {
    question: "С каких карт можно оплатить?",
    answer:
      "Всех российских банковских карт. Оплата идёт через YooKassa, подписка активируется сразу после платежа.",
  },
  {
    question: "Подойдет ли VPN для стриминга и игр?",
    answer:
      "Да. Серверы оптимизированы под низкий пинг и стабильность 4K/UHD-стриминга. Для онлайн-игр выбирайте ближайшую локацию.",
  },
];

export function Faq() {
  const [open, setOpen] = useState<number | null>(0);
  const reduce = useReducedMotion();

  return (
    <div className="faq">
      {ITEMS.map((item, index) => {
        const isOpen = open === index;
        return (
          <div className={`faq-item ${isOpen ? "is-open" : ""}`} key={item.question}>
            <button
              aria-expanded={isOpen}
              className="faq-trigger"
              onClick={() => setOpen(isOpen ? null : index)}
              type="button"
            >
              <span>{item.question}</span>
              <ChevronDown
                aria-hidden="true"
                className="faq-chevron"
                size={20}
                style={{ transform: isOpen ? "rotate(180deg)" : "none" }}
              />
            </button>
            <AnimatePresence initial={false}>
              {isOpen && (
                <motion.div
                  animate={{ height: "auto", opacity: 1 }}
                  className="faq-body"
                  exit={{ height: 0, opacity: 0 }}
                  initial={reduce ? false : { height: 0, opacity: 0 }}
                  transition={{ duration: 0.22, ease: [0.4, 0, 0.2, 1] }}
                >
                  <p>{item.answer}</p>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        );
      })}
    </div>
  );
}
