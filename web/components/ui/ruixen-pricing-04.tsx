"use client";

import { cn } from "@/lib/utils";
import NumberFlow from "@number-flow/react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { CheckIcon, Crown, ShieldCheck } from "lucide-react";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import type { Plan, PlanId } from "@/lib/site-config";

type BillingPeriod = "monthly" | "quarterly";

type CurrentSubscription = {
  plan: string;
  status: "active" | "expired" | "cancelled" | string;
};

type Pricing04Props = {
  plans: Plan[];
  initialPlan?: PlanId | null;
  loadingPlan?: PlanId | "";
  mode?: "link" | "payment";
  onSelect?: (plan: PlanId) => void;
  currentSubscription?: CurrentSubscription | null;
};

function findPlan(plans: Plan[], planId?: PlanId | null): Plan | undefined {
  return plans.find((plan) => Array.isArray(plan.periods) && plan.periods.some((period) => period.id === planId));
}

function billingPeriodForPlan(plan: Plan | undefined, planId?: PlanId | null): BillingPeriod {
  if (!planId || !plan) return "monthly";
  return plan.periods[1]?.id === planId ? "quarterly" : "monthly";
}

function planCodeOf(periodId: string): "premium" | "vip" | null {
  if (periodId === "basic" || periodId === "basic_3m") return "premium";
  if (periodId === "vip" || periodId === "vip_3m") return "vip";
  return null;
}

function quarterlySavings(plan: Plan): { percent: number; absolute: number; monthly: number } | null {
  const monthly = plan.periods[0];
  const quarterly = plan.periods[1];
  if (!monthly || !quarterly) return null;

  const baseline = monthly.amount * 3;
  const absolute = Math.max(0, baseline - quarterly.amount);
  if (absolute <= 0) return null;

  const percent = Math.round((absolute / baseline) * 100);
  const monthlyEquivalent = Math.round(quarterly.amount / 3);
  return { percent, absolute, monthly: monthlyEquivalent };
}

function maxQuarterlyDiscountPercent(plans: Plan[]): number {
  return plans.reduce((max, plan) => {
    const savings = quarterlySavings(plan);
    return savings && savings.percent > max ? savings.percent : max;
  }, 0);
}

type ButtonState = {
  label: string;
  disabled: boolean;
  reason?: string;
};

function resolveButtonState({
  period,
  plan,
  loadingPlan,
  mode,
  currentSubscription,
}: {
  period: { id: PlanId };
  plan: Plan;
  loadingPlan: PlanId | "";
  mode: "link" | "payment";
  currentSubscription?: CurrentSubscription | null;
}): ButtonState {
  if (loadingPlan === period.id) {
    return { label: "Создаем платеж...", disabled: true };
  }

  const defaultLabel = mode === "payment" ? "Оплатить" : "Продолжить";

  const activeSub = currentSubscription && currentSubscription.status === "active" ? currentSubscription : null;
  if (!activeSub) {
    return { label: defaultLabel, disabled: loadingPlan !== "" };
  }

  const currentCode = planCodeOf(activeSub.plan);
  if (!currentCode) {
    return { label: defaultLabel, disabled: loadingPlan !== "" };
  }

  if (activeSub.plan === period.id) {
    return { label: "Текущий тариф", disabled: true, reason: "current" };
  }

  if (plan.code === currentCode) {
    return {
      label: mode === "payment" ? "Продлить" : "Продлить",
      disabled: loadingPlan !== "",
    };
  }

  if (currentCode === "vip" && plan.code === "premium") {
    return { label: "VIP активен", disabled: true, reason: "downgrade" };
  }

  if (currentCode === "premium" && plan.code === "vip") {
    return { label: "Перейти на VIP", disabled: loadingPlan !== "" };
  }

  return { label: defaultLabel, disabled: loadingPlan !== "" };
}

export default function Pricing04({
  plans,
  initialPlan,
  loadingPlan = "",
  mode = "link",
  onSelect,
  currentSubscription,
}: Pricing04Props) {
  const reduceMotion = useReducedMotion();
  const safePlans = useMemo(() => plans.filter((plan) => Array.isArray(plan.periods) && plan.periods.length > 0), [plans]);
  const firstPlan = safePlans[0];
  const initial = findPlan(safePlans, initialPlan) ?? firstPlan;
  const [billingPeriod, setBillingPeriod] = useState<BillingPeriod>(billingPeriodForPlan(initial, initialPlan));

  if (!firstPlan) return null;

  const handleSwitch = () => {
    setBillingPeriod((prev) => (prev === "monthly" ? "quarterly" : "monthly"));
  };

  const maxDiscount = maxQuarterlyDiscountPercent(safePlans);

  return (
    <div className="pricing-04 relative mx-auto flex max-w-5xl flex-col items-center justify-center py-10">
      <div className="mx-auto flex max-w-2xl flex-col items-center justify-center">
        <div className="mx-auto flex max-w-2xl flex-col items-center text-center">
          <h2 className="mt-6 text-3xl font-bold md:text-4xl lg:text-5xl">
            Выберите подписку
          </h2>
          <p className="mt-6 text-center text-base text-accent-foreground/80 md:text-lg">
            Premium для ежедневного VPN или VIP для приоритетной сети. Дальше останется только оплатить и скачать приложение.
          </p>
        </div>
        <div className="mt-6 grid w-full max-w-xl grid-cols-[1fr_auto_1fr] items-center gap-4">
          <span className="justify-self-end text-base font-medium">1 месяц</span>
          <button
            aria-label="Переключить срок подписки"
            aria-pressed={billingPeriod === "quarterly"}
            className="relative justify-self-center rounded-full focus:outline-none"
            onClick={handleSwitch}
            type="button"
          >
            <div className="h-7 w-14 rounded-full bg-primary transition shadow-md outline-none" />
            <div
              className={cn(
                "absolute left-1 top-1 inline-flex size-5 items-center justify-center rounded-full bg-primary-foreground transition-all duration-300 ease-in-out",
                billingPeriod === "quarterly" ? "translate-x-7" : "translate-x-0",
              )}
            />
          </button>
          <span className="pricing-quarterly-label justify-self-start">
            3 месяца
            {maxDiscount > 0 && (
              <span className="pricing-quarterly-badge">−{maxDiscount}%</span>
            )}
          </span>
        </div>
      </div>

      <div className="mx-auto grid w-full max-w-4xl grid-cols-1 gap-4 pt-8 lg:grid-cols-2 lg:gap-6 lg:pt-12">
        {safePlans.map((plan) => (
          <PlanCard
            billingPeriod={billingPeriod}
            currentSubscription={currentSubscription}
            key={plan.code}
            loadingPlan={loadingPlan}
            mode={mode}
            onSelect={onSelect}
            plan={plan}
            reduceMotion={reduceMotion}
          />
        ))}
      </div>
    </div>
  );
}

