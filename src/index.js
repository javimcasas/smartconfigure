// ─── SmartConfigure Worker ────────────────────────────────────────
// Access gated by SmartMatrix SSO; requires "smartconfigure" in the user's
// apps. The site itself is static (download page + browser-side Log
// Viewer, see public/), so this Worker only handles the session handoff,
// the session cookie and a tiny /api/me. The desktop tool never talks to
// this Worker: binaries are downloaded straight from GitHub Releases.
//
// Session pattern (same as HyperParse / oceanstor-app / ScriptForge):
//   1. ?sso=<handoff token>  → verify with SSO_SHARED_SECRET, re-sign our own
//      24h session token, set it as the HttpOnly cookie, redirect without ?sso
//   2. every other request     → verify the cookie; /api/* gets 401 JSON,
//      everything else 302s to the hub when the session is missing/expired

// SmartMatrix hub — where to send anyone whose session is missing or expired.
const HUB_URL = "https://hubsmartmatrix.com/";
const APP_ID = "smartconfigure";
const COOKIE = "sc_session";
const SESSION_TTL_MS = 86400 * 1000;

function b64urlDecode(str) {
  str = str.replace(/-/g, "+").replace(/_/g, "/");
  while (str.length % 4) str += "=";
  return atob(str);
}

async function verifySso(token, secret) {
  const [payloadB64, sigB64] = token.split(".");
  if (!payloadB64 || !sigB64) return null;
  const key = await crypto.subtle.importKey(
    "raw", new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" }, false, ["verify"]
  );
  const sig = Uint8Array.from(b64urlDecode(sigB64), c => c.charCodeAt(0));
  const valid = await crypto.subtle.verify("HMAC", key, sig, new TextEncoder().encode(payloadB64));
  if (!valid) return null;
  const payload = JSON.parse(b64urlDecode(payloadB64));
  if (payload.exp && Date.now() > payload.exp) return null;
  return payload;
}

// Re-sign the verified `?sso=` payload into this app's own session token.
// The hub's handoff token is short-lived (minutes) and must never become the
// cookie itself; the session below carries the same fields with its own exp.
async function signSso(payload, secret) {
  const enc = new TextEncoder();
  const toB64url = bytes => btoa(String.fromCharCode(...new Uint8Array(bytes)))
    .replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  const payloadB64 = toB64url(enc.encode(JSON.stringify(payload)));
  const key = await crypto.subtle.importKey("raw", enc.encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const sig = await crypto.subtle.sign("HMAC", key, enc.encode(payloadB64));
  return `${payloadB64}.${toB64url(sig)}`;
}

function parseCookies(req) {
  const header = req.headers.get("Cookie") || "";
  return Object.fromEntries(
    header.split(";").map(c => c.trim().split("=")).filter(p => p[0])
  );
}

async function getSessionUser(request, secret) {
  const raw = parseCookies(request)[COOKIE];
  if (!raw) return null;
  try {
    return await verifySso(decodeURIComponent(raw), secret);
  } catch { return null; }
}

function hasAccess(payload) {
  return !!payload && Array.isArray(payload.apps) && payload.apps.includes(APP_ID);
}

function json(data, status = 200) {
  return new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
}

async function handleApi(request, url, user) {
  if (url.pathname === "/api/me" && request.method === "GET") {
    return json({ userId: user.userId, email: user.email, exp: user.exp });
  }
  return json({ error: "Route not found" }, 404);
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    // Either secret name is accepted (ScriptForge uses SSO_SECRET, OceanStor SSO_SHARED_SECRET).
    const secret = env.SSO_SHARED_SECRET || env.SSO_SECRET;
    if (!secret) {
      return new Response("SmartConfigure is not configured: SSO_SHARED_SECRET is missing.", { status: 503 });
    }

    // 1. Session handoff from SmartMatrix (?sso=...)
    const ssoParam = url.searchParams.get("sso");
    if (ssoParam) {
      const payload = await verifySso(ssoParam, secret);
      if (!payload) return Response.redirect(HUB_URL, 302);
      if (!hasAccess(payload)) {
        return new Response("Not authorized for SmartConfigure", { status: 403 });
      }
      url.searchParams.delete("sso");
      const sessionToken = await signSso(
        { userId: payload.userId, email: payload.email, apps: payload.apps, exp: Date.now() + SESSION_TTL_MS },
        secret
      );
      const headers = new Headers();
      headers.append("Set-Cookie",
        `${COOKIE}=${encodeURIComponent(sessionToken)}; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=86400`);
      headers.append("Location", url.pathname + (url.search || ""));
      return new Response(null, { status: 302, headers });
    }

    // 2. API: JSON 401 when the session is dead
    if (url.pathname.startsWith("/api/")) {
      const user = await getSessionUser(request, secret);
      if (!user || !hasAccess(user)) return json({ error: "Not authorized" }, 401);
      try {
        return await handleApi(request, url, user);
      } catch (e) {
        return json({ error: e.message || "Internal error" }, 500);
      }
    }

    // 3. Static assets, only behind a valid session
    const user = await getSessionUser(request, secret);
    if (!user) return Response.redirect(HUB_URL, 302);
    if (!hasAccess(user)) {
      return new Response("Not authorized. Enter through SmartMatrix.", { status: 403 });
    }
    return env.ASSETS.fetch(request);
  }
};
