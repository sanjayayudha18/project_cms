import { Outlet, createRoute } from "@tanstack/react-router";
import { useState } from "react";
import { rootRoute } from "./__root";

export const authRoute = createRoute({
  id: "_auth",
  getParentRoute: () => rootRoute,
  component: AuthLayout,
});

const ARTWORK_SRC = "/assets/crown-login-artwork.png";
const ARTWORK_ALT =
  "Ilustrasi operasional kas: mesin ATM, kendaraan cash-in-transit, kontainer uang, lembaran uang, dan diagram rute pengiriman";

function AuthLayout() {
  const [artworkState, setArtworkState] = useState<"loading" | "ready" | "error">("loading");

  return (
    <div className="login-shell" data-testid="login-shell">
      <section className="login-shell__identity" aria-label="Formulir masuk Company Portal">
        <div className="login-shell__identity-inner">
          <header className="login-shell__brand">
            <div className="login-shell__mark" aria-hidden="true">
              <svg
                width="22"
                height="22"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                role="img"
                aria-label="CMS"
              >
                <title>CMS</title>
                <rect x="3" y="3" width="7" height="7" rx="1" />
                <rect x="14" y="3" width="7" height="7" rx="1" />
                <rect x="3" y="14" width="7" height="7" rx="1" />
                <rect x="14" y="14" width="7" height="7" rx="1" />
              </svg>
            </div>
            <div className="login-shell__brand-text">
              <span className="login-shell__brand-name">CMS</span>
              <span className="login-shell__brand-divider" aria-hidden="true" />
              <span className="login-shell__brand-sub">Company Portal</span>
            </div>
          </header>

          <p className="login-shell__lead">
            Kelola operasional kas ATM secara terintegrasi, dari peramalan hingga rekonsiliasi.
          </p>
          <p className="login-shell__product">Cash Management System</p>

          <div className="login-shell__outlet">
            <Outlet />
          </div>

          <footer className="login-shell__footer">
            <p className="login-shell__footer-note">Akses diotorisasi dan dicatat.</p>
            <p className="login-shell__footer-copy">© 2026 CIMB Niaga STCC</p>
          </footer>
        </div>
      </section>

      <aside
        className={`login-shell__artwork login-shell__artwork--${artworkState}`}
        aria-label="Panel visual operasional kas"
      >
        {artworkState !== "error" && (
          <img
            className="login-shell__artwork-img"
            src={ARTWORK_SRC}
            alt={ARTWORK_ALT}
            data-testid="login-artwork"
            onLoad={() => setArtworkState("ready")}
            onError={() => setArtworkState("error")}
          />
        )}
        {artworkState === "loading" && (
          <div className="login-shell__artwork-skeleton" aria-hidden="true" />
        )}
        <div className="login-shell__artwork-scrim">
          <p className="login-shell__artwork-caption">
            Operasional ATM dan cash-in-transit dalam satu sistem manajemen kas.
          </p>
        </div>
      </aside>
    </div>
  );
}
