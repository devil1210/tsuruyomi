package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	fasthttpWs "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"tsuruyomi/internal/config"
	"tsuruyomi/internal/database"
	"tsuruyomi/internal/extensions"
)

// GetLogin serves the login HTML page.
func GetLogin(c *fiber.Ctx) error {
	// In the real system, this renders Login.jte (a JTE template).
	// For now, we return a simple login HTML page stub.
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.SendString(`
		<!DOCTYPE html>
		<html>
		<head><title>Suwayomi Login</title></head>
		<body>
			<form method="POST">
				<label>User: <input type="text" name="user"></label><br/>
				<label>Pass: <input type="password" name="pass"></label><br/>
				<button type="submit">Login</button>
			</form>
		</body>
		</html>
	`)
}

// PostLogin handles submission of the login form.
func PostLogin(c *fiber.Ctx) error {
	user := c.FormValue("user")
	pass := c.FormValue("pass")

	// Placeholder verification matching config
	// Set the session cookie and redirect
	c.Cookie(&fiber.Cookie{
		Name:  "logged-in",
		Value: user,
		Path:  "/",
	})

	_ = pass // suppress unused warning
	redirect := c.Query("redirect", "/")
	return c.Redirect(redirect)
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
