export type Role =
  | "STUDENT"
  | "PARENT"
  | "TEACHER"
  | "SCHOOL_ADMIN"
  | "ADMIN"
  | "CONTENT_EDITOR";

export interface Principal {
  id: string;
  role: Role;
  displayName: string;
  grade?: number;
  preferredLocale: "en" | "id";
  timezone: string;
  otpEnabled?: boolean;
  avatarItemId?: string;
  totalSp?: number;
  astroCoins?: number;
  pilotLevel?: number;
  currentStreak?: number;
}

export interface LiveFeedItem {
  id: string;
  maskedName: string;
  grade: number;
  avatarId: string;
  points: number;
  sourceLabel: string;
  submittedAt: string;
}

export class ApiError extends Error {
  status: number;
  detail?: string;

  constructor(status: number, message: string, detail?: string) {
    super(message);
    this.status = status;
    this.detail = detail;
  }
}

export function createApiClient(baseUrl: string) {
  const cleanBase = baseUrl.replace(/\/$/, "");

  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await fetch(`${cleanBase}${path}`, {
      ...init,
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...(init?.headers ?? {}),
      },
    });
    if (!res.ok) {
      let detail: string | undefined;
      try {
        const body = (await res.json()) as { detail?: string; title?: string };
        detail = body.detail ?? body.title;
      } catch {
        // ignore non-json errors
      }
      throw new ApiError(res.status, detail ?? `HTTP ${res.status}`, detail);
    }
    return (await res.json()) as T;
  }

  return {
    login(identifier: string, password: string) {
      return request<{ user: Principal }>("/api/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ identifier, password }),
      });
    },
    me() {
      return request<{ user: Principal }>("/api/v1/auth/me", { method: "GET" });
    },
    refresh() {
      return request<{ user: Principal }>("/api/v1/auth/refresh", { method: "POST" });
    },
    logout() {
      return request<{ ok: boolean }>("/api/v1/auth/logout", { method: "POST" });
    },
    async getLiveFeed() {
      const res = await request<{ items?: LiveFeedItem[] | null }>("/api/v1/auth/live-feed", {
        method: "GET",
      });
      return { items: res.items ?? [] };
    },
    requestParentOtp() {
      return request<{ sent: boolean; devCode?: string }>("/api/v1/auth/otp/request", {
        method: "POST",
      });
    },
    verifyParentOtp(code: string) {
      return request<{ verified: boolean }>("/api/v1/auth/otp/verify", {
        method: "POST",
        body: JSON.stringify({ code }),
      });
    },
  };
}
