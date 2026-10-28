package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html"
	"log"
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
)

// GetLogin serves the login HTML page.
func GetLogin(c *fiber.Ctx) error {
	hasError := c.Query("error") == "1"
	redirect := c.Query("redirect", "/")

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
	return c.SendString(htmlPage)
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
		return c.Redirect("/login.html?error=1")
	}

	// If auth is disabled, allow entry
	if cfg.Auth.Mode == "none" {
		return c.Redirect(redirect)
	}

	// Validate credentials in constant time
	userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.Auth.Username)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.Auth.Password)) == 1

	if !userMatch || !passMatch {
		return c.Redirect(fmt.Sprintf("/login.html?error=1&redirect=%s", redirect))
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

	return c.Redirect(redirect)
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

// GetWebview retrieves the integrated webview status or forwards to the sidecar.
func GetWebview(c *fiber.Ctx) error {
	cfg, ok := c.Locals("config").(*config.Config)
	if ok && cfg != nil && cfg.Sidecar.Enabled && cfg.Sidecar.BaseURL != "" {
		return extensions.ForwardToSidecar(c, cfg.Sidecar.BaseURL)
	}
	return c.JSON(fiber.Map{
		"status": "not_supported",
	})
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
