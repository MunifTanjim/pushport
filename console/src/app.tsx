import { useEffect, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Navigate, Route, Routes } from "react-router";
import { clearSession, getSession } from "./auth/token";
import { Layout } from "./components/layout";
import { ToastProvider } from "./components/toast";
import Login from "./pages/login";
import Overview from "./pages/overview";
import AppOverview from "./pages/app-overview";
import Apps from "./pages/apps";
import AppDetail from "./pages/app-detail";
import Plans from "./pages/plans";
import Settings from "./pages/settings";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 15_000 },
  },
});

function RequireAuth({ children }: { children: ReactNode }) {
  if (!getSession()) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function RequireAdmin({ children }: { children: ReactNode }) {
  if (getSession()?.principal !== "admin") return <Navigate to="/" replace />;
  return <>{children}</>;
}

function Home() {
  return getSession()?.principal === "app" ? <AppOverview /> : <Overview />;
}

export default function App() {
  // The API client fires pp:unauthorized on any 401 while a token is stored (e.g. rotated elsewhere).
  useEffect(() => {
    const onUnauthorized = () => {
      clearSession();
      queryClient.clear();
      if (!window.location.pathname.startsWith("/login")) {
        window.location.assign("/login");
      }
    };
    window.addEventListener("pp:unauthorized", onUnauthorized);
    return () => window.removeEventListener("pp:unauthorized", onUnauthorized);
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <BrowserRouter basename="/">
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route
              element={
                <RequireAuth>
                  <Layout />
                </RequireAuth>
              }
            >
              <Route index element={<Home />} />
              <Route
                path="/apps"
                element={
                  <RequireAdmin>
                    <Apps />
                  </RequireAdmin>
                }
              />
              <Route path="/apps/:appId" element={<AppDetail />} />
              <Route
                path="/plans"
                element={
                  <RequireAdmin>
                    <Plans />
                  </RequireAdmin>
                }
              />
              <Route
                path="/settings"
                element={
                  <RequireAdmin>
                    <Settings />
                  </RequireAdmin>
                }
              />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </ToastProvider>
    </QueryClientProvider>
  );
}
