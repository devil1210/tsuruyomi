package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	fasthttpWs "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"tsuruyomi/internal/auth"
	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/extensions"
	"tsuruyomi/internal/util"
)

// GetLogin serves the login HTML page.
func GetLogin(c *fiber.Ctx) error {
	hasError := c.Query("error") == "1"
	redirect := c.Query("redirect", "/")
	return renderLoginForm(c, redirect, hasError)
}

func renderLoginForm(c *fiber.Ctx, redirect string, hasError bool) error {
	errorBanner := ""
	if hasError {
		errorBanner = `<div style="background: rgba(239, 68, 68, 0.2); border: 1px solid #ef4444; color: #fca5a5; padding: 12px; border-radius: 8px; margin-bottom: 20px; font-size: 14px; text-align: center;">Usuario o contraseña incorrectos.</div>`
	}

	htmlPage := fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Tsuruyomi - Iniciar Sesión</title>
	<link rel="icon" type="image/svg+xml" href="/favicon.svg">
	<style>
		* { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
		body { background: #0f111a; color: #e2e8f0; display: flex; align-items: center; justify-content: center; min-height: 100vh; padding: 20px; }
		.card { background: #1a1d2e; border: 1px solid #2d334d; border-radius: 16px; padding: 36px 32px; width: 100%%; max-width: 400px; box-shadow: 0 10px 25px rgba(0,0,0,0.5); }
		.logo { text-align: center; margin-bottom: 28px; }
		.logo img { width: 72px; height: 72px; border-radius: 16px; margin-bottom: 12px; }
		.logo h1 { font-size: 24px; font-weight: 700; color: #ffffff; letter-spacing: -0.5px; }
		.logo p { font-size: 13px; color: #94a3b8; margin-top: 4px; }
		.form-group { margin-bottom: 20px; }
		label { display: block; font-size: 13px; font-weight: 500; color: #cbd5e1; margin-bottom: 8px; }
		input[type="text"], input[type="password"] {
			width: 100%%; padding: 12px 14px; background: #0b0c13; border: 1px solid #334155; border-radius: 8px; color: #f8fafc; font-size: 15px; outline: none; transition: border-color 0.2s;
		}
		input[type="text"]:focus, input[type="password"]:focus { border-color: #6366f1; }
		button {
			width: 100%%; padding: 13px; background: #6366f1; color: white; border: none; border-radius: 8px; font-size: 15px; font-weight: 600; cursor: pointer; transition: background 0.2s;
		}
		button:hover { background: #4f46e5; }
	</style>
</head>
<body>
	<div class="card">
		<div class="logo">
			<img src="/favicon.svg" alt="Tsuruyomi" onerror="this.src='/favicon.ico'">
			<h1>Tsuruyomi</h1>
			<p>Servidor Privado de Lectura</p>
		</div>
		%s
		<form method="POST" action="/login.html?redirect=%s">
			<div class="form-group">
				<label for="user">Usuario</label>
				<input type="text" id="user" name="user" required autofocus autocomplete="username">
			</div>
			<div class="form-group">
				<label for="pass">Contraseña</label>
				<input type="password" id="pass" name="pass" required autocomplete="current-password">
			</div>
			<button type="submit">Iniciar Sesión</button>
		</form>
	</div>
</body>
</html>`, errorBanner, html.EscapeString(redirect))

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Status(fiber.StatusOK).SendString(htmlPage)
}

// PostLogin handles submission of the login form.
func PostLogin(c *fiber.Ctx) error {
	user := c.FormValue("user")
	pass := c.FormValue("pass")
	redirect := c.Query("redirect", "/")
	if redirect == "" || !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
		redirect = "/"
	}

	cfg, ok := c.Locals("config").(*config.Config)
	if !ok || cfg == nil {
		return renderLoginForm(c, redirect, true)
	}

	// If auth is disabled, allow entry
	if cfg.Auth.Mode == "none" || cfg.Auth.Mode == "" {
		return c.Redirect(redirect, fiber.StatusSeeOther)
	}

	// Validate credentials in constant time
	userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.Auth.Username)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.Auth.Password)) == 1

	if !userMatch || !passMatch {
		// Return 200 with the login form containing name="user" and name="pass"
		// Suwayomi/Tsumiru protocol signals invalid credentials with HTTP 200 + HTML form
		return renderLoginForm(c, redirect, true)
	}

	// Issue signed session token
	token := auth.CreateSessionToken(user, cfg.Auth.Password)
	c.Cookie(&fiber.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionDuration.Seconds()),
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   c.Protocol() == "https" || c.Get("X-Forwarded-Proto") == "https",
	})

	// HTTP 303 See Other is strictly expected by Tsumiru and modern browsers following POST-redirect-GET
	return c.Redirect(redirect, fiber.StatusSeeOther)
}

// GetLogout logs out the user by clearing the session cookie.
func GetLogout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HTTPOnly: true,
	})
	return c.Redirect("/login.html")
}

// GetGlobalMeta retrieves the global metadata.
func GetGlobalMeta(c *fiber.Ctx) error {
	meta := make(map[string]string)
	if database.DB != nil {
		rows, err := database.DB.Query(c.Context(), "SELECT meta_key, value FROM suwayomi.globalmeta WHERE user_id = 1")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var k, v string
				if err := rows.Scan(&k, &v); err == nil {
					meta[k] = v
				}
			}
		}
	}
	return c.JSON(meta)
}

// ModifyGlobalMeta updates global metadata.
func ModifyGlobalMeta(c *fiber.Ctx) error {
	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if database.DB != nil {
		for k, v := range body {
			var valStr string
			if s, ok := v.(string); ok {
				valStr = s
			} else {
				vBytes, _ := json.Marshal(v)
				valStr = string(vBytes)
			}
			_, _ = database.DB.Exec(c.Context(), `
				INSERT INTO suwayomi.globalmeta (user_id, meta_key, value)
				VALUES (1, $1, $2)
				ON CONFLICT (user_id, meta_key) DO UPDATE SET value = EXCLUDED.value
			`, k, valStr)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

// GetAbout returns static server about info.
func GetAbout(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version": "1.0.0-go-rewrite",
		"name":    "Tsuruyomi",
		"build":   "native",
	})
}

// CheckUpdate checks for server updates.
func CheckUpdate(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"hasUpdate": false,
	})
}

const webviewHTML = `<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tsuruyomi - WebView</title>
    <link rel="icon" href="/favicon.svg" type="image/svg+xml">
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            background: #0f111a;
            color: #e2e8f0;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            height: 100vh;
            overflow: hidden;
            display: flex;
            flex-direction: column;
        }
        /* Top Navigation Header */
        .webview-header {
            height: 50px;
            background: #141726;
            border-bottom: 1px solid #232738;
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 16px;
            gap: 14px;
            flex-shrink: 0;
            box-shadow: 0 2px 10px rgba(0, 0, 0, 0.4);
            z-index: 100;
        }
        .brand-section {
            display: flex;
            align-items: center;
            gap: 12px;
            flex-shrink: 0;
        }
        .brand-logo {
            width: 26px;
            height: 26px;
        }
        .brand-title {
            font-weight: 700;
            font-size: 15px;
            letter-spacing: -0.3px;
            color: #ffffff;
            display: flex;
            align-items: center;
            gap: 6px;
        }
        .brand-title span.tag {
            font-size: 10px;
            background: #e53935;
            color: #ffffff;
            padding: 2px 6px;
            border-radius: 4px;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .badge-status {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            padding: 4px 10px;
            border-radius: 20px;
            font-size: 12px;
            font-weight: 500;
            transition: all 0.25s ease;
            white-space: nowrap;
        }
        .badge-connecting {
            background: rgba(234, 179, 8, 0.15);
            border: 1px solid #eab308;
            color: #fef08a;
        }
        .badge-online {
            background: rgba(34, 197, 94, 0.15);
            border: 1px solid #22c55e;
            color: #86efac;
        }
        .badge-error {
            background: rgba(239, 68, 68, 0.15);
            border: 1px solid #ef4444;
            color: #fca5a5;
        }
        .badge-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            display: inline-block;
        }
        .badge-connecting .badge-dot { background: #eab308; animation: pulse 1.2s infinite ease-in-out; }
        .badge-online .badge-dot { background: #22c55e; }
        .badge-error .badge-dot { background: #ef4444; }

        @keyframes pulse {
            0%, 100% { opacity: 1; transform: scale(1); }
            50% { opacity: 0.4; transform: scale(0.85); }
        }

        .url-section {
            display: flex;
            align-items: center;
            gap: 6px;
            flex-grow: 1;
            max-width: 680px;
        }
        .url-input-wrapper {
            position: relative;
            width: 100%;
            display: flex;
            align-items: center;
        }
        .url-input {
            width: 100%;
            padding: 7px 12px;
            background: #090a10;
            border: 1px solid #2d334d;
            border-radius: 6px;
            color: #f1f5f9;
            font-size: 13px;
            font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
            outline: none;
            transition: border-color 0.2s;
        }
        .url-input:focus { border-color: #6366f1; }
        .btn-bar {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 6px;
            padding: 6px 12px;
            background: #1e2235;
            border: 1px solid #2d334d;
            border-radius: 6px;
            color: #cbd5e1;
            font-size: 13px;
            font-weight: 500;
            cursor: pointer;
            text-decoration: none;
            transition: all 0.2s;
            white-space: nowrap;
        }
        .btn-bar:hover {
            background: #282e46;
            color: #ffffff;
            border-color: #3e4668;
        }
        .btn-bar-primary {
            background: #e53935;
            color: #ffffff;
            border-color: #e53935;
        }
        .btn-bar-primary:hover {
            background: #d32f2f;
            border-color: #d32f2f;
        }
        .actions-section {
            display: flex;
            align-items: center;
            gap: 8px;
            flex-shrink: 0;
        }
        /* Viewport Container */
        .viewport-container {
            flex-grow: 1;
            width: 100%;
            position: relative;
            background: #090a10;
        }
        #webview-iframe {
            width: 100%;
            height: 100%;
            border: none;
            display: block;
        }
    </style>
</head>
<body>
    <header class="webview-header">
        <div class="brand-section">
            <svg class="brand-logo" viewBox="0 0 512 512" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M256 32L64 224L256 480L448 224L256 32Z" stroke="#e53935" stroke-width="28" stroke-linejoin="round"/>
                <path d="M256 32V480M64 224H448" stroke="#e53935" stroke-width="20"/>
            </svg>
            <div class="brand-title">Tsuruyomi <span class="tag">WebView</span></div>
            <div id="status-badge" class="badge-status badge-connecting">
                <span class="badge-dot"></span>
                <span id="badge-text">Conectando Servidor (CT 108)...</span>
            </div>
        </div>

        <div class="url-section">
            <div class="url-input-wrapper">
                <input type="text" id="webview-url" class="url-input" placeholder="https://..." spellcheck="false" autocomplete="off">
            </div>
            <button id="btn-go" class="btn-bar btn-bar-primary" title="Navegar a la URL">Ir</button>
            <button id="btn-reload" class="btn-bar" title="Recargar página">↻</button>
        </div>

        <div class="actions-section">
            <a id="btn-open-external" href="#" target="_blank" rel="noreferrer" class="btn-bar" title="Abrir en pestaña directa local">🌐 Abrir local</a>
            <button id="btn-close" class="btn-bar" title="Cerrar WebView y volver">✕ Salir</button>
        </div>
    </header>

    <main class="viewport-container">
        <iframe id="webview-iframe" src="about:blank" sandbox="allow-same-origin allow-scripts allow-forms allow-popups allow-modals"></iframe>
    </main>

    <script>
        (function() {
            const urlInput = document.getElementById('webview-url');
            const badge = document.getElementById('status-badge');
            const badgeText = document.getElementById('badge-text');
            const iframe = document.getElementById('webview-iframe');
            const btnOpenExternal = document.getElementById('btn-open-external');
            const btnGo = document.getElementById('btn-go');
            const btnReload = document.getElementById('btn-reload');
            const btnClose = document.getElementById('btn-close');

            let currentTargetUrl = '';
            let loadStartTime = 0;
            let latencyReported = false;

            function setBadge(type, text) {
                badge.className = 'badge-status badge-' + type;
                badgeText.textContent = text;
            }

            function navigateTo(url) {
                if (!url || (!url.startsWith('http://') && !url.startsWith('https://'))) {
                    setBadge('error', 'URL no válida');
                    return;
                }
                currentTargetUrl = url;
                urlInput.value = url;
                btnOpenExternal.href = url;
                latencyReported = false;
                setBadge('connecting', 'Conectando vía Servidor (CT 108)...');
                loadStartTime = performance.now();

                if (window.location.hash.substring(1) !== url) {
                    history.replaceState(null, '', '#' + url);
                }

                const proxyUrl = '/api/v1/webview/proxy?url=' + encodeURIComponent(url);
                iframe.src = proxyUrl;
            }

            function initFromHash() {
                let hash = window.location.hash;
                if (hash && hash.length > 1) {
                    let target = '';
                    try { target = decodeURIComponent(hash.substring(1)); }
                    catch(e) { target = hash.substring(1); }
                    if (target) {
                        navigateTo(target);
                        return;
                    }
                }
                setBadge('connecting', 'Esperando URL (#http...)');
            }

            window.addEventListener('message', function(ev) {
                if (ev.data && ev.data.type === 'tsuruyomi_webview_load') {
                    latencyReported = true;
                    if (ev.data.error) {
                        setBadge('error', 'Servidor CT 108: Error (' + (ev.data.status || '502') + ')');
                    } else {
                        setBadge('online', '🟢 Servidor CT 108: HTTP ' + ev.data.status + ' (' + ev.data.latency + 'ms)');
                    }
                    if (ev.data.url && ev.data.url !== currentTargetUrl) {
                        currentTargetUrl = ev.data.url;
                        urlInput.value = ev.data.url;
                        btnOpenExternal.href = ev.data.url;
                        history.replaceState(null, '', '#' + ev.data.url);
                    }
                }
            });

            iframe.addEventListener('load', function() {
                if (!latencyReported && currentTargetUrl) {
                    const elapsed = Math.round(performance.now() - loadStartTime);
                    setBadge('online', '🟢 Servidor CT 108: Conectado (~' + elapsed + 'ms)');
                }
            });

            btnGo.addEventListener('click', function() {
                const val = urlInput.value.trim();
                if (val) navigateTo(val);
            });

            urlInput.addEventListener('keydown', function(e) {
                if (e.key === 'Enter') {
                    const val = urlInput.value.trim();
                    if (val) navigateTo(val);
                }
            });

            btnReload.addEventListener('click', function() {
                if (currentTargetUrl) {
                    latencyReported = false;
                    setBadge('connecting', 'Recargando vía Servidor (CT 108)...');
                    loadStartTime = performance.now();
                    try {
                        iframe.contentWindow.location.reload();
                    } catch(e) {
                        iframe.src = '/api/v1/webview/proxy?url=' + encodeURIComponent(currentTargetUrl);
                    }
                }
            });

            btnClose.addEventListener('click', function() {
                if (window.opener) {
                    window.close();
                } else if (window.history.length > 1) {
                    window.history.back();
                } else {
                    window.location.href = '/library';
                }
            });

            window.addEventListener('hashchange', initFromHash);
            initFromHash();
        })();
    </script>
</body>
</html>`

// GetWebview retrieves the integrated webview status, forwards to sidecar, or renders the webview frame portal.
func GetWebview(c *fiber.Ctx) error {
	cfg, ok := c.Locals("config").(*config.Config)
	if ok && cfg != nil && cfg.Sidecar.Enabled && cfg.Sidecar.BaseURL != "" {
		return extensions.ForwardToSidecar(c, cfg.Sidecar.BaseURL)
	}

	accept := c.Get("Accept")
	if strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html") {
		return c.JSON(fiber.Map{
			"status": "not_supported",
		})
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.SendString(webviewHTML)
}

// GetWebviewProxy acts as a server-side reverse proxy to display external sources
// within the Tsuruyomi WebView frame, verifying connectivity directly from the server.
func GetWebviewProxy(c *fiber.Ctx) error {
	rawURL := c.Query("url")
	if rawURL == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Error: No URL provided in query parameter 'url'")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return c.Status(fiber.StatusBadRequest).SendString("Error: Invalid or unsupported URL scheme")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Error creating request: %v", err))
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9,en;q=0.8")

	client := &http.Client{
		Timeout: 25 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return renderProxyErrorPage(c, rawURL, err.Error(), 0, latency)
	}
	defer resp.Body.Close()

	// If Cloudflare challenge (403/503) and FlareSolverr is enabled, attempt solve
	cfg, _ := c.Locals("config").(*config.Config)
	if (resp.StatusCode == 403 || resp.StatusCode == 503) && cfg != nil && cfg.FlareSolverr.Enabled {
		fsStart := time.Now()
		solvedHTML, fsErr := util.SolveAndFetchHTML(ctx, cfg, rawURL, "")
		if fsErr == nil && solvedHTML != "" {
			latency = time.Since(fsStart).Milliseconds()
			return deliverProxiedHTML(c, rawURL, solvedHTML, fiber.StatusOK, latency)
		}
	}

	contentType := resp.Header.Get("Content-Type")
	isHTML := strings.Contains(strings.ToLower(contentType), "text/html") || contentType == ""

	if isHTML {
		bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 15*1024*1024))
		if readErr != nil {
			return renderProxyErrorPage(c, rawURL, fmt.Sprintf("Error leyendo respuesta: %v", readErr), resp.StatusCode, latency)
		}
		return deliverProxiedHTML(c, rawURL, string(bodyBytes), resp.StatusCode, latency)
	}

	// For non-HTML assets (images, stylesheets, scripts)
	for k, vv := range resp.Header {
		lower := strings.ToLower(k)
		if lower == "x-frame-options" || lower == "content-security-policy" || lower == "x-content-security-policy" || lower == "strict-transport-security" {
			continue
		}
		for _, v := range vv {
			c.Response().Header.Add(k, v)
		}
	}
	c.Response().Header.Del("X-Frame-Options")
	c.Response().Header.Del("Content-Security-Policy")
	c.Response().Header.Del("X-Content-Security-Policy")
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Response().BodyWriter(), resp.Body)
	return nil
}

func deliverProxiedHTML(c *fiber.Ctx, targetURL string, rawHTML string, statusCode int, latencyMs int64) error {
	finalBase := targetURL
	if u, err := url.Parse(targetURL); err == nil {
		finalBase = u.String()
	}

	injectedScript := fmt.Sprintf(`
<base href="%s">
<script>
(function() {
    try {
        if (window.parent && window.parent !== window) {
            window.parent.postMessage({
                type: 'tsuruyomi_webview_load',
                status: %d,
                latency: %d,
                url: %q
            }, '*');
        }
    } catch(e) {}

    document.addEventListener('click', function(e) {
        var a = e.target.closest('a');
        if (a && a.href && !a.href.startsWith('javascript:') && !a.href.startsWith('#')) {
            if (a.target !== '_blank') {
                e.preventDefault();
                window.location.href = '/api/v1/webview/proxy?url=' + encodeURIComponent(a.href);
            }
        }
    }, true);
})();
</script>`, html.EscapeString(finalBase), statusCode, latencyMs, targetURL)

	modifiedHTML := rawHTML
	headIdx := strings.Index(strings.ToLower(rawHTML), "<head>")
	if headIdx != -1 {
		insertPos := headIdx + 6
		modifiedHTML = rawHTML[:insertPos] + injectedScript + rawHTML[insertPos:]
	} else {
		modifiedHTML = injectedScript + rawHTML
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	c.Response().Header.Del("X-Frame-Options")
	c.Response().Header.Del("Content-Security-Policy")
	c.Response().Header.Del("X-Content-Security-Policy")
	c.Response().Header.Set("X-Tsuruyomi-Proxy-Status", fmt.Sprintf("%d", statusCode))
	c.Response().Header.Set("X-Tsuruyomi-Proxy-Latency", fmt.Sprintf("%dms", latencyMs))
	return c.Status(statusCode).SendString(modifiedHTML)
}

func renderProxyErrorPage(c *fiber.Ctx, targetURL string, errMsg string, statusCode int, latencyMs int64) error {
	errorHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Error de Conexión en WebView - Tsuruyomi</title>
    <style>
        body { background: #0f111a; color: #f8fafc; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; padding: 20px; box-sizing: border-box; }
        .card { background: #1a1d2e; border: 1px solid #ef4444; border-radius: 12px; padding: 32px; max-width: 540px; width: 100%%; box-shadow: 0 10px 25px rgba(0,0,0,0.5); text-align: center; }
        h2 { color: #f87171; margin-top: 0; font-size: 1.3rem; }
        p { color: #94a3b8; font-size: 0.95rem; line-height: 1.5; }
        .details { background: #0b0c13; border: 1px solid #334155; border-radius: 8px; padding: 12px; font-family: monospace; font-size: 0.85rem; color: #fca5a5; text-align: left; word-break: break-all; margin: 16px 0; }
        .btn-group { display: flex; gap: 12px; justify-content: center; margin-top: 24px; }
        .btn { padding: 10px 18px; border-radius: 8px; font-size: 0.9rem; font-weight: 600; text-decoration: none; cursor: pointer; border: none; }
        .btn-retry { background: #6366f1; color: #fff; }
        .btn-direct { background: #334155; color: #cbd5e1; }
    </style>
    <script>
        try {
            if (window.parent && window.parent !== window) {
                window.parent.postMessage({
                    type: 'tsuruyomi_webview_load',
                    status: %d,
                    latency: %d,
                    error: true
                }, '*');
            }
        } catch(e) {}
    </script>
</head>
<body>
    <div class="card">
        <h2>⚠️ Error al conectar con el servidor origen</h2>
        <p>El servidor Tsuruyomi (CT 108) no pudo establecer comunicación con la fuente remota.</p>
        <div class="details">%s</div>
        <p style="font-size: 0.85rem; color: #64748b;">URL solicitada: <code>%s</code></p>
        <div class="btn-group">
            <button class="btn btn-retry" onclick="location.reload()">Reintentar conexión</button>
            <a class="btn btn-direct" href="%s" target="_blank" rel="noreferrer">Abrir directamente en navegador</a>
        </div>
    </div>
</body>
</html>`, statusCode, latencyMs, html.EscapeString(errMsg), html.EscapeString(targetURL), html.EscapeString(targetURL))

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	c.Response().Header.Del("X-Frame-Options")
	c.Response().Header.Del("Content-Security-Policy")
	return c.Status(fiber.StatusBadGateway).SendString(errorHTML)
}

// WebviewWS handles the WebSocket connection for webview management by proxying to the sidecar.
func WebviewWS(conn *websocket.Conn) {
	cfg, ok := conn.Locals("config").(*config.Config)
	if !ok || cfg == nil || !cfg.Sidecar.Enabled || cfg.Sidecar.BaseURL == "" {
		log.Printf("[WebviewWS] Sidecar is not enabled or configured")
		conn.Close()
		return
	}

	wsURL := strings.Replace(cfg.Sidecar.BaseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	targetURL := wsURL + "/api/v1/webview"

	sidecarConn, resp, err := fasthttpWs.DefaultDialer.Dial(targetURL, nil)
	if err != nil {
		log.Printf("[WebviewWS] Failed to connect to sidecar WS at %s: %v", targetURL, err)
		if resp != nil {
			log.Printf("[WebviewWS] Sidecar WS handshake status: %s", resp.Status)
		}
		conn.Close()
		return
	}
	defer sidecarConn.Close()

	done := make(chan struct{})
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			close(done)
			conn.Close()
			sidecarConn.Close()
		})
	}

	// Client -> Sidecar
	go func() {
		defer closeBoth()
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := sidecarConn.WriteMessage(messageType, p); err != nil {
				return
			}
		}
	}()

	// Sidecar -> Client
	go func() {
		defer closeBoth()
		for {
			messageType, p, err := sidecarConn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(messageType, p); err != nil {
				return
			}
		}
	}()

	<-done
}

// UpdateServer executes the update script on the host and returns the execution log as HTML.
func UpdateServer(c *fiber.Ctx) error {
	cmd := exec.Command("/opt/suwayomi/update.sh")
	output, err := cmd.CombinedOutput()
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	if err != nil {
		return c.SendString(fmt.Sprintf(`
			<!DOCTYPE html>
			<html>
			<head>
				<title>Actualización del Servidor Fallida</title>
				<style>
					body { font-family: sans-serif; background: #2c0000; color: #ff9999; padding: 20px; }
					pre { background: #1a0000; padding: 15px; border-radius: 5px; border: 1px solid #ff3333; overflow-x: auto; }
				</style>
			</head>
			<body>
				<h1>Actualización del Servidor Fallida</h1>
				<p>Error al ejecutar el script de actualización: <strong>%v</strong></p>
				<h3>Salida del log:</h3>
				<pre>%s</pre>
				<p><a href="/" style="color: #ffcccc;">Volver al Inicio</a></p>
			</body>
			</html>
		`, err, string(output)))
	}

	return c.SendString(fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Actualización del Servidor Completada</title>
			<style>
				body { font-family: sans-serif; background: #002c00; color: #99ff99; padding: 20px; }
				pre { background: #001a00; padding: 15px; border-radius: 5px; border: 1px solid #33ff33; overflow-x: auto; }
			</style>
		</head>
		<body>
			<h1>Actualización del Servidor Completada</h1>
			<p>El script de actualización se ejecutó correctamente.</p>
			<h3>Salida del log:</h3>
			<pre>%s</pre>
			<p><a href="/" style="color: #ccffcc;">Volver al Inicio</a></p>
		</body>
		</html>
	`, string(output)))
}

// PostUpdateServer handles JSON POST requests to update the server.
func PostUpdateServer(c *fiber.Ctx) error {
	cmd := exec.Command("/opt/suwayomi/update.sh")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
			"log":     string(output),
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"log":     string(output),
	})
}
