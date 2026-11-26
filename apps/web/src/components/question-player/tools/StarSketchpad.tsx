import { useRef, useState, type PointerEvent } from "react";
import { useTranslation } from "react-i18next";

type ToolMode = "pencil" | "highlighter" | "eraser";

const INKS = ["#111827", "#4F46E5", "#0EA5E9", "#DC2626"] as const;

export function StarSketchpad() {
  const { t } = useTranslation("player");
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const drawingRef = useRef(false);
  const [mode, setMode] = useState<ToolMode>("pencil");
  const [ink, setInk] = useState<string>(INKS[0]);

  function getPoint(e: PointerEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current;
    if (!canvas) return { x: 0, y: 0 };
    const rect = canvas.getBoundingClientRect();
    return {
      x: ((e.clientX - rect.left) / rect.width) * canvas.width,
      y: ((e.clientY - rect.top) / rect.height) * canvas.height,
    };
  }

  function handlePointerDown(e: PointerEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    canvas.setPointerCapture(e.pointerId);
    drawingRef.current = true;
    const pt = getPoint(e);
    ctx.beginPath();
    ctx.moveTo(pt.x, pt.y);
  }

  function handlePointerMove(e: PointerEvent<HTMLCanvasElement>) {
    if (!drawingRef.current) return;
    const ctx = canvasRef.current?.getContext("2d");
    if (!ctx) return;
    const pt = getPoint(e);

    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    if (mode === "eraser") {
      ctx.globalCompositeOperation = "destination-out";
      ctx.lineWidth = 18;
    } else if (mode === "highlighter") {
      ctx.globalCompositeOperation = "source-over";
      ctx.strokeStyle = "rgba(251, 191, 36, 0.35)";
      ctx.lineWidth = 14;
    } else {
      ctx.globalCompositeOperation = "source-over";
      ctx.strokeStyle = ink;
      ctx.lineWidth = 3;
    }
    ctx.lineTo(pt.x, pt.y);
    ctx.stroke();
  }

  function handlePointerUp() {
    drawingRef.current = false;
  }

  function clearCanvas() {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
  }

  return (
    <div className="space-y-2.5">
      <div className="flex flex-wrap items-center gap-1.5">
        {(["pencil", "highlighter", "eraser"] as const).map((m) => (
          <button
            key={m}
            type="button"
            onClick={() => setMode(m)}
            className={`min-h-10 rounded-lg px-2.5 text-xs font-semibold cursor-pointer ${
              mode === m
                ? "bg-[#6366F1] text-white"
                : "bg-slate-200 text-slate-800 hover:bg-slate-300"
            }`}
          >
            {t(`tools.${m}`)}
          </button>
        ))}
        <div className="flex items-center gap-1 ml-1">
          {INKS.map((c) => (
            <button
              key={c}
              type="button"
              aria-label={`Ink ${c}`}
              onClick={() => {
                setInk(c);
                setMode("pencil");
              }}
              className={`h-6 w-6 rounded-full border-2 cursor-pointer ${
                ink === c && mode === "pencil" ? "border-[#22D3EE] scale-110" : "border-white"
              }`}
              style={{ backgroundColor: c }}
            />
          ))}
        </div>
        <button
          type="button"
          onClick={clearCanvas}
          className="ml-auto min-h-10 rounded-lg bg-red-100 px-2.5 text-xs font-semibold text-red-700 hover:bg-red-200 cursor-pointer"
        >
          {t("tools.clear")}
        </button>
      </div>

      <canvas
        ref={canvasRef}
        width={420}
        height={260}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
        className="w-full h-56 rounded-xl border border-slate-300 bg-white touch-none cursor-crosshair"
        style={{
          backgroundImage: "radial-gradient(#cbd5e1 1px, transparent 1px)",
          backgroundSize: "16px 16px",
        }}
      />
    </div>
  );
}
