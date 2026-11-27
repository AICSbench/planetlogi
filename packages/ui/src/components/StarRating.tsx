export interface StarRatingProps {
  stars: 0 | 1 | 2 | 3;
  label?: string;
}

export function StarRating({ stars, label }: StarRatingProps) {
  return (
    <span
      className="inline-flex items-center gap-0.5 text-base"
      role="img"
      aria-label={label ?? `${stars} of 3 stars`}
    >
      {[1, 2, 3].map((idx) => (
        <span
          key={idx}
          aria-hidden="true"
          className={idx <= stars ? "text-[#FBBF24]" : "text-slate-600"}
        >
          ★
        </span>
      ))}
    </span>
  );
}
