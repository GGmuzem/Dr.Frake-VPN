"use client";

import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { CheckCircle2, Tag, X, Loader2 } from "lucide-react";

type PromoState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "success"; discountPercent: number; originalAmount: number; finalAmount: number; discountAmount: number }
  | { status: "error"; message: string };

type PromoCodeInputProps = {
  /** Called whenever a valid promo code is confirmed (or cleared). */
  onConfirm: (code: string) => void;
  /** Plan used for server-side preview call. */
  previewPlan?: string;
};

export function PromoCodeInput({ onConfirm, previewPlan = "basic" }: PromoCodeInputProps) {
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const [promo, setPromo] = useState<PromoState>({ status: "idle" });
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) setTimeout(() => inputRef.current?.focus(), 80);
  }, [open]);

  async function apply() {
    const code = input.trim().toUpperCase();
    if (!code) return;

    setPromo({ status: "loading" });
    try {
      const res = await fetch("/api/payments/preview", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ plan: previewPlan, promo_code: code }),
      });
      const data = await res.json();
      if (!res.ok) {
        setPromo({ status: "error", message: data.error ?? "Промокод недействителен" });
        onConfirm("");
        return;
      }
      if (!data.promo_applied) {
        setPromo({ status: "error", message: "Промокод не применён к этому тарифу" });
        onConfirm("");
        return;
      }
      setPromo({
        status: "success",
        discountPercent: data.discount_percent,
        originalAmount: data.original_amount,
        finalAmount: data.amount,
        discountAmount: data.discount_amount,
      });
      onConfirm(code);
    } catch {
      setPromo({ status: "error", message: "Ошибка соединения" });
      onConfirm("");
    }
  }

  function clear() {
    setInput("");
    setPromo({ status: "idle" });
    onConfirm("");
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter") apply();
    if (e.key === "Escape") { clear(); setOpen(false); }
  }

  const isSuccess = promo.status === "success";
  const isError = promo.status === "error";
  const isLoading = promo.status === "loading";

  return (
    <div className="promo-code-wrapper">
      {!open ? (
        <button
          className="promo-toggle"
          onClick={() => setOpen(true)}
          type="button"
        >
          <Tag size={14} />
          Есть промокод?
        </button>
      ) : (
        <AnimatePresence>
          <motion.div
            animate={{ opacity: 1, y: 0 }}
            className="promo-panel"
            exit={{ opacity: 0, y: -6 }}
            initial={{ opacity: 0, y: -6 }}
            transition={{ duration: 0.18, ease: "easeOut" }}
          >
            <div className={`promo-field ${isSuccess ? "promo-field--success" : ""} ${isError ? "promo-field--error" : ""}`}>
              <Tag size={15} className="promo-field-icon" />
              <input
                ref={inputRef}
                autoComplete="off"
                className="promo-input"
                disabled={isLoading || isSuccess}
                id="promo-code-input"
                maxLength={32}
                onChange={(e) => {
                  setInput(e.target.value.toUpperCase());
                  if (promo.status !== "idle") setPromo({ status: "idle" });
                  if (isSuccess) onConfirm("");
                }}
                onKeyDown={handleKeyDown}
                placeholder="ПРОМОКОД"
                spellCheck={false}
                type="text"
                value={input}
              />
              {isSuccess ? (
                <button className="promo-field-btn promo-field-btn--clear" onClick={clear} type="button" title="Убрать промокод">
                  <X size={14} />
                </button>
              ) : (
                <button
                  className="promo-field-btn promo-field-btn--apply"
                  disabled={!input.trim() || isLoading}
                  onClick={apply}
                  type="button"
                >
                  {isLoading ? <Loader2 size={14} className="promo-spin" /> : "Применить"}
                </button>
              )}
            </div>

            <AnimatePresence mode="wait">
              {isSuccess && (
                <motion.div
                  animate={{ opacity: 1, height: "auto" }}
                  className="promo-result promo-result--success"
                  exit={{ opacity: 0, height: 0 }}
                  initial={{ opacity: 0, height: 0 }}
                  key="success"
                  transition={{ duration: 0.2 }}
                >
                  <CheckCircle2 size={14} />
                  <span>
                    Скидка <strong>{promo.discountPercent}%</strong> применена
                    {promo.discountAmount > 0 && (
                      <> — вы экономите <strong>{promo.discountAmount} ₽</strong></>
                    )}
                  </span>
                </motion.div>
              )}
              {isError && (
                <motion.div
                  animate={{ opacity: 1, height: "auto" }}
                  className="promo-result promo-result--error"
                  exit={{ opacity: 0, height: 0 }}
                  initial={{ opacity: 0, height: 0 }}
                  key="error"
                  transition={{ duration: 0.2 }}
                >
                  <X size={13} />
                  <span>{promo.message}</span>
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        </AnimatePresence>
      )}
    </div>
  );
}
