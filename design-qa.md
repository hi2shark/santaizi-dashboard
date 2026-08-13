# Nazhua visual QA

Source of truth: live Nazhua at `https://nazhua.nezha.in/#/`, verified on 2026-08-12 to be built from upstream commit `d08c973bb4446a24356f49b81d75d6773286596e`.

## Evidence

Live captures are retained under `design-references/nazhua/` at 1920×947, 1440×900 and 390×844, including top, full-page, scrolled, search, sort, map-hover and mobile-detail states. Stable implementation baselines are under `web/e2e/status.spec.ts-snapshots/`.

The final same-canvas comparisons are:

- `design-references/nazhua/qa-compare-desktop-1920x947.png`
- `design-references/nazhua/qa-compare-desktop-1440x900.png`
- `design-references/nazhua/qa-compare-mobile-390x844.png`
- `design-references/nazhua/qa-compare-detail-mobile-390x844.png`

The comparisons use the same viewport and first-screen state. Source production data and Santaizi mock data intentionally differ; geometry, hierarchy, assets and component treatment are the comparison targets.

## Accepted geometry and hierarchy

- Header: one 60px desktop / 70px mobile Nazhua header with the upstream 3px dot field; no ServerStatus header is mounted concurrently.
- Desktop 1440: map `1080×524` at `(180, 80)`, filter at `y=614`, cards at `y=660`, first card width about `353px`.
- Desktop 1920: 1300px list track, 1260px map, cards begin at `y=748` without an empty hero region.
- Mobile 390: map `350×170` at `(20, 90)`, single-column cards, fixed search action and no horizontal overflow.
- Detail mobile: map first, globe/name panel, three metric rings, live counters, cycle traffic, host information and network charts in the upstream order.
- No usable location: the map node and its height are absent; the list follows the header/filter directly.

## Intentional differences

- Product copy and data use 三太子 / Santaizi and the Santaizi V2 contract.
- The function menu remains available for theme, locale, color mode, service, network and Admin navigation.
- Source mobile filter controls are about 30px high. This port uses the repository-required 44px targets, moving the first card down by 28px while preserving the source order and density.
- Light mode uses independent accessible tokens. The world map uses the same cool-grey family as the page (`#e4e9f0` panel, card-like 3px dots) with a CSS-filtered `#aaa` SVG. Map points are gold when online, red when offline, and mixed when a cluster contains both; light-mode points use a white halo so they stay readable on the grey panel. Detail panels use the same `#e4e9f0` cool-grey surface instead of near-white cards.
- System Chinese fonts replace the upstream bundled Sarasa font.
- The detail globe uses the same geographic data and visual orientation through a Canvas orthographic renderer, avoiding a separate WebGL dependency. Light mode uses a cool-grey ocean and darker land so the globe stays visible on white cards.

## Automated coverage

Unit tests cover the typed view adapter, public notes, location parsing, online/formatting/cycle mapping, filters, sorting and theme resolution. Playwright covers the two shells, map/points, cycle-transfer batching, card/row/ServerStatus modes, search dialog, detail, service, network, password, no-location, retry, theme switch and mobile fallback. It also asserts the 1440px source geometry, the 12px minimum font size and 44px mobile targets.

Baselines cover dark/light 1920×947, dark/light 1440×900, dark 1399×945, dark/light 390×844 and dark detail 390×844.

Final result: passed after live-source comparison, independent light-panel tuning, and isolated visual-test color initialization that prevents dark/light baseline bleed.
