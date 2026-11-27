import type { HTMLAttributes, ReactNode } from "react";

export interface GalaxyCardProps extends HTMLAttributes<HTMLDivElement> {
  surface?: "space" | "paper";
  children: ReactNode;
}

export function GalaxyCard({
  surface = "space",
  className = "",
  children,
  ...props
}: GalaxyCardProps) {
  const surfaceClass =
    surface === "paper"
      ? "bg-[#FAFBFD] text-[#111827] border border-slate-200 shadow-lg"
      : "bg-[#131B3A]/90 text-slate-100 border border-indigo-400/20 shadow-xl backdrop-blur";

  return (
    <div className={`rounded-2xl p-5 ${surfaceClass} ${className}`} {...props}>
      {children}
    </div>
  );
}
