# T5.3 Session Expiry Real-Time Test

## Procedure
1. Start backend with 2-minute session lifetime:
   ```bash
   SESSION_MAX_LIFETIME=2m go run ./backend/cmd/api/main.go
   ```

2. Start frontend dev servers:
   ```bash
   cd frontend/CompanyPortal-Vite && npm run dev
   cd frontend/VendorPortal-Vite && npm run dev
   ```

3. Log in on both portals (use any valid dev credentials).

4. Wait 2 minutes without making any requests.

5. Verify automatic logout:
   - Attempt any action that requires auth (e.g., open a page that calls the API).
   - Expected: 401 → automatic redirect to login with "Sesi berakhir, silakan login kembali" notice.
   - Cookie cleared: verify refresh_token cookie has Max-Age=-1.

## Notes
- If using idle timeout in addition to absolute timeout (future feature), wait times may differ.
- The 2-minute window includes both frontend and backend overhead.
- Test on both portals (internal and vendor) to verify both auth paths.
