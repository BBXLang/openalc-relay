# Synthetic Web verification page

Vue 3 + TypeScript, built with Vite. This is a minimal, read-only local PoC, not
the formal Console. Native HarmonyOS business integration remains deferred under
[ADR-0003](../../docs/decisions/0003-web-first.md). The owner's domain, server and
SSL are not used by this experiment.

After the owner's screenshot correction on 2026-11-05, this PoC uses an empty
mobile-first inbox, top segmented navigation, and a floating installation-ID
reader. It continues to show synthetic event metadata rather than SMS conversations. Shared
visual tokens are in [the UI guide](../../docs/design/RELAY-UI.zh-CN.md).

The page reads the separate Go collector's **committed SQLite metadata**, filtered
by the installation UUID shown in Android's debug controls. It displays 25 records
per page, supports manual refresh/next page, and shows loading, empty and error
states. Changing the installation or a failed read clears previous results.
No polling, WebSocket, browser persistence, webpage fixture generation or SMS
sending is included. Installation UUID filtering is not authentication.

## Run locally

Requires Node >=22.12 and Go >=1.26. From the repository root:

```powershell
npm ci --prefix apps/web
npm --prefix apps/web run build
cd server
go run ./cmd/poc-collector -web-dir ../apps/web/dist
```

Open `http://127.0.0.1:8787/`, paste the Android PoC Installation ID, then click
Read / Refresh. Use the literal IP; `localhost` and domain Host values are refused.
Android's existing `adb reverse tcp:8787 tcp:8787` test procedure is documented in
the [Android README](../android/README.md). A phone is not required for host tests.
Without `-web-dir`, the collector exposes only its synthetic API and `/` stays 404.

The collector accepts loopback peers and literal-loopback Host only. Browser reads
must be same-origin. Browser POSTs remain forbidden. The API has no CORS allowance;
assets use a restrictive CSP and responses disable caching. Never proxy this
unauthenticated harness to the Internet, even behind SSL.

## Repeatable verification

From the repository root, with JDK 25 and an isolated Gradle home configured:

```powershell
. ./scripts/use-android-env.ps1 # This workstation's existing local SDK/JDK setup
./scripts/test-web-poc.ps1
```

On CI or a machine with JDK 25 already configured, the environment helper is not
needed. The test runner installs pinned npm dependencies and Playwright Chromium,
runs the real Room/JVM suite, seeds 100 fixtures, verifies offline retention,
recovers through deliberate ACK loss, restarts Go on the same SQLite file, then
tests the built page in desktop and mobile-sized Chromium. Every displayed UUID
is compared against the Kotlin seed manifest; the successful chain has no mocked
API/data. Error/loading tests use explicit browser fault injection.

Raw evidence stays ignored in `bin/poc-integration/<run-id>/` and
`apps/web/build/`. CI defines this check but has not run remotely. See
[Web PoC evidence and limits](../../docs/development/WEB-POC.md).
An additional [Xiaomi-to-browser check](../../docs/development/DEVICE-SYNC-2026-11-05.md)
used two button-created synthetic events on a physical phone, the USB loopback
tunnel, the collector, and mobile/desktop Chromium views.

The owner approved this first UI direction on 2026-11-05. Production authentication,
authorization, encrypted payload storage and deployment are separate work.
