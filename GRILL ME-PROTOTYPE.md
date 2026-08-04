

**1. Which frontend?** You have two SPAs defined — CodexCash (internal) and VendorPortal. Are we prototyping one or both? Or is this a unified demo that shows both flows in one app for now?
Answer:this a unified demo that shows both flows in one app

**2. What are you showing?** A prototype to stakeholders could mean:
- Static screens / clickable mockups (no real data)
- Hardcoded JSON / mock data powering real React components
- A subset of actual pages with TanStack Router, just no backend

This matters because "prototype" can range from a Figma-in-React to a near-production frontend shell. What fidelity are you targeting?

Answer:Hardcoded JSON / mock data powering real React components
near-production frontend shell make this prototype in VITE

**3. Which modules/screens first?** Your system has 10+ modules. Stakeholders won't want to see an auth login page. What's the money shot — the thing that makes them say "yes, build this"? I'd guess:
- DSR dashboard (daily cash positions)
- Forecast/replenishment schedule view
- CIT order tracking
- Invoice validation flow

Which 2-3 screens sell the project?

Answer: This will Do
- DSR dashboard (daily cash positions)
- Forecast/replenishment schedule view
- CIT order tracking
- Invoice validation flow

**4. Mock data or live-ish?** Will you seed static JSON files that mimic real DSR/ATM data? Or literally placeholder "Lorem ipsum" data? For a banking stakeholder demo, realistic numbers (even fake) matter.

Answer:Seed static JSON files that mimic real DSR/ATM data

**5. Docker strategy for prototype phase.** You said Nginx. Simple enough. But:
- Single `docker-compose.yml` with just the frontend container? Or do you want the full compose file (redis, caddy, etc.) already stubbed even if unused?
- Will this prototype container eventually become the production frontend container, or is this throwaway?

Answer:Single `docker-compose.yml` with just the frontend container, we will deal with prod conatiner later.

**6. Design system — ready or build as we go?** Your `ui_design.md` steering is thorough (OKLCH tokens, component rules, typography). Do you want the prototype to fully implement the design system from day one, or is this a "get the layout and flows right, polish later" situation?

Answer:get the layout and flows right, polish later

**7. Routing strategy.** TanStack Router with file-based routes from the start? Or plain React Router for speed, then migrate later? Starting with TanStack Router is more work upfront but avoids a rewrite.

Answer: plain React Router for speed, then migrate later