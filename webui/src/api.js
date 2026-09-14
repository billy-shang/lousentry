import axios from "axios";
import { getStore, removeStore } from "./storage";

const http = axios.create({
  baseURL: "/api",
  timeout: 30000,
});

http.interceptors.request.use((cfg) => {
  const token = getStore("token");
  if (token) {
    cfg.headers.Authorization = `Bearer ${token}`;
  }
  return cfg;
});

http.interceptors.response.use(
  (res) => res.data,
  (err) => {
    const status = err.response && err.response.status;
    if (status === 401) {
      removeStore("token");
      removeStore("user");
      if (!location.pathname.endsWith("/login")) {
        location.href = "/login";
      }
    }
    return Promise.reject(err);
  }
);

export function errMsg(e) {
  const d = e && e.response && e.response.data;
  if (!d) return (e && e.message) || "请求失败";
  return d.message || "请求失败";
}

export default http;
