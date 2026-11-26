import type { ButtonHTMLAttributes, ReactNode } from "react";

export type GalaxyButtonVariant = "primary" | "secondary" | "gold" | "ghost" | "danger";

export interface GalaxyButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: GalaxyButtonVariant;
  size?: "md" | "lg";
  children: ReactNode;
}

const VARIANT_CLASSES: Record<GalaxyButtonVariant, string> = {
  primary:
    "bg-[#6366F1] hover:bg-[#5558E6] text-white shadow-lg shadow-indigo-500/25 border border-indigo-400/30",
  secondary:
    "bg-[#1E2A55] hover:bg-[#2A3A73] text-slate-100 border border-indigo-300/20",
  gold:
    "bg-[#FBBF24] hover:bg-[#F59E0B] text-slate-950 font-bold shadow-lg shadow-amber-400/20",
  ghost:
    "bg-transparent hover:bg-white/10 text-slate-200 border border-transparent",
  danger:
    "bg-[#EF4444] hover:bg-[#DC2626] text-white border border-red-400/30",
};

const SIZE_CLASSES = {
  // Minimum 44px touch target enforced for elementary kids (Grades 1-6)
  md: "min-h-11 px-4 py-2 text-sm rounded-xl",
  lg: "min-h-12 px-6 py-3 text-base rounded-2xl",
};

export function GalaxyButton({
  variant = "primary",
  size = "md",
  className = "",
  type = "button",
  children,
  ...props
}: GalaxyButtonProps) {
  return (
    <button
      type={type}
      className={`inline-flex items-center justify-center gap-2 font-semibold transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#22D3EE] disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer ${VARIANT_CLASSES[variant]} ${SIZE_CLASSES[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}
