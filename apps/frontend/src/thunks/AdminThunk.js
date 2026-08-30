import axios from "axios";
import { getAuth } from "../stores/AuthStore";

const normalizeHost = (host) => {
  const value = String(host ?? "").trim().replace(/\/+$/, "");
  if (!value) return null;
  if (/^https?:\/\//i.test(value)) return value;
  return `${/^(localhost|127\.0\.0\.1|\[::1\])(?::\d+)?$/i.test(value) ? "http" : "https"}://${value}`;
};

const adminHost = normalizeHost(APP_ADMIN_SERVER_HOST);
const admin = adminHost
  ? axios.create({ baseURL: `${adminHost}/v1/admin`, timeout: 10000 })
  : null;

admin?.interceptors.request.use((config) => {
  const token = getAuth()?.accessToken;
  if (token) {
    config.headers = config.headers ?? {};
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

const requireAdminClient = () => {
  if (!admin) throw new Error("관리자 서버 주소가 설정되지 않았습니다.");
  return admin;
};

export const getAdminSession = async () => (await requireAdminClient().get("/session")).data;
export const getAdminOverview = async () => (await requireAdminClient().get("/overview")).data;
export const getAdminEvents = async (limit = 100) =>
  (await requireAdminClient().get("/events", { params: { limit } })).data;
