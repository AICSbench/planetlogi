export type BrainZone =
  | "memory"
  | "flexibility"
  | "speed"
  | "problem_solving"
  | "adventure";

export interface BrainGameDescriptor {
  code: string;
  zone: BrainZone;
  titleKey: string;
  entryCostAc: number;
}

export const BRAIN_GALAXY_ZONES: readonly BrainZone[] = [
  "memory",
  "flexibility",
  "speed",
  "problem_solving",
  "adventure",
] as const;