function PlanCard({
  plan,
  billingPeriod,
  loadingPlan,
  mode,
  onSelect,
  reduceMotion,
  currentSubscription,
}: {
  plan: Plan;
  billingPeriod: BillingPeriod;
  loadingPlan: PlanId | "";
  mode: "link" | "payment";
  onSelect?: (plan: PlanId) => void;
  reduceMotion: boolean | null;
  currentSubscription?: CurrentSubscription | null;
}) {
  const period = billingPeriod === "quarterly" ? plan.periods[1] ?? plan.periods[0] : plan.periods[0];
  if (!period) return null;

  const isVip = plan.code === "vip";
  const Icon = isVip ? Crown : ShieldCheck;

  const buttonState = resolveButtonState({ period, plan, loadingPlan, mode, currentSubscription });
  const isCurrent = buttonState.reason === "current";

  const savings = billingPeriod === "quarterly" ? quarterlySavings(plan) : null;

  return (
    <motion.div
      className={cn(
        "pricing-card relative flex w-full flex-col items-start overflow-hidden rounded-2xl border border-foreground/10 transition-all lg:rounded-3xl",
        isVip && "pricing-card-featured",
        isCurrent && "pricing-card-current",
      )}
      initial={reduceMotion ? false : { opacity: 0, y: 18 }}
      transition={{ duration: 0.28, ease: "easeOut" }}
      whileHover={reduceMotion ? undefined : { y: -3 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.25 }}
    >
      {isVip && <div className="pricing-card-glow" aria-hidden="true" />}
      <div className="flex w-full flex-col items-start rounded-t-2xl p-4 md:p-8 lg:rounded-t-3xl">
        <div className="flex w-full items-center justify-between gap-3">
          <h3 className="pt-5 text-xl font-medium text-foreground">{plan.title}</h3>
          <span className="pricing-plan-icon">
            <Icon size={20} />
          </span>
        </div>
        {isCurrent && (
          <span className="pricing-current-badge" aria-label="Активная подписка">
            Активная подписка
          </span>
        )}
        <h4 className="mt-3 text-3xl font-bold md:text-5xl">
          <NumberFlow
            value={period.amount}
            suffix={billingPeriod === "monthly" ? "/мес" : "/3 мес"}
            format={{
              currency: "RUB",
              style: "currency",
              currencySign: "standard",
              minimumFractionDigits: 0,
              maximumFractionDigits: 0,
              currencyDisplay: "narrowSymbol",
            }}
          />
        </h4>
        {savings && (
          <div className="pricing-savings">
            <span className="pricing-savings-equivalent">≈ {savings.monthly} ₽/мес</span>
          </div>
        )}
        <p className="mt-2 text-sm text-muted-foreground md:text-base">
          {plan.description}
        </p>
      </div>
      <div className="flex w-full flex-col items-start px-4 py-2 md:px-8">
        {mode === "payment" ? (
          <Button
            className="w-full"
            disabled={buttonState.disabled}
            onClick={() => onSelect?.(period.id)}
            size="lg"
          >
            {buttonState.label}
          </Button>
        ) : (
          <Button asChild className="w-full" size="lg">
            <a href={`/auth?plan=${period.id}`}>{buttonState.label}</a>
          </Button>
        )}
        <div className="mx-auto h-8 w-full overflow-hidden">
          <AnimatePresence mode="wait">
            <motion.span
              animate={{ y: 0, opacity: 1 }}
              className="mx-auto mt-3 block text-center text-sm text-muted-foreground"
              exit={{ y: -20, opacity: 0 }}
              initial={{ y: 20, opacity: 0 }}
              key={billingPeriod}
              transition={{ duration: 0.2, ease: "easeOut" }}
            >
              {billingPeriod === "monthly"
                ? "Оплата за 1 месяц"
                : savings
                  ? `Оплата сразу за 3 месяца — −${savings.percent}%`
                  : "Оплата сразу за 3 месяца"}
            </motion.span>
          </AnimatePresence>
        </div>
      </div>
      <div className="mb-4 ml-1 flex w-full flex-col items-start gap-2 p-5">
        <span className="mb-2 text-left text-base">Включено:</span>
        {(plan.features ?? []).map((feature) => (
          <div className="flex items-center justify-start gap-2" key={feature}>
            <div className="flex items-center justify-center">
              <CheckIcon className="size-5" />
            </div>
            <span>{feature}</span>
          </div>
        ))}
      </div>
    </motion.div>
  );
}
