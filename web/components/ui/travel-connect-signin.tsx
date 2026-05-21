"use client";

import type React from "react";
import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowRight, Download, LockKeyhole, MailCheck } from "lucide-react";
import { motion, useReducedMotion } from "framer-motion";
import { cn } from "@/lib/utils";

type RoutePoint = {
  x: number;
  y: number;
  delay: number;
};

type Route = {
  start: RoutePoint;
  end: RoutePoint;
};

const routes: Route[] = [
  { start: { x: 0.16, y: 0.34, delay: 0 }, end: { x: 0.38, y: 0.2, delay: 1.2 } },
  { start: { x: 0.38, y: 0.2, delay: 1.2 }, end: { x: 0.58, y: 0.36, delay: 2.4 } },
  { start: { x: 0.18, y: 0.72, delay: 0.7 }, end: { x: 0.48, y: 0.54, delay: 2 } },
  { start: { x: 0.7, y: 0.24, delay: 0.4 }, end: { x: 0.52, y: 0.7, delay: 2.8 } },
];

function hash2d(x: number, y: number) {
  const value = Math.sin(x * 12.9898 + y * 78.233) * 43758.5453;
  return value - Math.floor(value);
}

function DotMap() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });

  const dots = useMemo(() => {
    if (!dimensions.width || !dimensions.height) return [];
    const items: Array<{ x: number; y: number; opacity: number }> = [];
    const gap = 12;

    for (let x = 0; x < dimensions.width; x += gap) {
      for (let y = 0; y < dimensions.height; y += gap) {
        const nx = x / dimensions.width;
        const ny = y / dimensions.height;
        const isInMapShape =
          (nx > 0.06 && nx < 0.28 && ny > 0.12 && ny < 0.44) ||
          (nx > 0.16 && nx < 0.28 && ny > 0.42 && ny < 0.82) ||
          (nx > 0.32 && nx < 0.48 && ny > 0.16 && ny < 0.38) ||
          (nx > 0.38 && nx < 0.54 && ny > 0.36 && ny < 0.68) ||
          (nx > 0.48 && nx < 0.82 && ny > 0.12 && ny < 0.52) ||
          (nx > 0.68 && nx < 0.86 && ny > 0.62 && ny < 0.82);

        if (isInMapShape && hash2d(x, y) > 0.26) {
          items.push({ x, y, opacity: 0.12 + hash2d(y, x) * 0.34 });
        }
      }
    }

    return items;
  }, [dimensions]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const parent = canvas?.parentElement;
    if (!canvas || !parent) return;

    const resizeObserver = new ResizeObserver((entries) => {
      const { width, height } = entries[0].contentRect;
      const nextWidth = Math.max(1, Math.floor(width));
      const nextHeight = Math.max(1, Math.floor(height));
      setDimensions({ width: nextWidth, height: nextHeight });
      canvas.width = nextWidth;
      canvas.height = nextHeight;
    });

    resizeObserver.observe(parent);
    return () => resizeObserver.disconnect();
  }, []);

  useEffect(() => {
    if (!dimensions.width || !dimensions.height) return;

    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    let animationFrameId = 0;
    let startTime = Date.now();

    const point = (routePoint: RoutePoint) => ({
      x: routePoint.x * dimensions.width,
      y: routePoint.y * dimensions.height,
    });

    const draw = () => {
      ctx.clearRect(0, 0, dimensions.width, dimensions.height);

      dots.forEach((dot) => {
        ctx.beginPath();
        ctx.arc(dot.x, dot.y, 1, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(234, 179, 8, ${dot.opacity})`;
        ctx.fill();
      });

      const currentTime = (Date.now() - startTime) / 1000;
      routes.forEach((route) => {
        const elapsed = currentTime - route.start.delay;
        if (elapsed <= 0) return;

        const progress = Math.min(elapsed / 3, 1);
        const start = point(route.start);
        const end = point(route.end);
        const x = start.x + (end.x - start.x) * progress;
        const y = start.y + (end.y - start.y) * progress;

        ctx.beginPath();
        ctx.moveTo(start.x, start.y);
        ctx.lineTo(x, y);
        ctx.strokeStyle = "rgba(234, 179, 8, 0.62)";
        ctx.lineWidth = 1.5;
        ctx.stroke();

        ctx.beginPath();
        ctx.arc(start.x, start.y, 3, 0, Math.PI * 2);
        ctx.fillStyle = "#EAB308";
        ctx.fill();

        ctx.beginPath();
        ctx.arc(x, y, 6, 0, Math.PI * 2);
        ctx.fillStyle = "rgba(234, 179, 8, 0.22)";
        ctx.fill();

        ctx.beginPath();
        ctx.arc(x, y, 3, 0, Math.PI * 2);
        ctx.fillStyle = "#FACC15";
        ctx.fill();

        if (progress === 1) {
          ctx.beginPath();
          ctx.arc(end.x, end.y, 3, 0, Math.PI * 2);
          ctx.fillStyle = "#EAB308";
          ctx.fill();
        }
      });

      if (currentTime > 12) startTime = Date.now();
      animationFrameId = requestAnimationFrame(draw);
    };

    draw();
    return () => cancelAnimationFrame(animationFrameId);
  }, [dimensions, dots]);

  return <canvas aria-hidden="true" className="auth-map-canvas" ref={canvasRef} />;
}

export default function TravelConnectSignIn({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const reduceMotion = useReducedMotion();

  return (
    <motion.section
      animate={{ opacity: 1, y: 0 }}
      className={cn("auth-card auth-card-modern", className)}
      initial={reduceMotion ? false : { opacity: 0, y: 16 }}
      transition={{ duration: 0.32, ease: "easeOut" }}
    >
      <aside className="auth-map-panel" aria-label="FBLink VPN">
        <DotMap />
        <div className="auth-map-content">
          <motion.div
            animate={{ opacity: 1, y: 0 }}
            className="auth-map-mark"
            initial={reduceMotion ? false : { opacity: 0, y: -12 }}
            transition={{ delay: 0.12, duration: 0.32 }}
          >
            <ArrowRight size={22} />
          </motion.div>
          <motion.h2
            animate={{ opacity: 1, y: 0 }}
            initial={reduceMotion ? false : { opacity: 0, y: -10 }}
            transition={{ delay: 0.18, duration: 0.32 }}
          >
            FBLink VPN
          </motion.h2>
          <motion.p
            animate={{ opacity: 1, y: 0 }}
            initial={reduceMotion ? false : { opacity: 0, y: -10 }}
            transition={{ delay: 0.24, duration: 0.32 }}
          >
            Один кабинет для оплаты, продления и скачивания приложений.
          </motion.p>
          <div className="auth-map-benefits">
            <span>
              <Download size={16} /> Приложения рядом
            </span>
            <span>
              <LockKeyhole size={16} /> HttpOnly cookies
            </span>
            <span>
              <MailCheck size={16} /> Email-код
            </span>
          </div>
        </div>
      </aside>
      {children}
    </motion.section>
  );
}
