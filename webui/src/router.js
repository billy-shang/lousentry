import { createRouter, createWebHistory } from "vue-router";
import { getStore } from "./storage";
import Login from "./views/Login.vue";
import Layout from "./views/Layout.vue";
import Home from "./views/Home.vue";
import Logs from "./views/Logs.vue";
import Settings from "./views/Settings.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", component: Login },
    {
      path: "/",
      component: Layout,
      children: [
        { path: "", component: Home },
        { path: "logs", component: Logs },
        { path: "settings", component: Settings },
      ],
    },
  ],
});

router.beforeEach((to) => {
  const token = getStore("token");
  if (to.path !== "/login" && !token) return "/login";
  if (to.path === "/login" && token) return "/";
  return true;
});

export default router;
