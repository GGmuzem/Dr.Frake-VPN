"use client";

import { cn } from "@/lib/utils";
import NumberFlow from "@number-flow/react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { CheckIcon, Crown, ShieldCheck } from "lucide-react";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import type { Plan, PlanId } from "@/lib/site-config";
import { PromoCodeInput } from "@/components/PromoCodeInput";

type BillingPeriod = "monthly" | "quarterly";

type CurrentSubscription = {
  plan: string;
  status: "active" | "expired" | "cancelled" | string;
};

type Pricing04Props = {
  plans: Plan[];
  initialPlan?: PlanId | null;
  initialPromoCode?: string;
  loadingPlan?: PlanId | "";
  mode?: "link" | "payment";
  onSelect?: (plan: PlanId, promoCode?: string) => void;
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

  const defaultLabel = mode === "payment" ? "Выбрать" : "Продолжить";

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
      label: "Продлить",
      disabled: loadingPlan !== "",
    };
  }

  if (currentCode === "vip" && plan.code === "premium") {
    return { label: "VIP подключен", disabled: true, reason: "downgrade" };
  }

  if (currentCode === "premium" && plan.code === "vip") {
    return { label: "Перейти на VIP", disabled: loadingPlan !== "" };
  }

  return { label: defaultLabel, disabled: loadingPlan !== "" };
}

