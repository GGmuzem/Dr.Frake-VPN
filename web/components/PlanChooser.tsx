"use client";

import { useMemo, useState } from "react";
import { motion, useReducedMotion } from "motion/react";
import { Check, Crown, ShieldCheck } from "lucide-react";
import { formatRub } from "../lib/site-config";
import type { Plan, PlanId } from "../lib/site-config";

type PlanChooserProps = {
  plans: Plan[];
  initialPlan?: PlanId | null;
  loadingPlan?: PlanId | "";
  mode?: "link" | "payment";
  onSelect?: (plan: PlanId) => void;
};

function findPlan(plans: Plan[], planId?: PlanId | null): Plan | undefined {
  return plans.find((plan) => Array.isArray(plan.periods) && plan.periods.some((period) => period.id === planId));
}

export function PlanChooser({ plans, initialPlan, loadingPlan = "", mode = "link", onSelect }: PlanChooserProps) {
  const reduceMotion = useReducedMotion();
  const safePlans = useMemo(() => plans.filter((plan) => Array.isArray(plan.periods) && plan.periods.length > 0), [plans]);
  const firstPlan = safePlans[0];
  const initial = findPlan(plans, initialPlan) ?? firstPlan;
  const [planCode, setPlanCode] = useState(initial?.code ?? "premium");
  const currentPlan = safePlans.find((plan) => plan.code === planCode) ?? firstPlan;
  const initialPeriod = currentPlan?.periods.find((period) => period.id === initialPlan) ?? currentPlan?.periods[0];
  const [periodId, setPeriodId] = useState<PlanId>(initialPeriod?.id ?? "basic");

  const period = useMemo(() => {
    const selectedPlan = safePlans.find((plan) => plan.code === planCode) ?? firstPlan;
    return selectedPlan?.periods.find((item) => item.id === periodId) ?? selectedPlan?.periods[0];
  }, [firstPlan, periodId, planCode, safePlans]);

  if (!currentPlan || !period) return null;

  const isPayment = mode === "payment";
  const buttonText = loadingPlan === period.id ? "Создаем платеж..." : isPayment ? "Оплатить" : "Продолжить";

  return (
    <motion.div
      className="plan-chooser"
      initial={reduceMotion ? false : { opacity: 0, y: 16 }}
      transition={{ duration: 0.28, ease: "easeOut" }}
      viewport={{ once: true, amount: 0.2 }}
      whileInView={{ opacity: 1, y: 0 }}
    >
      <div className="plan-switch" role="tablist" aria-label="Тариф">
        {safePlans.map((plan) => {
          const Icon = plan.code === "vip" ? Crown : ShieldCheck;
          return (
            <button
              aria-selected={plan.code === planCode}
              className={plan.code === planCode ? "active" : ""}
              key={plan.code}
              onClick={() => {
                setPlanCode(plan.code);
                setPeriodId(plan.periods[0]?.id ?? "basic");
              }}
              role="tab"
              type="button"
            >
              <Icon size={18} />
              <span>{plan.title}</span>
            </button>
          );
        })}
      </div>

      <div className="plan-chooser-body">
        <div className="plan-chooser-copy">
          <h3>{currentPlan.title}</h3>
          <p>{currentPlan.description}</p>
          <ul className="feature-list compact">
            {(currentPlan.features ?? []).map((feature) => (
              <li key={feature}>
                <Check size={15} /> {feature}
              </li>
            ))}
          </ul>
        </div>

        <div className="period-picker" aria-label="Срок подписки">
          {currentPlan.periods.map((item) => (
            <button
              className={item.id === period.id ? "period-option active" : "period-option"}
              key={item.id}
              onClick={() => setPeriodId(item.id)}
              type="button"
            >
              <span>{item.label}</span>
              <strong>{formatRub(item.amount)}</strong>
            </button>
          ))}
        </div>

        <div className="plan-checkout">
          <span>Итого</span>
          <strong>{formatRub(period.amount)}</strong>
          {isPayment ? (
            <button
              className="button button-primary"
              disabled={loadingPlan !== ""}
              onClick={() => onSelect?.(period.id)}
              type="button"
            >
              {buttonText}
            </button>
          ) : (
            <a className="button button-primary" href={`/auth?plan=${period.id}`}>
              {buttonText}
            </a>
          )}
        </div>
      </div>
    </motion.div>
  );
}
