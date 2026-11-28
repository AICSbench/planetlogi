import type { AstroBuddyConfig } from "@planetlogi/api-client";
import { useTranslation } from "react-i18next";

interface AstroBuddyAvatarProps {
  config: Partial<AstroBuddyConfig>;
  size?: number;
  className?: string;
}

export function AstroBuddyAvatar({ config, size = 96, className = "" }: AstroBuddyAvatarProps) {
  const { t } = useTranslation("stories");
  const body = config.bodyShape ?? "round";
  const color = config.colorHex ?? "#6366F1";
  const horns = config.hornsStyle ?? "antennae";
  const eyes = config.eyesStyle ?? "bright";
  const mouth = config.mouthStyle ?? "smile";

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 120 120"
      className={className}
      role="img"
      aria-label={config.fullName?.trim() ? config.fullName : t("customization.buddyFallbackName")}
    >
      <defs>
        <radialGradient id={`glow-${color.replace("#", "")}`} cx="35%" cy="30%" r="70%">
          <stop offset="0%" stopColor="#FFFFFF" stopOpacity="0.45" />
          <stop offset="100%" stopColor={color} stopOpacity="1" />
        </radialGradient>
      </defs>

      {/* Cosmic aura ring */}
      <circle cx="60" cy="64" r="46" fill={color} fillOpacity="0.16" />

      {/* Horns / Ears / Halo */}
      {horns === "antennae" ? (
        <g stroke="#22D3EE" strokeWidth="4" strokeLinecap="round">
          <line x1="45" y1="30" x2="36" y2="14" />
          <line x1="75" y1="30" x2="84" y2="14" />
          <circle cx="36" cy="12" r="5" fill="#FBBF24" stroke="none" />
          <circle cx="84" cy="12" r="5" fill="#FBBF24" stroke="none" />
        </g>
      ) : null}
      {horns === "cat_ears" ? (
        <g fill={color} stroke="#E2E8F0" strokeWidth="2.5">
          <polygon points="30,44 24,18 48,30" />
          <polygon points="90,44 96,18 72,30" />
        </g>
      ) : null}
      {horns === "comet_horns" ? (
        <g fill="#FBBF24">
          <path d="M36 34 Q24 14 44 20 Z" />
          <path d="M84 34 Q96 14 76 20 Z" />
        </g>
      ) : null}
      {horns === "halo" ? (
        <ellipse cx="60" cy="18" rx="26" ry="7" fill="none" stroke="#FBBF24" strokeWidth="4.5" />
      ) : null}

      {/* Body Shape */}
      {body === "round" ? (
        <circle cx="60" cy="66" r="36" fill={`url(#glow-${color.replace("#", "")})`} stroke="#FFFFFF" strokeOpacity="0.35" strokeWidth="2.5" />
      ) : null}
      {body === "cube" ? (
        <rect x="26" y="32" width="68" height="68" rx="18" fill={`url(#glow-${color.replace("#", "")})`} stroke="#FFFFFF" strokeOpacity="0.35" strokeWidth="2.5" />
      ) : null}
      {body === "pear" ? (
        <path
          d="M60 28 C80 28 96 54 94 78 C92 98 76 104 60 104 C44 104 28 98 26 78 C24 54 40 28 60 28 Z"
          fill={`url(#glow-${color.replace("#", "")})`}
          stroke="#FFFFFF"
          strokeOpacity="0.35"
          strokeWidth="2.5"
        />
      ) : null}
      {body === "star" ? (
        <polygon
          points="60,24 72,46 97,50 79,67 84,92 60,80 36,92 41,67 23,50 48,46"
          fill={`url(#glow-${color.replace("#", "")})`}
          stroke="#FFFFFF"
          strokeOpacity="0.4"
          strokeWidth="2.5"
          strokeLinejoin="round"
        />
      ) : null}

      {/* Eyes */}
      {eyes === "bright" ? (
        <g>
          <circle cx="47" cy="60" r="6.5" fill="#0B1026" />
          <circle cx="73" cy="60" r="6.5" fill="#0B1026" />
          <circle cx="49" cy="58" r="2.2" fill="#FFFFFF" />
          <circle cx="75" cy="58" r="2.2" fill="#FFFFFF" />
        </g>
      ) : null}
      {eyes === "starry" ? (
        <g fill="#FEF08A">
          <polygon points="47,52 49,58 55,58 50,62 52,68 47,64 42,68 44,62 39,58 45,58" />
          <polygon points="73,52 75,58 81,58 76,62 78,68 73,64 68,68 70,62 65,58 71,58" />
        </g>
      ) : null}
      {eyes === "cyclops" ? (
        <g>
          <circle cx="60" cy="58" r="12" fill="#FFFFFF" />
          <circle cx="60" cy="58" r="6.5" fill="#0B1026" />
          <circle cx="63" cy="55" r="2.5" fill="#22D3EE" />
        </g>
      ) : null}
      {eyes === "visor" ? (
        <g>
          <rect x="36" y="51" width="48" height="16" rx="8" fill="#0B1026" stroke="#22D3EE" strokeWidth="2.5" />
          <line x1="42" y1="57" x2="54" y2="57" stroke="#22D3EE" strokeWidth="2.5" strokeLinecap="round" />
        </g>
      ) : null}

      {/* Mouth */}
      {mouth === "smile" ? (
        <path d="M48 76 Q60 86 72 76" fill="none" stroke="#0B1026" strokeWidth="3.5" strokeLinecap="round" />
      ) : null}
      {mouth === "grin" ? (
        <path d="M46 74 Q60 88 74 74 Z" fill="#FFFFFF" stroke="#0B1026" strokeWidth="2.5" />
      ) : null}
      {mouth === "cat" ? (
        <path d="M48 77 Q54 83 60 77 Q66 83 72 77" fill="none" stroke="#0B1026" strokeWidth="3" strokeLinecap="round" />
      ) : null}
      {mouth === "whistle" ? (
        <circle cx="60" cy="79" r="5" fill="#0B1026" />
      ) : null}
    </svg>
  );
}