export default function Pricing04({
  plans,
  initialPlan,
  initialPromoCode = "",
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
  const [selectedPlanId, setSelectedPlanId] = useState<PlanId | null>(initialPlan ?? null);
  const [promoCode, setPromoCode] = useState<string>(initialPromoCode);
  const [promoDetails, setPromoDetails] = useState<{
    discountPercent: number;
    originalAmount: number;
    finalAmount: number;
    discountAmount: number;
  } | null>(null);

  if (!firstPlan) return null;

  const selectedPeriodAndPlan = useMemo(() => {
    if (!selectedPlanId) return null;
    for (const plan of safePlans) {
      const period = plan.periods.find((p) => p.id === selectedPlanId);
      if (period) return { plan, period };
    }
    return null;
  }, [safePlans, selectedPlanId]);

  const handleSwitch = () => {
    const nextPeriod = billingPeriod === "monthly" ? "quarterly" : "monthly";
    setBillingPeriod(nextPeriod);

    if (selectedPlanId && selectedPeriodAndPlan) {
      const currentPlan = selectedPeriodAndPlan.plan;
      const targetPeriodIndex = nextPeriod === "quarterly" ? 1 : 0;
      const targetPeriod = currentPlan.periods[targetPeriodIndex] ?? currentPlan.periods[0];
      setSelectedPlanId(targetPeriod.id);
    }
  };

  const maxDiscount = maxQuarterlyDiscountPercent(safePlans);

  return (
    <div className="pricing-04 relative mx-auto flex w-full max-w-5xl flex-col items-center justify-center py-10">
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
            className="relative justify-self-center rounded-full focus:outline-none cursor-pointer"
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
            selectedPlanId={selectedPlanId}
            onSelectPlanId={(id) => {
              if (mode === "payment") {
                setSelectedPlanId(id);
              }
            }}
          />
        ))}
      </div>

      <AnimatePresence mode="wait">
        {mode === "payment" && selectedPeriodAndPlan && (
          <motion.div
            initial={{ opacity: 0, height: 0, y: 30 }}
            animate={{ opacity: 1, height: "auto", y: 0 }}
            exit={{ opacity: 0, height: 0, y: 30 }}
            transition={{ type: "spring", stiffness: 350, damping: 30 }}
            className="w-full max-w-xl mx-auto mt-12 overflow-hidden"
          >
            <div className="relative rounded-2xl border border-primary/30 bg-[rgba(17,17,17,0.7)] p-6 md:p-8 backdrop-blur-md shadow-2xl">
              {/* Subtle top glow */}
              <div className="absolute -top-12 left-1/2 -translate-x-1/2 w-48 h-12 bg-primary/20 rounded-full blur-2xl pointer-events-none" />
              
              <h3 className="text-xl font-bold text-foreground mb-6 flex items-center gap-2">
                <span>Оформление заказа</span>
              </h3>
              
              {/* Selected plan details */}
              <div className="flex justify-between items-center pb-4 border-b border-foreground/10">
                <div>
                  <h4 className="font-semibold text-lg">
                    {selectedPeriodAndPlan.plan.title}
                  </h4>
                  <p className="text-sm text-muted-foreground">
                    Тарифный план • {selectedPeriodAndPlan.period.label}
                  </p>
                </div>
                <div className="text-right">
                  <div className="text-sm text-muted-foreground">Базовая цена</div>
                  <div className="font-medium text-foreground">
                    {selectedPeriodAndPlan.period.amount} ₽
                  </div>
                </div>
              </div>
              
              {/* Promo input field */}
              <div className="py-6 border-b border-foreground/10">
                <PromoCodeInput
                  key={selectedPlanId}
                  previewPlan={selectedPlanId ?? undefined}
                  initialCode={promoCode}
                  onConfirm={(code, details) => {
                    setPromoCode(code);
                    if (details) {
                      setPromoDetails(details);
                    } else {
                      setPromoDetails(null);
                    }
                  }}
                />
              </div>

              {/* Price Breakdown */}
              <div className="py-4 space-y-2">
                {promoDetails && promoDetails.discountAmount > 0 && (
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">
                      Скидка по промокоду ({promoDetails.discountPercent}%)
                    </span>
                    <span className="text-green-500 font-medium">
                      -{promoDetails.discountAmount} ₽
                    </span>
                  </div>
                )}
              </div>

              {/* Total Summary & Pay CTA */}
              <div className="pt-4 flex flex-col sm:flex-row items-center justify-between gap-4 mt-2">
                <div>
                  <div className="text-sm text-muted-foreground">Итого к оплате:</div>
                  <div className="text-3xl font-extrabold text-foreground tracking-tight flex items-baseline gap-2">
                    {promoDetails ? (
                      <>
                        <span className="text-sm line-through text-muted-foreground font-normal">
                          {promoDetails.originalAmount} ₽
                        </span>
                        <NumberFlow
                          value={promoDetails.finalAmount}
                          format={{
                            style: "currency",
                            currency: "RUB",
                            minimumFractionDigits: 0,
                            maximumFractionDigits: 0,
                            currencyDisplay: "narrowSymbol",
                          }}
                        />
                      </>
                    ) : (
                      <NumberFlow
                        value={selectedPeriodAndPlan.period.amount}
                        format={{
                          style: "currency",
                          currency: "RUB",
                          minimumFractionDigits: 0,
                          maximumFractionDigits: 0,
                          currencyDisplay: "narrowSymbol",
                        }}
                      />
                    )}
                  </div>
                </div>

                <Button
                  size="lg"
                  className="w-full sm:w-auto px-10 bg-primary text-primary-foreground hover:bg-primary/90 font-bold transition-all duration-300 shadow-lg shadow-primary/20 cursor-pointer"
                  disabled={loadingPlan !== ""}
                  onClick={() => {
                    if (selectedPlanId) {
                      onSelect?.(selectedPlanId, promoCode);
                    }
                  }}
                >
                  {loadingPlan === selectedPlanId ? "Создаем платеж..." : "Оплатить"}
                </Button>
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
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
  selectedPlanId,
  onSelectPlanId,
}: {
  plan: Plan;
  billingPeriod: BillingPeriod;
  loadingPlan: PlanId | "";
  mode: "link" | "payment";
  onSelect?: (plan: PlanId) => void;
  reduceMotion: boolean | null;
  currentSubscription?: CurrentSubscription | null;
  selectedPlanId: PlanId | null;
  onSelectPlanId?: (id: PlanId) => void;
}) {
  const period = billingPeriod === "quarterly" ? plan.periods[1] ?? plan.periods[0] : plan.periods[0];
  if (!period) return null;

  const isVip = plan.code === "vip";
  const Icon = isVip ? Crown : ShieldCheck;

  const buttonState = resolveButtonState({ period, plan, loadingPlan, mode, currentSubscription });
  const isCurrent = buttonState.reason === "current";
  const isSelected = selectedPlanId === period.id;

  const savings = billingPeriod === "quarterly" ? quarterlySavings(plan) : null;

  return (
    <motion.div
      className={cn(
        "pricing-card relative flex w-full flex-col items-start overflow-hidden rounded-2xl border border-foreground/10 transition-all duration-300 lg:rounded-3xl",
        isVip && "pricing-card-featured",
        isCurrent && "pricing-card-current",
        isSelected && "border-primary ring-2 ring-primary bg-primary/5 shadow-[0_0_30px_rgba(234,179,8,0.15)]",
        (!buttonState.disabled && mode === "payment") ? "cursor-pointer" : "cursor-default"
      )}
      initial={reduceMotion ? false : { opacity: 0, y: 18 }}
      transition={{ duration: 0.28, ease: "easeOut" }}
      whileHover={reduceMotion ? undefined : (isCurrent || isSelected) ? undefined : { y: -3 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.25 }}
      onClick={() => {
        if (!buttonState.disabled && mode === "payment" && onSelectPlanId) {
          onSelectPlanId(period.id);
        }
      }}
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
            className={cn(
              "w-full transition-all duration-200 cursor-pointer",
              isSelected 
                ? "bg-primary text-primary-foreground font-semibold"
                : "bg-primary/10 hover:bg-primary/20 text-foreground border border-foreground/10"
            )}
            disabled={buttonState.disabled}
            onClick={(e) => {
              e.stopPropagation();
              if (onSelectPlanId) onSelectPlanId(period.id);
            }}
            size="lg"
          >
            {isSelected ? "Выбран" : "Выбрать"}
          </Button>
        ) : (
          <Button asChild className="w-full cursor-pointer" size="lg">
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
